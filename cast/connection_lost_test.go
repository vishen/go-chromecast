package cast

import (
	"crypto/tls"
	"io"
	"net"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// silentServer accepts tls connections and reads whatever it is sent, but
// never sends anything back. If closeAfterAccept is set, it closes the
// connections straight away instead.
func silentServer(t *testing.T, closeAfterAccept bool) (addr string, port int) {
	// Borrow a certificate from httptest.
	certServer := httptest.NewUnstartedServer(nil)
	certServer.StartTLS()
	certificates := certServer.TLS.Certificates
	certServer.Close()

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: certificates})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				if err := conn.(*tls.Conn).Handshake(); err != nil || closeAfterAccept {
					return
				}
				io.Copy(io.Discard, conn)
			}()
		}
	}()

	host, portString, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port, err = strconv.Atoi(portString)
	require.NoError(t, err)
	return host, port
}

func TestConnectionLost(t *testing.T) {
	send := func(c *Connection) error {
		return c.Send(1, &PayloadHeader{Type: "GET_STATUS"}, "sender-0", "receiver-0", "namespace")
	}

	t.Run("when nothing is received for too long", func(t *testing.T) {
		addr, port := silentServer(t, false)

		c := NewConnection()
		c.readTimeout = 100 * time.Millisecond
		require.NoError(t, c.Start(addr, port))
		defer c.Close()

		require.NoError(t, send(c))
		require.Eventually(t, func() bool { return send(c) == ErrConnectionLost }, 2*time.Second, 20*time.Millisecond)
	})

	t.Run("when the connection is closed by the device", func(t *testing.T) {
		addr, port := silentServer(t, true)

		c := NewConnection()
		require.NoError(t, c.Start(addr, port))
		defer c.Close()

		require.Eventually(t, func() bool { return send(c) == ErrConnectionLost }, 2*time.Second, 20*time.Millisecond)
	})
}
