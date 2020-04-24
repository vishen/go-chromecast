package main

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/buger/jsonparser"
	"github.com/gogo/protobuf/proto"
	"github.com/miekg/dns"

	"github.com/vishen/go-chromecast/cast"
	pb "github.com/vishen/go-chromecast/cast/proto"
)

//go:generate openssl req -new -nodes -x509 -out certs/server.pem -keyout certs/server.key -days 1 -subj "/C=DE/ST=NRW/L=Earth/O=Chromecast/OU=IT/CN=chromecast/emailAddress=chromecast"

const (
	mdnsAddr             = "224.0.0.251:5353"
	chromecastLookupName = "_googlecast._tcp.local."
	simulatorType        = "ChromecastSimulator"
	simulatorName        = "chromecast-simulator-test"
	simulatorID          = "abcdef0123456789"
	// Google-Home-Mini-b87d86bed423a6feb8b91a7d2778b55c._googlecast._tcp.local.
	simulatorDNS = simulatorType + "-" + simulatorID + "." + chromecastLookupName
)

type State string

const (
	State_IDLE          = "IDLE"
	State_MEDIA_RUNNING = "MEDIA_RUNNING"
)

var (
	currentState State = State_MEDIA_RUNNING
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

	// TODO: Change this to use a cancellable context
	done := make(chan struct{})
	done <- struct{}{}
}

func handleCastConn(conn net.Conn) {
	// Close the connection on exit
	defer conn.Close()

	// Application to be used for the connection
	sessionID := "aaaaaaaa-bbbb-1111-2222-333333333333" // TODO: Should this be randomized?
	castApp := cast.Application{
		AppId:        "CastSimulator",
		DisplayName:  "Testing",
		IsIdleScreen: true, // NOTE: Needs to start off as true.
		SessionId:    sessionID,
		TransportId:  sessionID,
	}
	castVolume := cast.Volume{0.4, false}

	switch currentState {
	case State_IDLE:
		castApp.IsIdleScreen = true
	case State_MEDIA_RUNNING:
		// TODO:
		castApp.IsIdleScreen = false
	}

	for {
		var length uint32
		if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
			fmt.Printf("unable to binary read payload: %v", err)
			break
		}

		if length == 0 {
			continue
		}

		payload := make([]byte, length)
		if _, err := io.ReadFull(conn, payload); err != nil {
			fmt.Printf("unable to read payload: %v", err)
			break
		}

		msg := &pb.CastMessage{}
		if err := proto.Unmarshal(payload, msg); err != nil {
			log.Printf("unable to umarshal proto cast message: %v", err)
			continue
		}

		fmt.Printf(
			"sourceid=%s destid=%s namespace=%s payload=%s\n",
			msg.GetSourceId(),
			msg.GetDestinationId(),
			msg.GetNamespace(),
			msg.GetPayloadUtf8(),
		)

		messageType, err := jsonparser.GetString([]byte(msg.GetPayloadUtf8()), "type")
		if err != nil {
			fmt.Printf("unable to get message type: %v\n", err)
			break
		}

		requestID, err := jsonparser.GetInt([]byte(msg.GetPayloadUtf8()), "requestId")
		if err != nil {
			fmt.Printf("unable to get requestId: %v\n", err)
			break
		}

		payloadHeader := cast.PayloadHeader{
			RequestId: int(requestID),
		}

		switch messageType {
		case "PING":
			// Handle
		case "CONNECT":
			// What to do here?
		case "GET_STATUS":
			payloadHeader.Type = "RECEIVER_STATUS"
			if err := sendResponse(conn, msg, &cast.ReceiverStatusResponse{
				PayloadHeader: payloadHeader,
				Status: cast.ReceiverStatus{
					Applications: []cast.Application{castApp},
					Volume:       castVolume,
				},
			}); err != nil {
				break
			}
		}
	}
}

func sendResponse(conn net.Conn, msg *pb.CastMessage, payload cast.Payload) error {
	payloadJson, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	payloadUtf8 := string(payloadJson)
	message := &pb.CastMessage{
		ProtocolVersion: pb.CastMessage_CASTV2_1_0.Enum(),
		SourceId:        msg.DestinationId,
		DestinationId:   msg.SourceId,
		Namespace:       msg.Namespace,
		PayloadType:     pb.CastMessage_STRING.Enum(),
		PayloadUtf8:     &payloadUtf8,
	}
	proto.SetDefaults(message)
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	if err := binary.Write(conn, binary.BigEndian, uint32(len(data))); err != nil {
		return err
	}
	if _, err := conn.Write(data); err != nil {
		return err
	}
	return nil
}

func startTCPServer() (addr net.Addr, err error) {
	cert, err := tls.LoadX509KeyPair("certs/server.pem", "certs/server.key")
	if err != nil {
		return addr, err
	}

	config := tls.Config{Certificates: []tls.Certificate{cert}}
	ln, err := tls.Listen("tcp4", "", &config)
	if err != nil {
		return addr, err
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("error: unable to accept connection: %v", err)
				continue
			}
			fmt.Printf("listener conn: %#v\n", conn)
			go handleCastConn(conn)
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
