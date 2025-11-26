package cmd

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"
)

var localIPCmd = &cobra.Command{
	Use:   "localip",
	Short: "Print the local IP address used by go-chromecast",
	Run: func(cmd *cobra.Command, args []string) {
		ifaceName, _ := cmd.Flags().GetString("iface")
		ip, err := detectLocalIP(ifaceName)
		if err != nil {
			exit("unable to determine local IP: %v", err)
		}
		fmt.Println(ip)
	},
}

// detectLocalIP attempts to detect the local IP address based on the network interface
func detectLocalIP(ifaceName string) (string, error) {
	var iface *net.Interface
	var err error

	if ifaceName != "" {
		iface, err = net.InterfaceByName(ifaceName)
		if err != nil {
			return "", err
		}
	}

	if iface != nil {
		// Use the specified interface
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String(), nil
				}
			}
		}
	} else {
		// Try to find the default route interface
		interfaces, err := net.Interfaces()
		if err != nil {
			return "", err
		}

		for _, iface := range interfaces {
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
						return ipnet.IP.String(), nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not detect local IP address")
}

func init() {
	localIPCmd.Flags().String("iface", "", "network interface to use for detecting local IP (optional)")
	rootCmd.AddCommand(localIPCmd)
}
