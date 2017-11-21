package events

type AppStopped struct {
	AppID       string
	DisplayName string
	StatusText  string
}
