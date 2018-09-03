package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/application"
	castdns "github.com/vishen/go-chromecast/dns"
)

func castApplication(cmd *cobra.Command, args []string) (*application.Application, error) {
	var entry castdns.CastDNSEntry
	dnsEntries := castdns.FindCastDNSEntries()

	device, _ := cmd.Flags().GetString("device")
	deviceName, _ := cmd.Flags().GetString("device_name")
	deviceUuid, _ := cmd.Flags().GetString("uuid")
	debug, _ := cmd.Flags().GetBool("debug")
	disableCache, _ := cmd.Flags().GetBool("disableCache")

	switch l := len(dnsEntries); l {
	case 0:
		return nil, errors.New("no cast dns entries found")
	default:
		found := false
		for _, d := range dnsEntries {
			if (deviceUuid != "" && d.UUID == deviceUuid) || (deviceName != "" && d.DeviceName == deviceName) || (device != "" && d.Device == device) {
				entry = d
				found = true
				break
			}
		}

		if !found && l == 1 {
			entry = dnsEntries[0]
			found = true
		}

		if found {
			break
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
			entry = dnsEntries[i-1]
			break
		}
	}
	// TODO(vishen): get these values from the configuration
	app := application.NewApplication(debug, disableCache)
	if err := app.Start(entry); err != nil {
		return nil, err
	}
	return app, nil
}
