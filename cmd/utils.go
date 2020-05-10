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
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/application"
	castdns "github.com/vishen/go-chromecast/dns"
	"github.com/vishen/go-chromecast/storage"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

var (
	cache = storage.NewStorage()
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

func castApplication(cmd *cobra.Command, args []string) (*application.Application, error) {
	deviceName, _ := cmd.Flags().GetString("device-name")
	deviceUuid, _ := cmd.Flags().GetString("uuid")
	device, _ := cmd.Flags().GetString("device")
	debug, _ := cmd.Flags().GetBool("debug")
	disableCache, _ := cmd.Flags().GetBool("disable-cache")
	addr, _ := cmd.Flags().GetString("addr")
	port, _ := cmd.Flags().GetString("port")
	ifaceName, _ := cmd.Flags().GetString("iface")
	dnsTimeoutSeconds, _ := cmd.Flags().GetInt("dns-timeout")
	useFirstDevice, _ := cmd.Flags().GetBool("first")

	applicationOptions := []application.ApplicationOption{
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

	var entry castdns.CastDNSEntry
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
			cache.Save(getCacheKey(cachedEntry.UUID), cachedEntryJson)
			cache.Save(getCacheKey(cachedEntry.Name), cachedEntryJson)
		}
		if debug {
			fmt.Printf("using device name=%s addr=%s port=%d uuid=%s\n", entry.GetName(), entry.GetAddr(), entry.GetPort(), entry.GetUUID())
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
	if err := app.Start(entry); err != nil {
		// NOTE: currently we delete the dns cache every time we get
		// an error, this is to make sure that if the device gets a new
		// ipaddress we will invalidate the cache.
		cache.Save(getCacheKey(entry.GetUUID()), []byte{})
		cache.Save(getCacheKey(entry.GetName()), []byte{})
		return nil, err
	}
	return app, nil
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

	fmt.Printf("Found %d cast dns entries, select one:\n", len(foundEntries))
	for i, d := range foundEntries {
		fmt.Printf("%d) device=%q device_name=%q address=\"%s:%d\" uuid=%q\n", i+1, d.Device, d.DeviceName, d.AddrV4, d.Port, d.UUID)
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
