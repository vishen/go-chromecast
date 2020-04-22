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
)

func main() {
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
				fmt.Printf("dns msg: %#v\n", msg)
				fmt.Printf("dns msg question: %#v\n", q)

				resp := dns.Msg{}
				resp.SetReply(&msg)
				resp.Compress = true
				resp.RecursionDesired = false
				resp.Authoritative = true
				resp.Question = nil // RFC6762 section 6 "responses MUST NOT contain any questions"

				ptr := &dns.PTR{
					Hdr: dns.RR_Header{
						Name:   chromecastLookupName,
						Rrtype: dns.TypePTR,
						Class:  dns.ClassINET,
						Ttl:    120,
					},
					Ptr: "chromecast-simulator",
				}

				resp.Answer = []dns.RR{ptr}

				srv := &dns.SRV{
					Hdr: dns.RR_Header{
						Name:   chromecastLookupName,
						Rrtype: dns.TypeSRV,
						Class:  dns.ClassINET,
						Ttl:    120,
					},
					Priority: 0,
					Weight:   0,
					Port:     uint16(port),
					Target:   ip,
				}

				a := &dns.A{
					Hdr: dns.RR_Header{
						Name:   "chromecast-simulator",
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    120,
					},
					A: net.IP{192, 168, 12, 13},
				}

				resp.Extra = []dns.RR{srv, a}

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
