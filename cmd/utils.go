package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/seancfoley/ipaddress-go/ipaddr"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/application"
	castdns "github.com/vishen/go-chromecast/dns"
	"github.com/vishen/go-chromecast/storage"
)

var (
	cache = storage.NewStorage()

	// Set up a global dns entry so we can attempt reconnects
	entry castdns.CastDNSEntry
)

type CachedDNSEntry struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
	Addr string `json:"addr"`
	Port int    `json:"port"`
}

func (e CachedDNSEntry) GetUUID() string {
	return e.UUID
}

func (e CachedDNSEntry) GetName() string {
	return e.Name
}

func (e CachedDNSEntry) GetAddr() string {
	return e.Addr
}

func (e CachedDNSEntry) GetPort() int {
	return e.Port
}

func castApplication(cmd *cobra.Command, args []string) (application.App, error) {
	// Handle broad-search flag internally
	broadSearch, _ := cmd.Flags().GetBool("broad-search")
	if broadSearch {
		return castApplicationWithBroadSearch(cmd, args)
	}
	deviceName, _ := cmd.Flags().GetString("device-name")
	deviceUuid, _ := cmd.Flags().GetString("uuid")
	device, _ := cmd.Flags().GetString("device")
	debug, _ := cmd.Flags().GetBool("debug")
	disableCache, _ := cmd.Flags().GetBool("disable-cache")
	addr, _ := cmd.Flags().GetString("addr")
	port, _ := cmd.Flags().GetString("port")
	ifaceName, _ := cmd.Flags().GetString("iface")
	serverPort, _ := cmd.Flags().GetInt("server-port")
	dnsTimeoutSeconds, _ := cmd.Flags().GetInt("dns-timeout")
	useFirstDevice, _ := cmd.Flags().GetBool("first")

	// Used to try and reconnect
	if deviceUuid == "" && entry != nil {
		deviceUuid = entry.GetUUID()
		entry = nil
	}

	if debug {
		log.SetLevel(log.DebugLevel)
	}

	applicationOptions := []application.ApplicationOption{
		application.WithServerPort(serverPort),
		application.WithDebug(debug),
		application.WithCacheDisabled(disableCache),
	}

	// If we need to look on a specific network interface for mdns or
	// for finding a network ip to host from, ensure that the network
	// interface exists.
	var iface *net.Interface
	if ifaceName != "" {
		var err error
		if iface, err = net.InterfaceByName(ifaceName); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("unable to find interface %q", ifaceName))
		}
		applicationOptions = append(applicationOptions, application.WithIface(iface))
	} else {
		// If no interface was specified, try to auto-detect the best interface
		if autoIface, err := detectBestInterface(); err == nil {
			iface = autoIface
			applicationOptions = append(applicationOptions, application.WithIface(iface))
		}
	}

	// If no address was specified, attempt to determine the address of any
	// local chromecast devices.
	if addr == "" {
		// If a device name or uuid was specified, check the cache for the ip+port
		found := false
		if !disableCache && (deviceName != "" || deviceUuid != "") {
			entry = findCachedCastDNS(deviceName, deviceUuid)
			found = entry.GetAddr() != ""
		}
		if !found {
			var err error
			if entry, err = findCastDNS(iface, dnsTimeoutSeconds, device, deviceName, deviceUuid, useFirstDevice); err != nil {
				return nil, errors.Wrap(err, "unable to find cast dns entry")
			}
		}
		if !disableCache {
			cachedEntry := CachedDNSEntry{
				UUID: entry.GetUUID(),
				Name: entry.GetName(),
				Addr: entry.GetAddr(),
				Port: entry.GetPort(),
			}
			cachedEntryJson, _ := json.Marshal(cachedEntry)
			if err := cache.Save(getCacheKey(cachedEntry.UUID), cachedEntryJson); err != nil {
				outputError("Failed to save UUID cache entry\n")
			}
			if err := cache.Save(getCacheKey(cachedEntry.Name), cachedEntryJson); err != nil {
				outputError("Failed to save name cache entry\n")
			}
		}
		if debug {
			outputInfo("using device name=%s addr=%s port=%d uuid=%s", entry.GetName(), entry.GetAddr(), entry.GetPort(), entry.GetUUID())
		}
	} else {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, errors.Wrap(err, "port needs to be a number")
		}
		entry = CachedDNSEntry{
			Addr: addr,
			Port: p,
		}
	}
	app := application.NewApplication(applicationOptions...)
	if err := app.Start(entry.GetAddr(), entry.GetPort()); err != nil {
		// NOTE: currently we delete the dns cache every time we get
		// an error, this is to make sure that if the device gets a new
		// ipaddress we will invalidate the cache.
		if err := cache.Save(getCacheKey(entry.GetUUID()), []byte{}); err != nil {
			fmt.Printf("Failed to save UUID cache entry: %v\n", err)
		}
		if err := cache.Save(getCacheKey(entry.GetName()), []byte{}); err != nil {
			fmt.Printf("Failed to save name cache entry: %v\n", err)
		}
		return nil, err
	}
	return app, nil
}

// reconnect will attempt to reconnect to the cast device
// TODO: This is all very hacky, currently a global dns entry is set which
// contains the device UUID, and this is then used to reconnect. This should
// be handled much nicer and we shouldn't need to pass around the cmd and args everywhere
// just to reconnect. This might require adding something that wraps the application and
// dns?
func reconnect(cmd *cobra.Command, args []string) (application.App, error) {
	return castApplication(cmd, args)
}

func getCacheKey(suffix string) string {
	return fmt.Sprintf("cmd/utils/dns/%s", suffix)
}

func findCachedCastDNS(deviceName, deviceUuid string) castdns.CastDNSEntry {
	for _, s := range []string{deviceName, deviceUuid} {
		cacheKey := getCacheKey(s)
		if b, err := cache.Load(cacheKey); err == nil {
			cachedEntry := CachedDNSEntry{}
			if err := json.Unmarshal(b, &cachedEntry); err == nil {
				return cachedEntry
			}
		}
	}
	return CachedDNSEntry{}
}

func findCastDNS(iface *net.Interface, dnsTimeoutSeconds int, device, deviceName, deviceUuid string, first bool) (castdns.CastDNSEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(dnsTimeoutSeconds))
	defer cancel()
	castEntryChan, err := castdns.DiscoverCastDNSEntries(ctx, iface)
	if err != nil {
		return castdns.CastEntry{}, err
	}

	isDeviceFilter := deviceUuid != "" || deviceName != "" || device != ""

	foundEntries := []castdns.CastEntry{}
	for entry := range castEntryChan {
		if first && !isDeviceFilter {
			return entry, nil
		} else if (deviceUuid != "" && entry.UUID == deviceUuid) || (deviceName != "" && entry.DeviceName == deviceName) || (device != "" && entry.Device == device) {
			return entry, nil
		}
		foundEntries = append(foundEntries, entry)
	}

	if len(foundEntries) == 0 || isDeviceFilter {
		return castdns.CastEntry{}, fmt.Errorf("no cast devices found on network")
	}

	// Always return entries in deterministic order.
	sort.Slice(foundEntries, func(i, j int) bool { return foundEntries[i].DeviceName < foundEntries[j].DeviceName })

	outputInfo("Found %d cast dns entries, select one:", len(foundEntries))
	for i, d := range foundEntries {
		outputInfo("%d) device=%q device_name=%q address=\"%s:%d\" uuid=%q", i+1, d.Device, d.DeviceName, d.AddrV4, d.Port, d.UUID)
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Enter selection: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("error reading console: %v\n", err)
			continue
		}
		i, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			continue
		} else if i < 1 || i > len(foundEntries) {
			continue
		}
		return foundEntries[i-1], nil
	}
}

// findCastDNSWithBroadSearch is like findCastDNS but uses comprehensive search
func findCastDNSWithBroadSearch(iface *net.Interface, dnsTimeoutSeconds int, device, deviceName, deviceUuid string, first bool) (castdns.CastDNSEntry, error) {
	// First try normal mDNS discovery with extended timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(dnsTimeoutSeconds*3))
	defer cancel()
	castEntryChan, err := castdns.DiscoverCastDNSEntries(ctx, iface)
	if err != nil {
		return castdns.CastEntry{}, err
	}

	isDeviceFilter := deviceUuid != "" || deviceName != "" || device != ""

	foundEntries := []castdns.CastEntry{}
	for entry := range castEntryChan {
		if first && !isDeviceFilter {
			return entry, nil
		} else if (deviceUuid != "" && entry.UUID == deviceUuid) || (deviceName != "" && entry.DeviceName == deviceName) || (device != "" && entry.Device == device) {
			return entry, nil
		}
		foundEntries = append(foundEntries, entry)
	}

	// If we found devices via mDNS and we're looking for a specific one, show the list
	if len(foundEntries) > 0 && isDeviceFilter {
		return castdns.CastEntry{}, fmt.Errorf("no cast devices found matching criteria")
	}

	// If no devices found via mDNS, try port scanning as fallback
	if len(foundEntries) == 0 {
		outputInfo("No devices found via mDNS, trying port scan...")
		
		if localSubnet, err := detectLocalSubnet(""); err == nil {
			if scannedDevices := performPortScanForDevices(localSubnet); len(scannedDevices) > 0 {
				// Convert scanned devices to CastEntry format
				for _, dev := range scannedDevices {
					entry := castdns.CastEntry{
						Device:     dev.Device,
						DeviceName: dev.DeviceName,
						AddrV4:     net.ParseIP(dev.AddrV4),
						Port:       dev.Port,
						UUID:       dev.UUID,
					}
					
					if first && !isDeviceFilter {
						return entry, nil
					} else if (deviceUuid != "" && entry.UUID == deviceUuid) || (deviceName != "" && entry.DeviceName == deviceName) || (device != "" && entry.Device == device) {
						return entry, nil
					}
					foundEntries = append(foundEntries, entry)
				}
			}
		}
	}

	if len(foundEntries) == 0 || isDeviceFilter {
		return castdns.CastEntry{}, fmt.Errorf("no cast devices found on network")
	}

	// Always return entries in deterministic order.
	sort.Slice(foundEntries, func(i, j int) bool { return foundEntries[i].DeviceName < foundEntries[j].DeviceName })

	outputInfo("Found %d cast dns entries, select one:", len(foundEntries))
	for i, d := range foundEntries {
		outputInfo("%d) device=%q device_name=%q address=\"%s:%d\" uuid=%q", i+1, d.Device, d.DeviceName, d.AddrV4, d.Port, d.UUID)
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Enter selection: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("error reading console: %v\n", err)
			continue
		}
		i, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			continue
		} else if i < 1 || i > len(foundEntries) {
			continue
		}
		return foundEntries[i-1], nil
	}
}

func outputError(msg string, args ...interface{}) {
	output(output_Error, msg, args...)
}

func outputInfo(msg string, args ...interface{}) {
	output(output_Info, msg, args...)
}

func exit(msg string, args ...interface{}) {
	outputError(msg, args...)
	os.Exit(1)
}

type outputLevel int

const (
	output_Info outputLevel = iota
	output_Error
)

func output(t outputLevel, msg string, args ...interface{}) {
	switch t {
	case output_Error:
		fmt.Printf("%serror%s: ", RED, NC)
	}
	if !strings.HasSuffix(msg, "\n") {
		msg = msg + "\n"
	}
	fmt.Printf(msg, args...)
}

const (
	RED = "\033[0;31m"
	NC  = "\033[0m" // No Color
)

// performPortScanForDevices scans the network for Chromecast devices
func performPortScanForDevices(subnet string) []CastDevice {
	var devices []CastDevice
	deviceMap := make(map[string]CastDevice)
	
	ipRange, err := ipaddr.NewIPAddressString(subnet).ToSequentialRange()
	if err != nil {
		return devices
	}
	
	// Use a smaller set of ports for command-line tools
	ports := []int{8009, 8008, 8443, 32236}
	
	var wg sync.WaitGroup
	ipCh := make(chan *ipaddr.IPAddress, 100)
	
	// Send IPs to scan
	go func() {
		it := ipRange.Iterator()
		for it.HasNext() {
			ip := it.Next()
			ipCh <- ip
		}
		close(ipCh)
	}()
	
	// Scan IPs in parallel
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dialer := &net.Dialer{
				Timeout: 300 * time.Millisecond,
			}
			for ip := range ipCh {
				for _, port := range ports {
					conn, err := dialer.Dial("tcp", fmt.Sprintf("%v:%d", ip, port))
					if err != nil {
						continue
					}
					conn.Close()
					
					// Try to get device info
					if info, err := application.GetInfo(ip.String()); err == nil {
						device := CastDevice{
							Device:     "Unknown Device",
							DeviceName: info.Name,
							AddrV4:     ip.String(),
							Port:       port,
							UUID:       "",
						}
						
						key := fmt.Sprintf("%s:%d", device.AddrV4, device.Port)
						if _, exists := deviceMap[key]; !exists {
							deviceMap[key] = device
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	
	// Convert map to slice
	for _, device := range deviceMap {
		devices = append(devices, device)
	}
	
	return devices
}

// detectBestInterface attempts to detect the best network interface for Chromecast communication
func detectBestInterface() (*net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					// Found a valid IPv4 interface
					return &iface, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("could not detect suitable network interface")
}

// castApplicationWithBroadSearch is like castApplication but uses broader device discovery
func castApplicationWithBroadSearch(cmd *cobra.Command, args []string) (application.App, error) {
	deviceName, _ := cmd.Flags().GetString("device-name")
	deviceUuid, _ := cmd.Flags().GetString("uuid")
	device, _ := cmd.Flags().GetString("device")
	debug, _ := cmd.Flags().GetBool("debug")
	disableCache, _ := cmd.Flags().GetBool("disable-cache")
	addr, _ := cmd.Flags().GetString("addr")
	port, _ := cmd.Flags().GetString("port")
	ifaceName, _ := cmd.Flags().GetString("iface")
	serverPort, _ := cmd.Flags().GetInt("server-port")
	dnsTimeoutSeconds, _ := cmd.Flags().GetInt("dns-timeout")
	useFirstDevice, _ := cmd.Flags().GetBool("first")

	// Used to try and reconnect
	if deviceUuid == "" && entry != nil {
		deviceUuid = entry.GetUUID()
		entry = nil
	}

	if debug {
		log.SetLevel(log.DebugLevel)
	}

	applicationOptions := []application.ApplicationOption{
		application.WithServerPort(serverPort),
		application.WithDebug(debug),
		application.WithCacheDisabled(disableCache),
	}

	// If we need to look on a specific network interface for mdns or
	// for finding a network ip to host from, ensure that the network
	// interface exists.
	var iface *net.Interface
	if ifaceName != "" {
		var err error
		if iface, err = net.InterfaceByName(ifaceName); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("unable to find interface %q", ifaceName))
		}
		applicationOptions = append(applicationOptions, application.WithIface(iface))
	} else {
		// If no interface was specified, try to auto-detect the best interface
		if autoIface, err := detectBestInterface(); err == nil {
			iface = autoIface
			applicationOptions = append(applicationOptions, application.WithIface(iface))
		}
	}

	// If no address was specified, attempt to determine the address of any
	// local chromecast devices using broad search.
	if addr == "" {
		// If a device name or uuid was specified, check the cache for the ip+port
		found := false
		if !disableCache && (deviceName != "" || deviceUuid != "") {
			entry = findCachedCastDNS(deviceName, deviceUuid)
			found = entry.GetAddr() != ""
		}
		if !found {
			var err error
			if entry, err = findCastDNSWithBroadSearch(iface, dnsTimeoutSeconds, device, deviceName, deviceUuid, useFirstDevice); err != nil {
				return nil, errors.Wrap(err, "unable to find cast dns entry")
			}
		}
		if !disableCache {
			cachedEntry := CachedDNSEntry{
				UUID: entry.GetUUID(),
				Name: entry.GetName(),
				Addr: entry.GetAddr(),
				Port: entry.GetPort(),
			}
			cachedEntryJson, _ := json.Marshal(cachedEntry)
			if err := cache.Save(getCacheKey(cachedEntry.UUID), cachedEntryJson); err != nil {
				outputError("Failed to save UUID cache entry\n")
			}
			if err := cache.Save(getCacheKey(cachedEntry.Name), cachedEntryJson); err != nil {
				outputError("Failed to save name cache entry\n")
			}
		}
		if debug {
			outputInfo("using device name=%s addr=%s port=%d uuid=%s", entry.GetName(), entry.GetAddr(), entry.GetPort(), entry.GetUUID())
		}
	} else {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, errors.Wrap(err, "port needs to be a number")
		}
		entry = CachedDNSEntry{
			Addr: addr,
			Port: p,
		}
	}
	app := application.NewApplication(applicationOptions...)
	if err := app.Start(entry.GetAddr(), entry.GetPort()); err != nil {
		// NOTE: currently we delete the dns cache every time we get
		// an error, this is to make sure that if the device gets a new
		// ipaddress we will invalidate the cache.
		if err := cache.Save(getCacheKey(entry.GetUUID()), []byte{}); err != nil {
			fmt.Printf("Failed to save UUID cache entry: %v\n", err)
		}
		if err := cache.Save(getCacheKey(entry.GetName()), []byte{}); err != nil {
			fmt.Printf("Failed to save name cache entry: %v\n", err)
		}
		return nil, err
	}
	return app, nil
}

