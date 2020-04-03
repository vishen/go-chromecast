package discovery

import (
	"fmt"
	"net"
	"strings"
)

// NewDevice returns an new chromecast device
func NewDevice(ip net.IP, port int, properties []string) *Device {
	return &Device{
		IP:         ip,
		Port:       port,
		Properties: parseProperties(properties),
	}
}

// Device represents a device discoverd on the network
type Device struct {
	IP         net.IP
	Port       int
	Properties map[string]string
}

// Compatibility with cmd.CastDNSEntry
func (d Device) GetName() string {
	return d.Name()
}
func (d Device) GetUUID() string {
	return d.ID()
}
func (d Device) GetAddr() string {
	return d.IP.String()
}
func (d Device) GetPort() int {
	return d.Port
}

// Addr return the ip and port of the device
func (d Device) Addr() string {
	return fmt.Sprintf("%s:%d", d.IP, d.Port)
}

// Name of the device
func (d Device) Name() string {
	return d.Properties["fn"]
}

// ID of the device (example: 7a5fd8ff1f425150a79ec0e36f497445)
func (d Device) ID() string {
	return d.Properties["id"]
}

// Type of the device (examples: Chromecast, Google Home Mini)
func (d Device) Type() string {
	return d.Properties["md"]
}

// Status of the device
func (d Device) Status() string {
	return d.Properties["rs"]
}

// parseProperties into a string map
// Input: {"key1=value1", "key2=value2"}
func parseProperties(s []string) map[string]string {
	m := make(map[string]string, len(s))
	for _, v := range s {
		s := strings.SplitN(v, "=", 2)
		if len(s) == 2 {
			m[s[0]] = s[1]
		}
	}
	return m
}
