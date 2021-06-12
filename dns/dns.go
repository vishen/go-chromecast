package dns

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/grandcat/zeroconf"
	log "github.com/vishen/go-chromecast/log"
)

// CastDNSEntry is the interface that satisfies a Cast type.
type CastDNSEntry interface {
	GetName() string
	GetUUID() string
	GetAddr() string
	GetPort() int
}

// CastEntry is the concrete cast entry type.
type CastEntry struct {
	AddrV4 net.IP
	AddrV6 net.IP
	Port   int

	Name string
	Host string

	UUID       string
	Device     string
	Status     string
	DeviceName string
	InfoFields map[string]string
}

// GetUUID returns a unqiue id of a cast entry.
func (e CastEntry) GetUUID() string {
	return e.UUID
}

// GetName returns the identified name of a cast entry.
func (e CastEntry) GetName() string {
	return e.DeviceName
}

// GetAddr returns the IPV4 of a cast entry if it is not nil otherwise the IPV6.
func (e CastEntry) GetAddr() string {
	if e.AddrV4 != nil {
		return e.AddrV4.String()
	} else {
		return fmt.Sprintf("[%s]", e.AddrV6.String())
	}
}

// GetPort returns the port of a cast entry.
func (e CastEntry) GetPort() int {
	return e.Port
}

// DiscoverCastDNSEntryByName returns the first cast dns device
// found that matches the name.
func DiscoverCastDNSEntryByName(ctx context.Context, iface *net.Interface, name string) (CastEntry, error) {
	castEntryChan, err := DiscoverCastDNSEntries(ctx, iface)
	if err != nil {
		return CastEntry{}, err
	}

	for d := range castEntryChan {
		if d.DeviceName == name {
			return d, nil
		}
	}
	return CastEntry{}, fmt.Errorf("No cast device found with name %q", name)
}

// DiscoverCastDNSEntries will return a channel with any cast dns entries
// found.
func DiscoverCastDNSEntries(ctx context.Context, iface *net.Interface) (<-chan CastEntry, error) {
	var opts = []zeroconf.ClientOption{}
	if iface != nil {
		opts = append(opts, zeroconf.SelectIfaces([]net.Interface{*iface}))
	}
	resolver, err := zeroconf.NewResolver(opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create new zeroconf resolver: %w", err)
	}

	castDNSEntriesChan := make(chan CastEntry, 5)
	entriesChan := make(chan *zeroconf.ServiceEntry, 5)
	go func() {
		if err := resolver.Browse(ctx, "_googlecast._tcp", "local", entriesChan); err != nil {
			log.Printf("error: unable to browser for mdns entries: %v", err)
			return
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(castDNSEntriesChan)
				return
			case entry := <-entriesChan:
				if entry == nil {
					continue
				}
				castEntry := CastEntry{
					Port: entry.Port,
					Host: entry.HostName,
				}
				if len(entry.AddrIPv4) > 0 {
					castEntry.AddrV4 = entry.AddrIPv4[0]
				}
				if len(entry.AddrIPv6) > 0 {
					castEntry.AddrV6 = entry.AddrIPv6[0]
				}
				infoFields := make(map[string]string, len(entry.Text))
				for _, value := range entry.Text {
					if kv := strings.SplitN(value, "=", 2); len(kv) == 2 {
						key := kv[0]
						val := kv[1]

						infoFields[key] = val

						switch key {
						case "fn":
							castEntry.DeviceName = val
						case "md":
							castEntry.Device = val
						case "id":
							castEntry.UUID = val
						}
					}
				}
				castEntry.InfoFields = infoFields
				castDNSEntriesChan <- castEntry
			}
		}
	}()
	return castDNSEntriesChan, nil
}
