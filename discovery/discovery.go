// Package discovery is used to discover devices on the network using a provided scanner
package discovery

import (
	"context"
	"fmt"
	"sort"
)

// Scanner scans for chromecast and pushes them onto the results channel (eventually multiple times)
// It must return immediately and scan in a different goroutine
// The results channel must be closed when the ctx is done
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

// Uniq scans until cancellation of the context and returns a map of chromecast devices by ID
func (s Service) Uniq(ctx context.Context, matchers ...DeviceMatcher) (map[string]*Device, error) {
	scanned := make(chan *Device, 5)

	err := s.Scanner.Scan(ctx, scanned)
	if err != nil {
		return nil, fmt.Errorf("could not initiliaze scanner: %v", err)
	}
	match := matchAll(matchers...)
	found := make(map[string]*Device)
	for {
		select {
		case <-ctx.Done():
			return found, nil
		case device := <-scanned:
			if match(device) {
				found[device.ID()] = device
			}
		}
	}
}

// Sorted scans until cancellation of the context and returns a sorted list of chromecast devices
func (s Service) Sorted(ctx context.Context, matchers ...DeviceMatcher) ([]*Device, error) {
	found, err := s.Uniq(ctx, matchers...)
	if err != nil {
		return nil, err
	}
	result := make([]*Device, 0, len(found))
	for _, d := range found {
		result = append(result, d)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name() < result[j].Name() })
	return result, nil
}
