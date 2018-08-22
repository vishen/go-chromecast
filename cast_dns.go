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

			/*
				&mdns.ServiceEntry{Name:"Chromecast-b380c5847b3182e4fb2eb0d0e270bf16._googlecast._tcp.local.", Host:"b380c584-7b31-82e4-fb2e-b0d0e270bf16.local.", AddrV4:net.IP{0xc0, 0xa8, 0x0, 0x73}, AddrV6:net.IP(nil), Port:8009, Info:"id=b380c5847b3182e4fb2eb0d0e270bf16|cd=71671206FF503E2F4857C87BA235CFAC|rm=9DCA7718E292093B|ve=05|md=Chromecast|ic=/setup/icon.png|fn=MarieGotGame?|ca=4101|st=0|bs=FA8FCA9134E7|nf=1|rs=", InfoFields:[]string{"id=b380c5847b3182e4fb2eb0d0e270bf16", "cd=71671206FF503E2F4857C87BA235CFAC", "rm=9DCA7718E292093B", "ve=05", "md=Chromecast", "ic=/setup/icon.png", "fn=MarieGotGame?", "ca=4101", "st=0", "bs=FA8FCA9134E7", "nf=1", "rs="}, TTL:120, Addr:net.IP{0xc0, 0xa8, 0x0, 0x73}, hasTXT:true, sent:true}
				&main.CastEntry{addrV4:net.IP{0xc0, 0xa8, 0x0, 0x73}, addrV6:net.IP(nil), port:8009, name:"Chromecast-b380c5847b3182e4fb2eb0d0e270bf16._googlecast._tcp.local.", host:"b380c584-7b31-82e4-fb2e-b0d0e270bf16.local.", uuid:"b380c5847b3182e4fb2eb0d0e270bf16", device:"Chromecast", status:"", deviceName:"MarieGotGame?", infoFields:map[string]string{"id":"b380c5847b3182e4fb2eb0d0e270bf16", "cd":"71671206FF503E2F4857C87BA235CFAC", "rm":"9DCA7718E292093B", "ve":"05", "md":"Chromecast", "ca":"4101", "ic":"/setup/icon.png", "fn":"MarieGotGame?", "st":"0", "bs":"FA8FCA9134E7", "nf":"1", "rs":""}}
				2018/08/22 19:30:54 no chromecast found
				exit status 1
				jonathanpentecost@Jonathans-MacBook-Pro-2 ~/g/s/g/v/go-chromecast> go run *.go  load ~/Movies/The\ Office\ \[2.01\]\ The\ Dundies.avi
				2018/08/22 19:31:43 Starting new app
				2018/08/22 19:31:43 Initalising
				2018/08/22 19:31:43 Getting dns entry
				&mdns.ServiceEntry{Name:"Google-Home-Mini-b87d86bed423a6feb8b91a7d2778b55c._googlecast._tcp.local.", Host:"b87d86be-d423-a6fe-b8b9-1a7d2778b55c.local.", AddrV4:net.IP{0xc0, 0xa8, 0x0, 0x34}, AddrV6:net.IP(nil), Port:8009, Info:"id=b87d86bed423a6feb8b91a7d2778b55c|cd=7110BCF1C2D6743969B61E5990970AD9|rm=999AE527C5BE349D|ve=05|md=Google Home Mini|ic=/setup/icon.png|fn=Living Room Speaker|ca=2052|st=1|bs=FA8FCA3E22DA|nf=1|rs=Default Media Receiver", InfoFields:[]string{"id=b87d86bed423a6feb8b91a7d2778b55c", "cd=7110BCF1C2D6743969B61E5990970AD9", "rm=999AE527C5BE349D", "ve=05", "md=Google Home Mini", "ic=/setup/icon.png", "fn=Living Room Speaker", "ca=2052", "st=1", "bs=FA8FCA3E22DA", "nf=1", "rs=Default Media Receiver"}, TTL:120, Addr:net.IP{0xc0, 0xa8, 0x0, 0x34}, hasTXT:true, sent:true}
				&main.CastEntry{addrV4:net.IP{0xc0, 0xa8, 0x0, 0x34}, addrV6:net.IP(nil), port:8009, name:"Google-Home-Mini-b87d86bed423a6feb8b91a7d2778b55c._googlecast._tcp.local.", host:"b87d86be-d423-a6fe-b8b9-1a7d2778b55c.local.", uuid:"b87d86bed423a6feb8b91a7d2778b55c", device:"Google Home Mini", status:"Default Media Receiver", deviceName:"Living Room Speaker", infoFields:map[string]string{"cd":"7110BCF1C2D6743969B61E5990970AD9", "ic":"/setup/icon.png", "bs":"FA8FCA3E22DA", "id":"b87d86bed423a6feb8b91a7d2778b55c", "rm":"999AE527C5BE349D", "ve":"05", "md":"Google Home Mini", "fn":"Living Room Speaker", "ca":"2052", "st":"1", "nf":"1", "rs":"Default Media Receiver"}}
				2018/08/22 19:31:46 no chromecast found
				exit status 1
			*/

			if ce.device == "Chromecast" && (uuid == "" || uuid == ce.uuid) {
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
