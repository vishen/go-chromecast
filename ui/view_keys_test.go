package ui

import "testing"

func TestKeysHelpLines(t *testing.T) {
	total := 0
	for i, k := range keysHelp {
		if i > 0 {
			total += len(keyHelpSeparator)
		}
		total += k.width()
	}

	if lines := keysHelpLines(total); len(lines) != 1 {
		t.Errorf("got %d lines when everything fits, want 1", len(lines))
	}

	for _, width := range []int{total - 1, 80, 40, 10, 0} {
		lines := keysHelpLines(width)
		if len(lines) < 2 {
			t.Errorf("width %d: got %d lines, want at least 2", width, len(lines))
		}

		count := 0
		for _, line := range lines {
			lineWidth := 0
			for i, k := range line {
				if i > 0 {
					lineWidth += len(keyHelpSeparator)
				}
				lineWidth += k.width()
			}
			// A line can only be too wide if it has a single key
			// that doesn't fit by itself.
			if lineWidth > width && len(line) > 1 {
				t.Errorf("width %d: line of %d columns with %d keys", width, lineWidth, len(line))
			}
			count += len(line)
		}
		if count != len(keysHelp) {
			t.Errorf("width %d: got %d keys, want %d", width, count, len(keysHelp))
		}
	}
}
