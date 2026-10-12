package application

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentTypeAndTranscode(t *testing.T) {
	dir := t.TempDir()
	file := func(name string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("not really media"), 0o600))
		return path
	}
	mp4, mkv, unknown := file("video.mp4"), file("video.mkv"), file("video.unknown")

	tests := []struct {
		name          string
		filename      string
		contentType   string
		transcode     bool
		wantType      string
		wantTranscode bool
	}{
		{"playable file isn't transcoded", mp4, "", true, "video/mp4", false},
		{"unknown file is transcoded", unknown, "", true, "video/mp4", true},
		{"unplayable file is transcoded", mkv, "", true, "video/mp4", true},
		// Without transcoding there is no need to know what the file is:
		// it is sent as it is and the chromecast works it out.
		{"playable file without transcoding", mp4, "", false, "", false},
		{"unknown file without transcoding", unknown, "", false, "", false},
		// A given content type is always used.
		{"content type and transcoding", unknown, "video/webm", true, "video/webm", true},
		{"content type without transcoding", unknown, "audio/flac", false, "audio/flac", false},
	}

	a := &Application{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotTranscode := a.contentTypeAndTranscode(tt.filename, tt.contentType, tt.transcode)
			require.Equal(t, tt.wantType, gotType)
			require.Equal(t, tt.wantTranscode, gotTranscode)
		})
	}
}
