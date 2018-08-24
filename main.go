package main

import (
	"fmt"
	"log"

	"github.com/vishen/go-chromecast/application"
	castdns "github.com/vishen/go-chromecast/dns"
)

func main() {
	debug := true
	app := application.NewApplication(debug)
	// What happens if we call Update first (which tries to write to the connection)
	// before we have a connection established?
	// app.Update()

	dnsEntries := castdns.FindCastDNSEntries()
	var entry castdns.CastDNSEntry
	for _, e := range dnsEntries {
		if e.Device == "Chromecast" {
			entry = e
		}
		log.Printf("found dns entry: %#v", e)
	}
	if err := app.Start(entry); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("app=%#v\n", app)
}
