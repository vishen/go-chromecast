package main

import (
	"fmt"
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

func getCastEntry(uuid string) *CastEntry {
	castEntriesCh := make(chan *CastEntry, 1)
	entriesCh := make(chan *mdns.ServiceEntry, 20)
	go func() {
		for entry := range entriesCh {
			ce := fillCastEntry(entry)
			if uuid == "" || uuid == ce.uuid {
				castEntriesCh <- ce
				return
			}
		}
		castEntriesCh <- nil
	}()

	mdns.Query(&mdns.QueryParam{
		Service: "_googlecast._tcp",
		Domain:  "local",
		Timeout: time.Second * 3,
		Entries: entriesCh,
	})
	close(entriesCh)

	return <-castEntriesCh
}

func printCastEntries() {
	entriesCh := make(chan *mdns.ServiceEntry, 20)
	go func() {
		for entry := range entriesCh {
			fmt.Println(fillCastEntry(entry).String())
		}
	}()

	err := mdns.Query(&mdns.QueryParam{
		Service: "_googlecast._tcp",
		Domain:  "local",
		Timeout: time.Second * 10,
		Entries: entriesCh,
	})
	if err != nil {
		log.Fatal(err)
	}

	close(entriesCh)
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

func (c *CastEntry) String() string {
	return fmt.Sprintf("[IPv4=%s; IPv6=%s; port=%d; name=%s; host=%s; uuid=%s; device=%s; deviceName=%s; status=%s]",
		c.addrV4.String(), c.addrV6.String(), c.port, c.name, c.host, c.uuid, c.device, c.deviceName, c.status)
}
