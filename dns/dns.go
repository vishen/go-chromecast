package dns

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

func init() {
	// Disable mdns annoying and verbose logging
	log.SetOutput(ioutil.Discard)
}

type CastDNSEntry interface {
	GetName() string
	GetUUID() string
	GetAddr() string
	GetPort() int
}

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

func (e CastEntry) GetUUID() string {
	return e.UUID
}

func (e CastEntry) GetName() string {
	return e.DeviceName
}

func (e CastEntry) GetAddr() string {
	return fmt.Sprintf("%s", e.AddrV4)
}

func (e CastEntry) GetPort() int {
	return e.Port
}

func FindCastDNSEntries() []CastEntry {
	entriesCh := make(chan *mdns.ServiceEntry, 20)
	go func() {
		// This will find any and all google products, including chromecast, home mini, etc.
		mdns.Query(&mdns.QueryParam{
			Service: "_googlecast._tcp",
			Domain:  "local",
			Timeout: time.Second * 3,
			Entries: entriesCh,
		})
		close(entriesCh)
	}()
	entries := make([]CastEntry, 0, 20)
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
	return entries
}
