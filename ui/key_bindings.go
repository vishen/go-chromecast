package ui

import (
	"github.com/vishen/go-chromecast/application"

	"github.com/jroimartin/gocui"
	"github.com/vishen/go-chromecast/log"
)

// setupKeyBindings binds keys to actions:
func (ui *UserInterface) setupKeyBindings() {
	ui.gui.SetKeybinding("", 'q', gocui.ModNone, ui.Stop)
	ui.gui.SetKeybinding("", 's', gocui.ModNone, ui.stopMedia)
	ui.gui.SetKeybinding("", gocui.KeySpace, gocui.ModNone, ui.playPause)
	ui.gui.SetKeybinding("", gocui.KeyArrowLeft, gocui.ModNone, ui.seekBackwards)
	ui.gui.SetKeybinding("", gocui.KeyArrowRight, gocui.ModNone, ui.seekForwards)
	ui.gui.SetKeybinding("", '=', gocui.ModNone, ui.volumeUp)
	ui.gui.SetKeybinding("", '+', gocui.ModNone, ui.volumeUp)
	ui.gui.SetKeybinding("", '-', gocui.ModNone, ui.volumeDown)
	ui.gui.SetKeybinding("", 'm', gocui.ModNone, ui.volumeMute)
	ui.gui.SetKeybinding("", gocui.KeyPgup, gocui.ModNone, ui.previousMedia)
	ui.gui.SetKeybinding("", gocui.KeyPgdn, gocui.ModNone, ui.nextMedia)
}

// playPause tells the app to play / pause:
func (ui *UserInterface) playPause(g *gocui.Gui, v *gocui.View) error {
	if ui.paused {
		log.Info("Play")
		ui.app.Unpause()
		ui.paused = false
	} else {
		log.Info("Pause")
		ui.app.Pause()
		ui.paused = true
	}

	return nil
}

// seekBackwards tells the app to rewind:
func (ui *UserInterface) seekBackwards(g *gocui.Gui, v *gocui.View) error {
	err := ui.app.Seek(ui.seekRewind)
	if err != nil {
		switch err {
		case application.ErrMediaNotYetInitialised:
			log.Warn("Rewind (nothing playing)")
			return nil
		default:
			log.WithError(err).Error("Rewind")
			return nil
		}
	}

	log.WithField("seconds", ui.seekRewind).Info("Rewind")
	return nil
}

// seekForwards tells the app to fastforward:
func (ui *UserInterface) seekForwards(g *gocui.Gui, v *gocui.View) error {
	err := ui.app.Seek(ui.seekFastforward)
	if err != nil {
		switch err {
		case application.ErrMediaNotYetInitialised:
			log.Warn("Fastforward (nothing playing)")
			return nil
		default:
			log.WithError(err).Error("Fastforward")
			return nil
		}
	}

	log.WithField("seconds", ui.seekFastforward).Info("Fastforward")
	return nil
}

// volumeUp increases the volume:
func (ui *UserInterface) volumeUp(g *gocui.Gui, v *gocui.View) error {
	ui.volumeMutex.Lock()
	defer ui.volumeMutex.Unlock()

	// Attempt to increment our version of the volume:
	if ui.volume+5 > 100 {
		log.Warn("Volume already at maximum")
		return nil
	}
	ui.volume += 5

	floatVolume := float32(ui.volume) / 100

	err := ui.app.SetVolume(floatVolume)
	if err != nil {
		switch err {
		case application.ErrVolumeOutOfRange:
			log.WithError(err).WithField("volume", floatVolume).Warn("Volume up")
			return nil
		default:
			log.WithError(err).WithField("volume", floatVolume).Error("Volume up")
			return nil
		}
	}

	log.WithField("volume", floatVolume).Info("Volume up")
	return nil
}

// volumeDown decreases the volume:
func (ui *UserInterface) volumeDown(g *gocui.Gui, v *gocui.View) error {
	ui.volumeMutex.Lock()
	defer ui.volumeMutex.Unlock()

	// Attempt to decrement our version of the volume:
	if ui.volume-5 < 0 {
		log.Warn("Volume already at minimum")
		return nil
	}
	ui.volume -= 5

	floatVolume := float32(ui.volume) / 100

	err := ui.app.SetVolume(floatVolume)
	if err != nil {
		switch err {
		case application.ErrVolumeOutOfRange:
			log.WithError(err).WithField("volume", floatVolume).Warn("Volume down")
			return nil
		default:
			log.WithError(err).WithField("volume", floatVolume).Error("Volume down")
			return nil
		}
	}

	log.WithField("volume", floatVolume).Info("Volume down")
	return nil
}

// volumeMute mutes the volume:
func (ui *UserInterface) volumeMute(g *gocui.Gui, v *gocui.View) error {
	if ui.muted {
		ui.muted = false
	} else {
		ui.muted = true
	}

	err := ui.app.SetMuted(ui.muted)
	if err != nil {
		log.WithError(err).WithField("muted", ui.muted).Error("Volume mute")
		return nil
	}

	if ui.muted {
		log.Info("Volume muted")
	} else {
		log.Info("Volume unmuted")
	}
	return nil
}

// stopMedia halts playback:
func (ui *UserInterface) stopMedia(g *gocui.Gui, v *gocui.View) error {
	err := ui.app.StopMedia()
	if err != nil {
		switch err {
		case application.ErrNoMediaStop:
			log.Warn("Stop (nothing playing)")
			return nil
		default:
			log.WithError(err).Error("Stop")
			return nil
		}
	}

	log.Info("Stop")
	return nil
}

// nextMedia starts playing the next item in the playlist:
func (ui *UserInterface) nextMedia(g *gocui.Gui, v *gocui.View) error {
	err := ui.app.Next()
	if err != nil {
		switch err {
		case application.ErrNoMediaNext:
			log.WithError(err).Warn("Next")
			return nil
		default:
			log.WithError(err).Error("Next")
			return nil
		}
	}

	log.Info("Next")
	return nil
}

// previousMedia starts playing the previous item in the playlist:
func (ui *UserInterface) previousMedia(g *gocui.Gui, v *gocui.View) error {
	err := ui.app.Previous()
	if err != nil {
		switch err {
		case application.ErrNoMediaPrevious:
			log.WithError(err).Warn("Previous")
			return nil
		default:
			log.WithError(err).Error("Previous")
			return nil
		}
	}

	log.Info("Previous")
	return nil
}
