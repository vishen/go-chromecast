package simulator

import "github.com/vishen/go-chromecast/cast"

type State string

const (
	State_IDLE    State = "IDLE"
	State_PLAYING State = "PLAYING"
)

type Chromecast struct {
	state State

	castApp    *cast.Application
	castVolume *cast.Volume
	castMedia  *cast.Media

	sessionID string

	quit chan struct{}
}

func NewChromecast(state State) *Chromecast {
	sessionID := "aaaaaaaa-bbbb-1111-2222-333333333333" // TODO: Should this be randomized?

	castApp := cast.Application{
		AppId:        "CastSimulator",
		DisplayName:  "Testing",
		IsIdleScreen: true, // NOTE: Needs to start off as true.
		SessionId:    sessionID,
		TransportId:  sessionID,
	}

	castVolume := cast.Volume{
		Level: 0.4,
		Muted: false,
	}

	castMedia := cast.Media{
		MediaSessionId: 1,
		PlayerState:    string(State_IDLE),
		IdleReason:     "simulator in waiting",
	}

	c := &Chromecast{
		state:      state,
		castApp:    &castApp,
		castVolume: &castVolume,
		castMedia:  &castMedia,
		sessionID:  sessionID,
		quit:       make(chan struct{}),
	}
	c.Update()
	return c
}

func (c *Chromecast) Wait() {
	<-c.quit
}

func (c *Chromecast) Stop() {
	c.quit <- struct{}{}
}

func (c *Chromecast) Update() {
	switch c.state {
	case State_IDLE:
		c.castApp.IsIdleScreen = true

		c.castMedia.PlayerState = string(State_IDLE)
		c.castMedia.IdleReason = "simulator in waiting"
	case State_PLAYING:
		c.castApp.IsIdleScreen = false

		c.castMedia.PlayerState = string(State_PLAYING)
		c.castMedia.IdleReason = ""
		c.castMedia.Volume = *c.castVolume
		c.castMedia.CurrentTime = 123.45

		c.castMedia.Media = cast.MediaItem{
			ContentId:   "some-content-id",
			ContentType: "application/simulator",
			Duration:    234.56,
		}
	}

}

func (c *Chromecast) SetVolume(level float32, muted bool) {
	c.castVolume.Level = level
	c.castVolume.Muted = muted

}

func (c *Chromecast) Application() cast.Application {
	return *c.castApp
}

func (c *Chromecast) Volume() cast.Volume {
	return *c.castVolume
}

func (c *Chromecast) Media() cast.Media {
	return *c.castMedia
}
