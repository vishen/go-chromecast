package ui

import (
	"github.com/jroimartin/gocui"
)

const (
	boldTextColour    = "\033[34;1m"
	normalTextColour  = "\033[34;2m"
	resetTextColour   = "\033[0m"
	volumeColour      = "\033[31;2m"
	volumeMutedColour = "\033[31;1m"
	progressColour    = "\033[33;2m"
)

// views sets up all of the views:
func (ui *UserInterface) views(g *gocui.Gui) error {
	ui.viewStatus(g)
	ui.viewVolume(g)
	ui.viewProgress(g)
	ui.viewLog(g)
	ui.viewKeys(g)
	return nil
}
