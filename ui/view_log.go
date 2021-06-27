package ui

import (
	"github.com/jroimartin/gocui"
	"github.com/vishen/go-chromecast/log"
)

const viewNameLog = "Log"

// viewLog renders the log view:
func (ui *UserInterface) viewLog(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	v, err := g.SetView(viewNameLog, 0, 8, maxX-1, maxY-5)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Title = viewNameLog
	v.Autoscroll = true

	// Tell the logger to use this view:
	consoleWriterFunc := func(opts *log.ConsoleWriterOptions) { opts.TimeFormat = "15:05:05" }
	log.SetLevel(log.DebugLevel)
	log.SetOutput(log.NewConsoleWriter(v, consoleWriterFunc))

	return nil
}
