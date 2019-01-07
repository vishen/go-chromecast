package ui

import (
	"fmt"

	"github.com/jroimartin/gocui"
)

const viewNameStatus = "Status"

// viewStatus renders the status view (what the Chromecast is doing):
func (ui *UserInterface) viewStatus(g *gocui.Gui) error {
	maxX, _ := g.Size()

	v, err := g.SetView(viewNameStatus, 0, 0, maxX-1, 3)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Title = fmt.Sprintf("%s (%s)", viewNameStatus, ui.displayName)

	return nil
}
