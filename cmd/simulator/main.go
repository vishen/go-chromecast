package main

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"path/filepath"

	"golang.org/x/net/ipv4"

	"github.com/buger/jsonparser"
	"github.com/gogo/protobuf/proto"
	"github.com/miekg/dns"

	"github.com/vishen/go-chromecast/cast"
	pb "github.com/vishen/go-chromecast/cast/proto"
	"github.com/vishen/go-chromecast/cmd/simulator/simulator"
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

// Flags
var (
	certsFolder    = flag.String("certs", "certs/", "certs folder location")
	initialState   = flag.String("state", "playing", "initial state of the chromecast simulator: idle or playing")
	printVerbose   = flag.Bool("verbose", true, "verbose logging")
	verbosityLevel = flag.Int("verbosity-level", 2, "verbosity level 1-3")
)

// Global Variables
var (
	chromecast *simulator.Chromecast
)

func main() {
	flag.Parse()

	var state simulator.State
	switch s := *initialState; s {
	case "idle":
		state = simulator.State_IDLE
	case "playing":
		state = simulator.State_PLAYING
	default:
		log.Fatalf("%q is not a valid 'state'", s)
	}

	chromecast = simulator.NewChromecast(state)

	verbose(1, "serving %q...", simulatorDNS)
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatal(err)
	}

	// Get Local IP
	var localIP string
	var interfaces []net.Interface
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback == net.FlagLoopback {
			continue
		}
		if (iface.Flags&net.FlagUp > 0) && (iface.Flags&net.FlagMulticast > 0) {
			interfaces = append(interfaces, iface)
		}
		addrs, err := iface.Addrs()
		if err != nil {
			verbose(1, "unable to get addresses for interface %q: %v", iface.Name, err)
			continue
		}
		for _, addr := range addrs {
			if netIP, ok := addr.(*net.IPNet); ok {
				if ipv4 := netIP.IP.To4(); ipv4 != nil {
					localIP = ipv4.String()
					verbose(1, "using iface=%q, ip=%q", iface.Name, localIP)
					break
				}
			}
		}
		if localIP != "" {
			break
		}
	}

	if localIP == "" {
		log.Fatal("unable to find a non-local ip address")
	}

	go func() {
		listenerAddr, err := startTCPServer(*certsFolder)
		if err != nil {
			log.Fatal(err)
		}
		// Get port from listener
		var listenerAddrPort int
		tcpAddr, ok := listenerAddr.(*net.TCPAddr)
		if !ok {
			log.Fatal("unable to get port from tcp address")
		}
		listenerAddrPort = tcpAddr.Port

		verbose(1, "simulator listening on %s:%d", localIP, listenerAddrPort)

		if err := startMDNSServer(localIP, listenerAddrPort, interfaces); err != nil {
			log.Fatal(err)
		}
	}()

	chromecast.Wait()
}

func verbose(level int, msg string, args ...interface{}) {
	if *printVerbose && level <= *verbosityLevel {
		log.Printf(msg, args...)
	}
}

func handleCastConn(conn net.Conn) {
	for {
		var length uint32
		if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
			break
		}

		if length == 0 {
			continue
		}

		payload := make([]byte, length)
		if _, err := io.ReadFull(conn, payload); err != nil {
			break
		}

		msg := &pb.CastMessage{}
		if err := proto.Unmarshal(payload, msg); err != nil {
			log.Printf("unable to umarshal proto cast message: %v", err)
			continue
		}

		verbose(2,
			"RECEIVED: sourceid=%s destid=%s namespace=%s payload=%s",
			msg.GetSourceId(),
			msg.GetDestinationId(),
			msg.GetNamespace(),
			msg.GetPayloadUtf8(),
		)

		messageType, err := jsonparser.GetString([]byte(msg.GetPayloadUtf8()), "type")
		if err != nil {
			verbose(1, "unable to get message type: %v", err)
			break
		}

		requestID, err := jsonparser.GetInt([]byte(msg.GetPayloadUtf8()), "requestId")
		if err != nil {
			verbose(1, "unable to get requestId: %v", err)
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
		case "STOP":
			chromecast.Stop()
		case "GET_STATUS":
			chromecast.Update()
			switch *msg.Namespace {
			case "urn:x-cast:com.google.cast.receiver":
				payloadHeader.Type = "RECEIVER_STATUS"
				if err := sendResponse(conn, msg, &cast.ReceiverStatusResponse{
					PayloadHeader: payloadHeader,
					Status: cast.ReceiverStatus{
						Applications: []cast.Application{chromecast.Application()},
						Volume:       chromecast.Volume(),
					},
				}); err != nil {
					break
				}
			case "urn:x-cast:com.google.cast.media":
				payloadHeader.Type = "RECEIVER_STATUS"
				if err := sendResponse(conn, msg, &cast.MediaStatusResponse{
					PayloadHeader: payloadHeader,
					Status:        []cast.Media{chromecast.Media()},
				}); err != nil {
					break
				}
			}
		case "SET_VOLUME":
			// RECEIVED: sourceid=sender-0 destid=receiver-0 namespace=urn:x-cast:com.google.cast.receiver payload={"type":"SET_VOLUME","requestId":5,"volume":{"level":0.5,"muted":false}}
			// TODO: This might be faster to parse into a struct rather than doing it twice?
			volumeLevel, err := jsonparser.GetFloat([]byte(msg.GetPayloadUtf8()), "volume", "level")
			if err != nil {
				verbose(1, "unable to get volume level: %v", err)
				break
			}
			volumeMuted, err := jsonparser.GetBoolean([]byte(msg.GetPayloadUtf8()), "volume", "muted")
			if err != nil {
				verbose(1, "unable to get volume muted: %v", err)
				break
			}
			chromecast.SetVolume(float32(volumeLevel), volumeMuted)
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
	verbose(2,
		"SENDIND: sourceid=%s destid=%s namespace=%s payload=%s",
		message.GetSourceId(),
		message.GetDestinationId(),
		message.GetNamespace(),
		message.GetPayloadUtf8(),
	)
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

func startTCPServer(certsFolder string) (addr net.Addr, err error) {
	pem := filepath.Join(certsFolder, "server.pem")
	key := filepath.Join(certsFolder, "server.key")
	cert, err := tls.LoadX509KeyPair(pem, key)
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
				verbose(1, "error: unable to accept connection: %v", err)
				continue
			}
			go func() {
				handleCastConn(conn)
				conn.Close()
			}()
		}
	}()
	return ln.Addr(), nil
}

func startMDNSServer(ip string, port int, interfaces []net.Interface) error {
	// https://en.wikipedia.org/wiki/Multicast_DNS
	// https://github.com/grandcat/zeroconf/blob/master/server.go
	// listen to incoming udp packets
	pc, err := net.ListenPacket("udp4", mdnsAddr)
	if err != nil {
		return err
	}

	verbose(2, "listening for mdns on %s: %#v", mdnsAddr, pc)

	// Join multicast groups to receive announcements
	pkConn := ipv4.NewPacketConn(pc.(*net.UDPConn))
	pkConn.SetControlMessage(ipv4.FlagInterface, true)

	for _, iface := range interfaces {
		pkConn.JoinGroup(&iface, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251)})
	}

	ipv4IP := net.ParseIP(ip)
	go func() {
		defer pc.Close()
		for {
			// TODO: is 1024 big enough? What is the size of a udp packet?
			buf := make([]byte, 1024)
			_, addr, err := pc.ReadFrom(buf)
			if err != nil {
				continue
			}
			var msg dns.Msg
			if err := msg.Unpack(buf); err != nil {
				continue
			}

			for _, q := range msg.Question {
				if q.Name != chromecastLookupName {
					continue
				}

				verbose(3, "mdns: query: %#v", q)

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

				verbose(3, "mdns: replying: %+v", resp)
				packedAnswer, err := resp.Pack()
				if err != nil {
					verbose(1, "error: unable to pack response: %v", err)
					continue
				}
				pc.WriteTo(packedAnswer, addr)
			}
		}
	}()
	return nil
}
