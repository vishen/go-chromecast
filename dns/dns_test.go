package dns

import (
	"errors"
	"reflect"
	"testing"

	"github.com/grandcat/zeroconf"
)

func TestDecode(t *testing.T) {
	testCases := []struct {
		val      string
		expected string
	}{
		{
			val:      "\\208\\161\\209\\130\\208\\176\\209\\129: \\208\\154\\208\\190\\208\\187\\208\\190\\208\\189\\208\\186\\208\\176",
			expected: "Стас: Колонка",
		},
		{
			val:      "\\208\\161\\209\\130\\208\\176\\209\\129: ABCDEF  HIJ\\208\\154\\208\\190\\208\\187\\208\\190\\208\\189\\208\\186\\208\\176",
			expected: "Стас: ABCDEF  HIJКолонка",
		},
		{
			val:      "contains no escape characters",
			expected: "contains no escape characters",
		},
		{
			val:      "contains some \\escape characters",
			expected: "contains some \\escape characters",
		},
		{
			val:      "contains some \\escape numbers: \\20",
			expected: "contains some \\escape numbers: \\20",
		},
	}

	for _, tt := range testCases {
		decoded := decode(tt.val)
		if decoded != tt.expected {
			t.Errorf("decoded to %s, but expected %s", decoded, tt.expected)
		}
	}
}

func TestNewDiscoveryResolver(t *testing.T) {
	initializationError := errors.New("cannot open multicast socket")
	for _, tc := range []struct {
		name               string
		failDual, failIPv4 bool
		want               []zeroconf.IPType
	}{
		{"dual stack", false, false, []zeroconf.IPType{zeroconf.IPv4AndIPv6}},
		{"IPv6 unavailable", true, false, []zeroconf.IPType{zeroconf.IPv4AndIPv6, zeroconf.IPv4}},
		{"both unavailable", true, true, []zeroconf.IPType{zeroconf.IPv4AndIPv6, zeroconf.IPv4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []zeroconf.IPType
			wantResolver := new(zeroconf.Resolver)
			resolver, err := newDiscoveryResolver(func(traffic zeroconf.IPType) (*zeroconf.Resolver, error) {
				calls = append(calls, traffic)
				if traffic == zeroconf.IPv4AndIPv6 && tc.failDual || traffic == zeroconf.IPv4 && tc.failIPv4 {
					return nil, initializationError
				}
				return wantResolver, nil
			})
			if !reflect.DeepEqual(calls, tc.want) {
				t.Fatalf("transports = %v, want %v", calls, tc.want)
			}
			if tc.failIPv4 {
				if !errors.Is(err, initializationError) || resolver != nil {
					t.Fatalf("got (%v, %v), want initialization failure", resolver, err)
				}
			} else if err != nil || resolver != wantResolver {
				t.Fatalf("got (%v, %v), want resolver", resolver, err)
			}
		})
	}
}
