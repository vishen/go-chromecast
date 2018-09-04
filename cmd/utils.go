package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/application"
	castdns "github.com/vishen/go-chromecast/dns"
	"github.com/vishen/go-chromecast/storage"
)

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

	// If a device name or uuid was specified, check the cache for the ip+port
	var entry castdns.CastDNSEntry

	if !disableCache {
		entry = findCachedCastDNS(deviceName, deviceUuid)
	}
	if entry.GetAddr() == "" {
		var err error
		if entry, err = findCastDNS(device, deviceName, deviceUuid); err != nil {
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

	app := application.NewApplication(debug, disableCache)
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

func findCastDNS(device, deviceName, deviceUuid string) (castdns.CastDNSEntry, error) {
	dnsEntries := castdns.FindCastDNSEntries()
	switch l := len(dnsEntries); l {
	case 0:
		return castdns.CastEntry{}, errors.New("no cast dns entries found")
	default:
		for _, d := range dnsEntries {
			if (deviceUuid != "" && d.UUID == deviceUuid) || (deviceName != "" && d.DeviceName == deviceName) || (device != "" && d.Device == device) {
				return d, nil
			}
		}

		if l == 1 {
			return dnsEntries[0], nil
		}

		fmt.Printf("Found %d cast dns entries, select one:\n", l)
		for i, d := range dnsEntries {
			fmt.Printf("%d) device=%q device_name=%q address=\"%s:%d\" status=%q uuid=%q\n", i+1, d.Device, d.DeviceName, d.AddrV4, d.Port, d.Status, d.UUID)
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
			} else if i < 1 || i > l {
				continue
			}
			return dnsEntries[i-1], nil
		}
	}
	return castdns.CastEntry{}, errors.New("no cast dns entries found")
}
