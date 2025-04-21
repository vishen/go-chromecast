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
	Info   *cast.DeviceInfo  `json:"info,omitempty"`
	App    *cast.Application `json:"app,omitempty"`
	Media  *cast.Media       `json:"media,omitempty"`
	Volume *cast.Volume      `json:"volume,omitempty"`
}

func fromApplicationStatus(info *cast.DeviceInfo, app *cast.Application, media *cast.Media, volume *cast.Volume) statusResponse {
	status := statusResponse{}

	if info != nil {
		status.Info = info
	}

	if app != nil {
		status.App = app
	}

	if media != nil {
		status.Media = media
	}

	if volume != nil {
		status.Volume = volume
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
