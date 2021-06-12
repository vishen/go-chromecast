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

	// Tell Logrus to log to this view:
	log.SetOutput(v)
	log.SetLevel(log.DebugLevel)
	// TODO
	// log.SetFormatter(&log.TextFormatter{
	// 	ForceColors:     true,
	// 	FullTimestamp:   true,
	// 	TimestampFormat: "15:04:05",
	// })

	return nil
}
