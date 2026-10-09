package ui

import (
	"github.com/jroimartin/gocui"
)

const resetTextColour = "\033[0m"

// The colours of the UI, which can be changed with SetTheme:
var (
	boldTextColour    string
	normalTextColour  string
	volumeColour      string
	volumeMutedColour string
	progressColour    string
)

func init() {
	if err := SetTheme(DefaultTheme()); err != nil {
		panic(err)
	}
}

// views sets up all of the views:
func (ui *UserInterface) views(g *gocui.Gui) error {
	ui.viewStatus(g)
	ui.viewVolume(g)
	ui.viewProgress(g)
	ui.viewLog(g)
	ui.viewKeys(g)
	return nil
}
