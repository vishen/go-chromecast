package cast

import (
	"crypto/tls"
	"encoding/binary"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	pb "github.com/vishen/go-chromecast/cast/proto"
)

// fakeDevice starts a TLS server that keeps sending cast messages to
// whoever connects, as fast as it can.
func fakeDevice(t *testing.T) (addr string, port int) {
	t.Helper()

	// Borrow the test certificate from httptest.
	ts := httptest.NewUnstartedServer(nil)
	ts.StartTLS()
	cert := ts.TLS.Certificates[0]
	ts.Close()

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatalf("unable to listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	sourceID, destinationID, namespace := "receiver-0", "sender-0", "urn:x-cast:com.google.cast.receiver"
	payload := `{"type":"RECEIVER_STATUS","requestId":1}`
	message := &pb.CastMessage{
		ProtocolVersion: pb.CastMessage_CASTV2_1_0.Enum(),
		SourceId:        &sourceID,
		DestinationId:   &destinationID,
		Namespace:       &namespace,
		PayloadType:     pb.CastMessage_STRING.Enum(),
		PayloadUtf8:     &payload,
	}
	data, err := proto.Marshal(message)
	if err != nil {
		t.Fatalf("unable to marshal message: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				for {
					if err := binary.Write(conn, binary.BigEndian, uint32(len(data))); err != nil {
						return
					}
					if _, err := conn.Write(data); err != nil {
						return
					}
				}
			}()
		}
	}()

	tcpAddr := listener.Addr().(*net.TCPAddr)
	return tcpAddr.IP.String(), tcpAddr.Port
}

// Closing the connection while messages are still being received used to
// panic with "send on closed channel".
func TestConnectionCloseWhileReceiving(t *testing.T) {
	addr, port := fakeDevice(t)

	for i := 0; i < 50; i++ {
		c := NewConnection()
		if err := c.Start(addr, port); err != nil {
			t.Fatalf("unable to start connection: %v", err)
		}
		// Read some messages, like the application does, and then stop
		// reading and close the connection.
		for j := 0; j < 10; j++ {
			select {
			case <-c.MsgChan():
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for a message")
			}
		}
		if err := c.Close(); err != nil {
			t.Fatalf("unable to close connection: %v", err)
		}
		// The channel must end up closed so readers don't block forever.
		timeout := time.After(5 * time.Second)
	drain:
		for {
			select {
			case _, ok := <-c.MsgChan():
				if !ok {
					break drain
				}
			case <-timeout:
				t.Fatal("message channel was not closed")
			}
		}
	}
}
