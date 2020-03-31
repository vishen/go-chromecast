// Package zeroconf provides a Scanner backed by the github.com/grandcat/zeroconf package
package zeroconf

import (
	"fmt"
	"net"
	"strings"

	"github.com/grandcat/zeroconf"
	"github.com/sirupsen/logrus"
	"github.com/vishen/go-chromecast/discovery"

	"context"
)

// Scanner backed by the github.com/grandcat/zeroconf package
// Nil values uses the default
type Scanner struct {
	Logger        logrus.FieldLogger
	ClientOptions []zeroconf.ClientOption
}

// Scan repeatedly scans the network and sends the chromecast found into the results channel.
// It finishes when the context is done.
func (s Scanner) Scan(ctx context.Context, results chan<- *discovery.Device) error {
	// generate entries
	// Discover all services on the network (e.g. _workstation._tcp)
	resolver, err := zeroconf.NewResolver(s.ClientOptions...)
	if err != nil {
		return fmt.Errorf("failed to initialize resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 5)
	err = resolver.Browse(ctx, "_googlecast._tcp", "local", entries)
	if err != nil {
		return fmt.Errorf("fail to browse services: %w", err)
	}

	go func() {
		defer close(results)
		// decode entries
		for e := range entries {
			c, err := s.decode(e)
			if err != nil {
				if s.Logger != nil {
					s.Logger.Errorf("could not decode: %w", err)
				}
				continue
			}
			select {
			case results <- c:
				continue
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// decode turns an zeroconf.ServiceEntry into a discovery.Device
func (s Scanner) decode(entry *zeroconf.ServiceEntry) (*discovery.Device, error) {
	if !strings.Contains(entry.Service, "_googlecast.") {
		return nil, fmt.Errorf("fdqn '%s does not contain '_googlecast.'", entry.Service)
	}

	var ip net.IP
	if len(entry.AddrIPv6) > 0 {
		ip = entry.AddrIPv6[0]
	} else if len(entry.AddrIPv4) > 0 {
		ip = entry.AddrIPv4[0]
	}

	return discovery.NewDevice(ip, entry.Port, entry.Text), nil
}
