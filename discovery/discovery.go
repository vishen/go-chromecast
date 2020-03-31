// Package discovery is used to discover devices on the network using a scanner
package discovery

import (
	"context"
	"fmt"
)

// Scanner scans for chromecast and pushes them onto the results channel (eventually multiple times)
// It must return immediately and scan in a different goroutine
// The the results channel must be closed  when the ctx is done
type Scanner interface {
	Scan(ctx context.Context, results chan<- *Device) error
}

// Service allows to discover chromecast via the given scanner
type Service struct {
	Scanner Scanner
}

// First returns the first chromecast that is discovered by the scanner (matching all matchers - if any)
func (s Service) First(ctx context.Context, matchers ...DeviceMatcher) (*Device, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // cancel child-ctx when the right client has been found

	result := make(chan *Device, 1)

	err := s.Scanner.Scan(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("could not initiliaze scanner: %v", err)
	}
	match := matchAll(matchers...)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case device := <-result:
			if match(device) {
				return device, nil
			}
		}
	}
}
