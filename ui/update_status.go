package ui

import (
	"fmt"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/vishen/go-chromecast/log"
)

// updateStatus polls the chromecast application for status info, and updates the UI:
func (ui *UserInterface) updateStatus(sleepTime time.Duration) {

	for {
		// Sleep before updating:
		time.Sleep(sleepTime)

		// Get the "status" view:
		viewStatus, err := ui.gui.View(viewNameStatus)
		if err != nil {
			log.WithError(err).Errorf("Unable to get gocui view (%s)", viewNameStatus)
			continue
		}

		// Get the "progress" view:
		viewProgress, err := ui.gui.View(viewNameProgress)
		if err != nil {
			log.WithError(err).Errorf("Unable to get gocui view (%s)", viewNameProgress)
			continue
		}

		// Get the "volume" view:
		viewVolume, err := ui.gui.View(viewNameVolume)
		if err != nil {
			log.WithError(err).Errorf("Unable to get gocui view (%s)", viewNameVolume)
			continue
		}

		// Update the "status" view:
		ui.gui.Update(func(*gocui.Gui) error { return nil })
		if err := ui.app.Update(); err != nil {
			log.WithError(err).Debug("Unable to update the app")
		}
		viewStatus.Clear()
		castApplication, castMedia, castVolume := ui.app.Status()

		// Update the displayName:
		if castApplication == nil {
			ui.displayName = "Idle"
		} else {
			ui.displayName = castApplication.DisplayName
		}

		// Update the media info:
		if castMedia != nil {
			var media string
			if castMedia.Media.Metadata.Artist != "" {
				media = fmt.Sprintf("%s - ", castMedia.Media.Metadata.Artist)
			}
			if castMedia.Media.Metadata.Title != "" {
				media += fmt.Sprintf("%s ", castMedia.Media.Metadata.Title)
			}
			if castMedia.Media.ContentId != "" {
				media += fmt.Sprintf("[%s] ", castMedia.Media.ContentId)
			}
			ui.media = media
		} else {
			ui.media = "unknown"
		}
		fmt.Fprintf(viewStatus, "%sMedia:  %s%s%s\n", normalTextColour, boldTextColour, ui.media, resetTextColour)

		if castApplication != nil {
			fmt.Fprintf(viewStatus, "%sDetail: %s%s%s\n", normalTextColour, boldTextColour, castApplication.StatusText, resetTextColour)
		}

		// Update the player status:
		if castMedia != nil {
			ui.paused = castMedia.PlayerState == "PAUSED"
		}

		// Update the playback position:
		if castMedia != nil {
			ui.positionCurrent = castMedia.CurrentTime
			ui.positionTotal = castMedia.Media.Duration
		} else {
			ui.positionCurrent = 0
			ui.positionTotal = 0
		}

		// Update the "progress" view:
		if castMedia != nil {
			viewProgress.Clear()
			viewWidth, _ := viewProgress.Size()
			progress := (castMedia.CurrentTime / castMedia.Media.Duration) * float32(viewWidth)

			// Draw a bar of "#" to represent progress:
			for i := 0; i < int(progress); i++ {
				fmt.Fprintf(viewProgress, "%s#", progressColour)
			}
		}

		// Update the "volume" view:
		if castVolume != nil {
			ui.volume = int(castVolume.Level * 100)
			ui.muted = castVolume.Muted

			viewVolume.Clear()
			if ui.muted {
				fmt.Fprintf(viewVolume, "%s(muted)", volumeMutedColour)
			} else {
				for i := 0; i < ui.volume/5; i++ {
					fmt.Fprintf(viewVolume, "%s#", volumeColour)
				}
			}
		}
	}
}
