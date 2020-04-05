package dns

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

func init() {
	// Disable mdns annoying and verbose logging
	log.SetOutput(ioutil.Discard)
}

const (
	maxEntries = 20
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

// GetAddr returns the IPV4 of a cast entry.
func (e CastEntry) GetAddr() string {
	return fmt.Sprintf("%s", e.AddrV4)
}

// GetPort returns the port of a cast entry.
func (e CastEntry) GetPort() int {
	return e.Port
}

// FindCastDNSEntries returns all found cast entries.
func FindCastDNSEntries(iface *net.Interface, dnsTimeoutSeconds int) []CastEntry {
	entriesCh := make(chan *mdns.ServiceEntry, maxEntries)
	go func() {
		// This will find any and all google products, including chromecast, home mini, etc.
		mdns.Query(&mdns.QueryParam{
			Service:   "_googlecast._tcp",
			Domain:    "local",
			Timeout:   time.Second * time.Duration(dnsTimeoutSeconds),
			Entries:   entriesCh,
			Interface: iface,
		})
		close(entriesCh)
	}()
	entries := make([]CastEntry, 0)
	for entry := range entriesCh {
		infoFields := make(map[string]string, len(entry.InfoFields))
		for _, infoField := range entry.InfoFields {
			splitField := strings.Split(infoField, "=")
			if len(splitField) != 2 {
				continue
			}
			infoFields[splitField[0]] = splitField[1]
		}
		entries = append(entries, CastEntry{
			AddrV4:     entry.AddrV4,
			AddrV6:     entry.AddrV6,
			Port:       entry.Port,
			Name:       entry.Name,
			Host:       entry.Host,
			InfoFields: infoFields,
			UUID:       infoFields["id"],
			Device:     infoFields["md"],
			DeviceName: infoFields["fn"],
			Status:     infoFields["rs"],
		})
	}

	// Always return entries in deterministic order.
	sort.Slice(entries, func(i, j int) bool { return entries[i].DeviceName < entries[j].DeviceName })

	return entries
}
