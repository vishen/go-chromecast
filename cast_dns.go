package main

import (
	"time"

	"github.com/micro/mdns"
)

func getCastEntry() *mdns.ServiceEntry {
	entriesCh := make(chan *mdns.ServiceEntry, 1)

	mdns.Query(&mdns.QueryParam{
		Service: "_googlecast._tcp",
		Domain:  "local",
		Timeout: time.Second * 3,
		Entries: entriesCh,
	})

	return <-entriesCh
}
