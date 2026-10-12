package cmd

import (
	"net"
	"testing"
)

func TestScanCIDR(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{"192.168.1.50/24", "192.168.1.0/24"},
		{"192.168.50.7/24", "192.168.50.0/24"},
		{"10.0.0.5/8", "10.0.0.0/24"},
		{"10.1.2.3/16", "10.1.2.0/24"},
		{"172.16.5.130/25", "172.16.5.128/25"},
		{"127.0.0.1/8", ""},
		{"8.8.8.8/24", ""},
		{"fe80::1/64", ""},
	}
	for _, tt := range tests {
		ip, ipnet, err := net.ParseCIDR(tt.addr)
		if err != nil {
			t.Fatalf("unable to parse %q: %v", tt.addr, err)
		}
		ipnet.IP = ip
		if got := scanCIDR(ipnet); got != tt.want {
			t.Errorf("scanCIDR(%q) = %q, want %q", tt.addr, got, tt.want)
		}
	}
}
