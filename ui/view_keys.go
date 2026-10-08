package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jroimartin/gocui"
)

const viewNameKeys = "Keys"

// keyHelp is the help of a key-shortcut: an action and the keys bound to it.
type keyHelp struct {
	action string
	keys   []string
}

// keysHelp are the key-shortcuts, in the order they are shown:
var keysHelp = []keyHelp{
	{"Quit", []string{"q"}},
	{"Play/Pause", []string{"SPACE"}},
	{"Volume", []string{"-", "+"}},
	{"Mute", []string{"m"}},
	{"Seek", []string{"←", "→"}},
	{"Previous/Next", []string{"PgUp", "PgDn"}},
	{"Stop", []string{"s"}},
	{"Replay", []string{"r"}},
	{"Skip Ad", []string{"a"}},
}

const (
	keyHelpSeparator  = ", "
	keyHelpKeysJoiner = " / "
)

// width is the number of columns the help takes on the screen:
func (k keyHelp) width() int {
	return utf8.RuneCountInString(k.action + ": " + strings.Join(k.keys, keyHelpKeysJoiner))
}

// String returns the help with the keys highlighted:
func (k keyHelp) String() string {
	keys := make([]string, len(k.keys))
	for i, key := range k.keys {
		keys[i] = boldTextColour + key + normalTextColour
	}
	return normalTextColour + k.action + ": " + strings.Join(keys, keyHelpKeysJoiner)
}

// keysHelpLines splits the key-shortcuts into as many lines as it takes for
// them to fit in the given width:
func keysHelpLines(width int) [][]keyHelp {
	lines := [][]keyHelp{}
	lineWidth := 0
	for _, k := range keysHelp {
		needed := k.width()
		if lineWidth > 0 {
			needed += utf8.RuneCountInString(keyHelpSeparator)
		}
		if len(lines) == 0 || (lineWidth > 0 && lineWidth+needed > width) {
			lines = append(lines, nil)
			lineWidth = 0
			needed = k.width()
		}
		lines[len(lines)-1] = append(lines[len(lines)-1], k)
		lineWidth += needed
	}
	return lines
}

// viewKeys renders the helper message for key-shortcuts:
func (ui *UserInterface) viewKeys(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	// The view grows upwards when the keys don't fit in one line:
	lines := keysHelpLines(maxX - 2)

	v, err := g.SetView(viewNameKeys, 0, maxY-2-len(lines), maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Title = viewNameKeys
	v.Clear()

	for i, line := range lines {
		if i > 0 {
			fmt.Fprintln(v)
		}
		for j, k := range line {
			if j > 0 {
				fmt.Fprint(v, keyHelpSeparator)
			}
			fmt.Fprint(v, k)
		}
	}
	fmt.Fprint(v, resetTextColour)
	return nil
}
