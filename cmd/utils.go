package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vishen/go-chromecast/discovery"
	"github.com/vishen/go-chromecast/discovery/zeroconf"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/application"
	"github.com/vishen/go-chromecast/storage"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

var (
	cache = storage.NewStorage()
)

// CastDNSEntry is used by DNS and caching discovery
type CastDNSEntry interface {
	GetName() string
	GetUUID() string
	GetAddr() string
	GetPort() int
}

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
	iface, _ := cmd.Flags().GetString("iface")
	first, _ := cmd.Flags().GetBool("first")

	var entry CastDNSEntry
	// If no address was specified, attempt to determine the address of any
	// local chromecast devices.
	if addr == "" {
		// If a device name or uuid was specified, check the cache for the ip+port
		found := false
		if !disableCache && (deviceName != "" || deviceUuid != "") {
			entry = findCachedDevice(deviceName, deviceUuid)
			found = entry.GetAddr() != ""
		}
		if !found {
			var err error
			if first {
				entry, err = selectFirstDevice(device, deviceName, deviceUuid)
			} else {
				entry, err = selectDevice(device, deviceName, deviceUuid)
			}
			if err != nil {
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
	app := application.NewApplication(iface, debug, disableCache)
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

func findCachedDevice(deviceName, deviceUuid string) CachedDNSEntry {
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

func deviceMatchers(deviceType, deviceName, deviceUuid string) []discovery.DeviceMatcher {
	var m []discovery.DeviceMatcher
	if deviceType != "" {
		m = append(m, discovery.WithType(deviceType))
	}
	if deviceUuid != "" {
		m = append(m, discovery.WithID(deviceUuid))
	}
	if deviceName != "" {
		m = append(m, discovery.WithName(deviceName))
	}
	return m
}

func selectFirstDevice(deviceType, deviceName, deviceUuid string) (*discovery.Device, error) {
	matchers := deviceMatchers(deviceType, deviceName, deviceUuid)
	discover := discovery.Service{
		Scanner: zeroconf.Scanner{Logger: log.New()},
	}
	return discover.First(context.Background(), matchers...)
}

func makeDeviceList(deviceType, deviceName, deviceUuid string) ([]*discovery.Device, error) {
	matchers := deviceMatchers(deviceType, deviceName, deviceUuid)
	discover := discovery.Service{
		Scanner: zeroconf.Scanner{Logger: log.New()},
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return discover.Sorted(ctx, matchers...)
}

func selectDevice(device, deviceName, deviceUuid string) (*discovery.Device, error) {
	devices, err := makeDeviceList(device, deviceName, deviceUuid)
	if err != nil {
		return nil, err
	}
	l := len(devices)
	if l == 0 {
		return nil, errors.New("no device found")
	}
	if l == 1 {
		return devices[0], nil
	}

	fmt.Printf("Found %d cast dns entries, select one:\n", l)
	for i, d := range devices {
		fmt.Printf("%d) device=%q device_name=%q address=\"%s\" status=%q uuid=%q\n", i+1, d.Type(), d.Name(), d.Addr(), d.Status(), d.ID())
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
			fmt.Printf("error parsing number: %v\n", err)
			continue
		} else if i < 1 || i > l {
			fmt.Printf("%d is an invalid choice\n", i)
			continue
		}
		return devices[i-1], nil
	}
}
