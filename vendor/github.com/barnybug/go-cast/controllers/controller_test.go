package controllers

import "testing"

func TestInterfaces(t *testing.T) {
	// assert controllers implement interfaces
	var _ Controller = (*ConnectionController)(nil)
	var _ Controller = (*HeartbeatController)(nil)
	var _ Controller = (*ReceiverController)(nil)
	var _ Controller = (*MediaController)(nil)
}
