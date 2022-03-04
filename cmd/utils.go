package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/application"
	castdns "github.com/vishen/go-chromecast/dns"
	"github.com/vishen/go-chromecast/storage"
)

func init() {
	log.SetOutput(os.Stdout)
}

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

	foundEntries := []castdns.CastEntry{}
	for entry := range castEntryChan {
		if first || (deviceUuid != "" && entry.UUID == deviceUuid) || (deviceName != "" && entry.DeviceName == deviceName) || (device != "" && entry.Device == device) {
			return entry, nil
		}
		foundEntries = append(foundEntries, entry)
	}

	if len(foundEntries) == 0 {
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
	fmt.Printf(msg, args...)
}

const (
	RED = "\033[0;31m"
	NC  = "\033[0m" // No Color
)
