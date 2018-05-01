package main

import (
	"log"
	"net"
	"strings"
	"time"

	"github.com/micro/mdns"
)

type CastEntry struct {
	addrV4 net.IP
	addrV6 net.IP
	port   int

	name string
	host string

	uuid       string
	device     string
	status     string
	deviceName string
	infoFields map[string]string
}

func getCastEntry() *CastEntry {
	entriesCh := make(chan *mdns.ServiceEntry, 1)

	mdns.Query(&mdns.QueryParam{
		Service: "_googlecast._tcp",
		Domain:  "local",
		Timeout: time.Second * 3,
		Entries: entriesCh,
	})

	return fillCastEntry(<-entriesCh)
}

func fillCastEntry(entry *mdns.ServiceEntry) *CastEntry {
	infoFields := make(map[string]string, len(entry.InfoFields))
	for _, infoField := range entry.InfoFields {
		splitField := strings.Split(infoField, "=")
		if len(splitField) != 2 {
			log.Printf("[error] Incorrect format for field in entry.InfoFields: %s\n", infoField)
			continue
		}
		infoFields[splitField[0]] = splitField[1]
	}
	return &CastEntry{
		addrV4:     entry.AddrV4,
		addrV6:     entry.AddrV6,
		port:       entry.Port,
		name:       entry.Name,
		host:       entry.Host,
		infoFields: infoFields,
		uuid:       infoFields["id"],
		device:     infoFields["md"],
		deviceName: infoFields["fn"],
		status:     infoFields["rs"],
	}
}
