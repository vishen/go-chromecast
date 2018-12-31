package ui

import (
	"fmt"

	"github.com/jroimartin/gocui"
)

const viewNameProgress = "Progress"

// viewProgress renders the progress view:
func (ui *UserInterface) viewProgress(g *gocui.Gui) error {
	maxX, _ := g.Size()

	v, err := g.SetView(viewNameProgress, 22, 4, maxX-1, 6)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Title = fmt.Sprintf("%s (%0.2fs / %0.2fs)", viewNameProgress, ui.positionCurrent, ui.positionTotal)

	return nil
}
