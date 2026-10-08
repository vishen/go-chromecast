package ui

import (
	"fmt"

	"github.com/jroimartin/gocui"
)

const viewNameKeys = "Keys"

// viewKeys renders the helper message for key-shortcuts:
func (ui *UserInterface) viewKeys(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	// The keys go at the bottom, or just below the other views if there
	// is no log view to fill the gap:
	y := maxY - 3
	if ui.hideLog {
		y = 7
	}

	if v, err := g.SetView(viewNameKeys, 0, y, maxX-1, y+2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		v.Title = viewNameKeys

		fmt.Fprintf(v, "%sQuit: %sq", normalTextColour, boldTextColour)
		fmt.Fprintf(v, "%s, Play/Pause: %sSPACE", normalTextColour, boldTextColour)
		fmt.Fprintf(v, "%s, Volume: %s-%s / %s+", normalTextColour, boldTextColour, normalTextColour, boldTextColour)
		fmt.Fprintf(v, "%s, Mute: %sm", normalTextColour, boldTextColour)
		fmt.Fprintf(v, "%s, Seek: %s←%s / %s→", normalTextColour, boldTextColour, normalTextColour, boldTextColour)
		fmt.Fprintf(v, "%s, Previous/Next: %sPgUp%s / %sPgDn", normalTextColour, boldTextColour, normalTextColour, boldTextColour)
		fmt.Fprintf(v, "%s, Stop: %ss", normalTextColour, boldTextColour)
		fmt.Fprintf(v, "%s, Skip Ad: %sa", normalTextColour, boldTextColour)
		fmt.Fprint(v, resetTextColour)
	}
	return nil
}
