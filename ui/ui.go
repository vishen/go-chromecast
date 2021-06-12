package ui

import (
	"sync"
	"time"

	"github.com/vishen/go-chromecast/application"

	"github.com/jroimartin/gocui"

	"github.com/vishen/go-chromecast/log"
)

// UserInterface is an alternaive way of running go-chromecast (based around a gocui GUI):
type UserInterface struct {
	app             *application.Application
	displayName     string
	gui             *gocui.Gui
	media           string
	muted           bool
	paused          bool
	positionCurrent float32
	positionTotal   float32
	seekFastforward int
	seekRewind      int
	volume          int
	volumeMutex     sync.Mutex
	wg              sync.WaitGroup
}

// NewUserInterface returns a new user-interface loaded with everything we need:
func NewUserInterface(app *application.Application) (*UserInterface, error) {

	// Use a GUI from gocui to handle the user-interface:
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return nil, err
	}

	// Make a new UserInterface with this GUI:
	newUserInterface := &UserInterface{
		app:             app,
		displayName:     "connecting",
		gui:             g,
		seekFastforward: 15,
		seekRewind:      -15,
		volume:          0,
	}

	// Tell the GUI about its "Manager" function (defines the gocui "views"):
	g.SetManagerFunc(newUserInterface.views)

	// Setup key-bindings:
	newUserInterface.setupKeyBindings()

	return newUserInterface, nil
}

// Run tells the UserInterface to start:
func (ui *UserInterface) Run() error {
	defer ui.gui.Close()

	// Run the main gocui loop in the background (because it blocks):
	ui.wg.Add(1)
	go func() {
		if err := ui.gui.MainLoop(); err != nil && err != gocui.ErrQuit {
			log.WithError(err).Error("Error from gocui")
		}
		ui.wg.Done()
	}()

	// Update the status from the application:
	go ui.updateStatus(time.Second)

	// Wait for the main gocui loop to end:
	ui.wg.Wait()
	return nil
}

// Stop tells the UserInterface to stop:
func (ui *UserInterface) Stop(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
