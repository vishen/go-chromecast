package cmd

import (
	"reflect"
	"testing"
)

func TestSplitAndTrim(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		sep      string
		expected []string
	}{
		{
			desc:     "Empty input",
			input:    "",
			sep:      ",",
			expected: []string{},
		},
		{
			desc:     "Single item",
			input:    "192.168.1.1",
			sep:      ",",
			expected: []string{"192.168.1.1"},
		},
		{
			desc:     "Multiple items with spaces",
			input:    "192.168.1.1, 10.0.0.1, 172.16.0.1",
			sep:      ",",
			expected: []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"},
		},
		{
			desc:     "Multiple items without spaces",
			input:    "192.168.1.1,10.0.0.1,172.16.0.1",
			sep:      ",",
			expected: []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"},
		},
		{
			desc:     "Items with leading/trailing spaces",
			input:    "  192.168.1.1  ,  10.0.0.1  ",
			sep:      ",",
			expected: []string{"192.168.1.1", "10.0.0.1"},
		},
		{
			desc:     "Different separator",
			input:    "item1|item2|item3",
			sep:      "|",
			expected: []string{"item1", "item2", "item3"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := splitAndTrim(tc.input, tc.sep)
			if len(result) == 0 && len(tc.expected) == 0 {
				// This is fine, just continue
				return
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("splitAndTrim(%q, %q) = %v; want %v", tc.input, tc.sep, result, tc.expected)
			}
		})
	}
}
