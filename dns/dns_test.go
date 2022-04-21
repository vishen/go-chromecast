package dns

import (
	"testing"
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
