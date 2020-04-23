package main

import (
	"fmt"
	"log"
	"net"

	"github.com/miekg/dns"
)

const (
	mdnsAddr             = "224.0.0.251:5353"
	chromecastLookupName = "_googlecast._tcp.local."
	simulatorType        = "ChromecastSimulator"
	simulatorName        = "chromcast-simulator-test"
	simulatorID          = "abcdef0123456789"
	// Google-Home-Mini-b87d86bed423a6feb8b91a7d2778b55c._googlecast._tcp.local.
	simulatorDNS = simulatorType + "-" + simulatorID + "." + chromecastLookupName
)

func main() {
	fmt.Printf("serving %q...\n", simulatorDNS)
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatal(err)
	}

	// Get Local IP
	var localIP string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback == net.FlagLoopback {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			log.Printf("unable to get addresses for interface %q: %v", iface.Name, err)
			continue
		}
		for _, addr := range addrs {
			if netIP, ok := addr.(*net.IPNet); ok {
				localIP = netIP.IP.String()
				fmt.Printf("using iface=%s\n", iface.Name)
				break
			}
		}
		if localIP != "" {
			break
		}
	}

	if localIP == "" {
		log.Fatal("unable to find a non-local ip address")
	}

	listenerAddr, err := startTCPServer()
	if err != nil {
		log.Fatal(err)
	}
	// Get port from listener
	var listenerAddrPort int
	tcpAddr, ok := listenerAddr.(*net.TCPAddr)
	if !ok {
		log.Printf("unable to get port from tcp address")
		return
	}
	listenerAddrPort = tcpAddr.Port

	if err := startMDNSServer(localIP, listenerAddrPort); err != nil {
		log.Fatal(err)
	}

	done := make(chan struct{})
	done <- struct{}{}
}

func startTCPServer() (addr net.Addr, err error) {
	ln, err := net.Listen("tcp4", "")
	if err != nil {
		return addr, err
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("error: unable to accept connection: %v", err)
			}
			fmt.Printf("listener conn: %#v\n", conn)
		}
	}()
	return ln.Addr(), nil
}

func startMDNSServer(ip string, port int) error {
	// https://en.wikipedia.org/wiki/Multicast_DNS
	// https://github.com/grandcat/zeroconf/blob/master/server.go
	// listen to incoming udp packets
	pc, err := net.ListenPacket("udp", mdnsAddr)
	if err != nil {
		return err
	}

	ipv4IP := net.ParseIP(ip)

	log.Printf("listening for mdns on %s\n", mdnsAddr)
	go func() {
		defer pc.Close()
		for {
			// TODO: is 1024 big enough? What is the size of a udp packet?
			buf := make([]byte, 1024)
			_, addr, err := pc.ReadFrom(buf)
			if err != nil {
				log.Printf("error: unable to read packet: %v", err)
				continue
			}
			var msg dns.Msg
			if err := msg.Unpack(buf); err != nil {
				log.Printf("error: unable to unpack packet: %v", err)
				continue
			}

			for _, q := range msg.Question {
				if q.Name != chromecastLookupName {
					continue
				}

				resp := dns.Msg{}
				resp.SetReply(&msg)
				resp.Compress = true
				resp.RecursionDesired = false
				resp.Authoritative = true
				resp.Question = nil // RFC6762 section 6 "responses MUST NOT contain any questions"

				// MSG answers: 0) &dns.PTR{Hdr:dns.RR_Header{Name:"_googlecast._tcp.local.", Rrtype:0xc, Class:0x1, Ttl:0x78, Rdlength:0x33}, Ptr:"Nest-Wifi-point-8874243a15830f7844419710c67933f5._googlecast._tcp.local."}
				ptr := &dns.PTR{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypePTR,
						Class:  dns.ClassINET,
						Ttl:    120,
					},
					Ptr: simulatorDNS,
				}

				resp.Answer = []dns.RR{ptr}

				// MSG extras: 1) &dns.SRV{Hdr:dns.RR_Header{Name:"Nest-Wifi-point-8874243a15830f7844419710c67933f5._googlecast._tcp.local.", Rrtype:0x21, Class:0x8001, Ttl:0x78, Rdlength:0x2d}, Priority:0x0, Weight:0x0, Port:0x1f49, Target:"8874243a-1583-0f78-4441-9710c67933f5.local."}
				srv := &dns.SRV{
					Hdr: dns.RR_Header{
						Name:   simulatorDNS,
						Rrtype: dns.TypeSRV,
						Class:  dns.ClassINET,
						Ttl:    120,
					},
					Priority: 0,
					Weight:   0,
					Port:     uint16(port),
					Target:   simulatorID + ".local.",
				}

				// MSG extras: 2) &dns.A{Hdr:dns.RR_Header{Name:"8874243a-1583-0f78-4441-9710c67933f5.local.", Rrtype:0x1, Class:0x8001, Ttl:0x78, Rdlength:0x4}, A:net.IP{0xc0, 0xa8, 0x56, 0x1c}}<Paste>
				a := &dns.A{
					Hdr: dns.RR_Header{
						Name:   simulatorID + ".local.",
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    120,
					},
					A: ipv4IP,
				}
				/*
				   MSG extras: 0) &dns.TXT{Hdr:dns.RR_Header{Name:"Nest-Wifi-point-8874243a15830f7844419710c67933f5._googlecast._tcp.local.", Rrtype:0x10, Class:0x8001, Ttl:0x1194, Rdlength:0xc2}, Txt:[]string{"id=8874243a15830f7844419710c67933f5", "cd=B20EF1BF3BE2BD2AAD9C5596F282F14E", "rm=CB187BA191653173", "ve=05", "md=Nest Wifi point", "ic=/setup/icon.png", "fn=Bedroom Wifi 2", "ca=198660", "st=0", "bs=FA8FCA5E8DE9", "nf=1", "rs="}}
				*/
				txt := &dns.TXT{
					Hdr: dns.RR_Header{
						Name:   simulatorDNS,
						Rrtype: dns.TypeTXT,
						Class:  dns.ClassINET,
						Ttl:    120,
					},
					Txt: []string{
						"id=" + simulatorID,
						"md=" + simulatorType,
						"fn=" + simulatorName,
					},
				}

				resp.Extra = []dns.RR{srv, a, txt}

				packedAnswer, err := resp.Pack()
				if err != nil {
					log.Printf("error: unable to pack response: %v", err)
					continue
				}
				pc.WriteTo(packedAnswer, addr)
			}
		}
	}()
	return nil
}
