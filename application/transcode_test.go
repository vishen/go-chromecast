package application

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTranscodeArgs(t *testing.T) {
	t.Run("without extra arguments", func(t *testing.T) {
		args := transcodeArgs("video.mkv", nil)
		require.Equal(t, []string{"-re", "-i", "video.mkv"}, args[:3])
		require.Equal(t, "pipe:1", args[len(args)-1])
	})

	t.Run("extra arguments go after the default ones and before the output", func(t *testing.T) {
		defaultArgs := transcodeArgs("video.mkv", nil)
		args := transcodeArgs("video.mkv", []string{"-map", "0:v", "-map", "0:a:1"})

		require.Equal(t, defaultArgs[:len(defaultArgs)-1], args[:len(defaultArgs)-1])
		require.Equal(t, []string{"-map", "0:v", "-map", "0:a:1", "pipe:1"}, args[len(defaultArgs)-1:])
	})
}
