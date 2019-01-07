package application

import "github.com/pkg/errors"

var (
	ErrApplicationNotSet      = errors.New("application isn't set")
	ErrMediaNotYetInitialised = errors.New("media not yet initialised")
	ErrNoMediaNext            = errors.New("media not yet initialised, there is nothing to go to next")
	ErrNoMediaPause           = errors.New("media not yet initialised, there is nothing to pause")
	ErrNoMediaPrevious        = errors.New("media not yet initialised, there is nothing previous")
	ErrNoMediaSkip            = errors.New("media not yet initialised, there is nothing to skip")
	ErrNoMediaStop            = errors.New("media not yet initialised, there is nothing to stop")
	ErrNoMediaUnpause         = errors.New("media not yet initialised, there is nothing to unpause")
	ErrVolumeOutOfRange       = errors.New("specified volume is out of range (0 - 1)")
)
