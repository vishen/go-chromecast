package http

import "github.com/vishen/go-chromecast/cast"

type connectResponse struct {
	DeviceUUID string `json:"device_uuid"`
}

type volumeResponse struct {
	Level float32 `json:"level"`
	Muted bool    `json:"muted"`
}

type statusResponse struct {
	AppID        string `json:"app_id"`
	DisplayName  string `json:"display_name"`
	IsIdleScreen bool   `json:"is_idle_screen"`
	StatusText   string `json:"status_text"`

	PlayerState   string  `json:"player_state"`
	CurrentTime   float32 `json:"current_time"`
	IdleReason    string  `json:"idle_reason"`
	CurrentItemID int     `json:"current_item_id"`
	LoadingItemID int     `json:"loading_item_id"`

	ContentID   string  `json:"content_id"`
	ContentType string  `json:"content_type"`
	StreamType  string  `json:"stream_type"`
	Duration    float32 `json:"duration"`

	Artist   string `json:"artist"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`

	VolumeLevel      float32 `json:"volume_level"`
	VolumeMuted      bool    `json:"volume_muted"`
	MediaVolumeLevel float32 `json:"media_volume_level"`
	MediaVolumeMuted bool    `json:"media_volume_muted"`

	SessionID      string `json:"session_id"`
	TransportID    string `json:"transport_id"`
	MediaSessionID int    `json:"media_session_id"`

	PlayerStateId int `json:"player_state_id"`
}

func fromApplicationStatus(app *cast.Application, media *cast.Media, volume *cast.Volume) statusResponse {
	status := statusResponse{}

	if app != nil {
		status.AppID = app.AppId
		status.DisplayName = app.DisplayName
		status.IsIdleScreen = app.IsIdleScreen
		status.StatusText = app.StatusText
		status.SessionID = app.SessionId
		status.TransportID = app.TransportId
	}

	if media != nil {
		status.PlayerState = media.PlayerState
		status.CurrentTime = media.CurrentTime
		status.IdleReason = media.IdleReason
		status.CurrentItemID = media.CurrentItemId
		status.LoadingItemID = media.LoadingItemId
		status.MediaSessionID = media.MediaSessionId

		status.MediaVolumeLevel = media.Volume.Level
		status.MediaVolumeMuted = media.Volume.Muted

		status.ContentID = media.Media.ContentId
		status.ContentType = media.Media.ContentType
		status.StreamType = media.Media.StreamType
		status.Duration = media.Media.Duration

		status.Artist = media.Media.Metadata.Artist
		status.Title = media.Media.Metadata.Title
		status.Subtitle = media.Media.Metadata.Subtitle

		status.PlayerStateId = media.CustomData.PlayerState

	}

	if volume != nil {
		status.VolumeLevel = volume.Level
		status.VolumeMuted = volume.Muted
	}

	return status
}

type device struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`

	Name string `json:"name"`
	Host string `json:"host"`

	UUID       string            `json:"uuid"`
	Device     string            `json:"device_type"`
	Status     string            `json:"status"`
	DeviceName string            `json:"device_name"`
	InfoFields map[string]string `json:"info_fields"`
}
