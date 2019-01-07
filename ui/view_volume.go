package ui

import (
	"fmt"

	"github.com/jroimartin/gocui"
)

const viewNameVolume = "Volume"

// viewVolume renders the volume view:
func (ui *UserInterface) viewVolume(g *gocui.Gui) error {
	v, err := g.SetView(viewNameVolume, 0, 4, 21, 6)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Title = fmt.Sprintf("%s (%d%%)", viewNameVolume, ui.volume)

	return nil
}
