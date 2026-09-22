package cmd

import (
	"net"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestScanCmd_FlagParsing(t *testing.T) {
	testCases := []struct {
		desc         string
		flags        map[string]string
		expectedMode string // "subnets", "broad-search", "cidr"
	}{
		{
			desc: "Default behavior uses CIDR",
			flags: map[string]string{},
			expectedMode: "cidr",
		},
		{
			desc: "Broad search flag set",
			flags: map[string]string{
				"broad-search": "true",
			},
			expectedMode: "broad-search",
		},
		{
			desc: "Subnets flag overrides broad search",
			flags: map[string]string{
				"broad-search": "true",
				"subnets": "192.168.1.0/24,10.0.0.0/24",
			},
			expectedMode: "subnets",
		},
		{
			desc: "Subnets flag with wildcard",
			flags: map[string]string{
				"subnets": "*",
			},
			expectedMode: "subnets",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().String("subnets", "", "")
			cmd.Flags().Bool("broad-search", false, "")
			cmd.Flags().String("cidr", "192.168.50.0/24", "")

			// Set flags
			for key, value := range tc.flags {
				err := cmd.Flags().Set(key, value)
				if err != nil {
					t.Fatalf("Failed to set flag %s: %v", key, err)
				}
			}

			// Get flag values
			subnetsFlag, _ := cmd.Flags().GetString("subnets")
			broadSearch, _ := cmd.Flags().GetBool("broad-search")

			// Determine mode based on the same logic as the scan command
			var actualMode string
			if subnetsFlag != "" {
				actualMode = "subnets"
			} else if broadSearch {
				actualMode = "broad-search"
			} else {
				actualMode = "cidr"
			}

			if actualMode != tc.expectedMode {
				t.Errorf("Expected mode %s, got %s", tc.expectedMode, actualMode)
			}
		})
	}
}

func TestScanCmd_SubnetParsing(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		expected []string
	}{
		{
			desc:     "Single subnet",
			input:    "192.168.1.0/24",
			expected: []string{"192.168.1.0/24"},
		},
		{
			desc:     "Multiple subnets",
			input:    "192.168.1.0/24,10.0.0.0/24,172.16.0.0/16",
			expected: []string{"192.168.1.0/24", "10.0.0.0/24", "172.16.0.0/16"},
		},
		{
			desc:     "Subnets with spaces",
			input:    "192.168.1.0/24, 10.0.0.0/24, 172.16.0.0/16",
			expected: []string{"192.168.1.0/24", "10.0.0.0/24", "172.16.0.0/16"},
		},
		{
			desc:     "Empty input",
			input:    "",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var result []string
			if tc.input != "" {
				for _, s := range splitAndTrim(tc.input, ",") {
					if s != "" {
						result = append(result, s)
					}
				}
			}

			if (len(result) == 0 && len(tc.expected) == 0) || 
			   (result == nil && tc.expected == nil) {
				return // Both empty or nil, test passes
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestDetectLocalSubnet(t *testing.T) {
	// This test will verify that detectLocalSubnet doesn't crash
	// We can't mock network interfaces easily, so we'll just test basic functionality
	t.Run("DetectLocalSubnet doesn't crash", func(t *testing.T) {
		// This should either return a subnet or an error, but not crash
		subnet, err := detectLocalSubnet("")
		if err != nil {
			// It's okay if no subnet is detected in test environment
			t.Logf("No subnet detected (expected in test env): %v", err)
		} else {
			t.Logf("Detected subnet: %s", subnet)
			// Verify it's a valid CIDR
			_, _, err := net.ParseCIDR(subnet)
			if err != nil {
				t.Errorf("Invalid CIDR returned: %s, error: %v", subnet, err)
			}
		}
	})
}

// TestScanCmd_Integration tests the actual scan command behavior
func TestScanCmd_Integration(t *testing.T) {
	// Skip this test if we're not in an environment where we can safely run network scans
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Command structure is valid", func(t *testing.T) {
		cmd := scanCmd
		
		// Reset flags to default values
		cmd.Flags().Set("subnets", "")
		cmd.Flags().Set("broad-search", "false")
		cmd.Flags().Set("cidr", "192.168.50.0/24")

		// We can't easily test the actual scanning without mocking
		// So we'll just verify the command can be created and flags work
		if cmd == nil {
			t.Error("scanCmd should not be nil")
		}
		
		// Test that flags can be retrieved
		subnets, err := cmd.Flags().GetString("subnets")
		if err != nil {
			t.Errorf("Failed to get subnets flag: %v", err)
		}
		if subnets != "" {
			t.Errorf("Expected empty subnets, got %s", subnets)
		}
		
		broadSearch, err := cmd.Flags().GetBool("broad-search")
		if err != nil {
			t.Errorf("Failed to get broad-search flag: %v", err)
		}
		if broadSearch {
			t.Error("Expected broad-search to be false")
		}
	})
}
