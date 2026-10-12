package youtube

import "testing"

func TestParseURL(t *testing.T) {
	tests := []struct {
		in         string
		videoID    string
		playlistID string
		wantErr    bool
	}{
		{in: "dQw4w9WgXcQ", videoID: "dQw4w9WgXcQ"},
		{in: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", videoID: "dQw4w9WgXcQ"},
		{in: "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s", videoID: "dQw4w9WgXcQ"},
		{in: "http://m.youtube.com/watch?v=dQw4w9WgXcQ", videoID: "dQw4w9WgXcQ"},
		{in: "https://music.youtube.com/watch?v=dQw4w9WgXcQ", videoID: "dQw4w9WgXcQ"},
		{in: "youtube.com/watch?v=dQw4w9WgXcQ", videoID: "dQw4w9WgXcQ"},
		{in: "https://youtu.be/dQw4w9WgXcQ?si=abc", videoID: "dQw4w9WgXcQ"},
		{in: "https://www.youtube.com/shorts/dQw4w9WgXcQ", videoID: "dQw4w9WgXcQ"},
		{in: "https://www.youtube.com/live/dQw4w9WgXcQ?feature=share", videoID: "dQw4w9WgXcQ"},
		{in: "https://www.youtube.com/embed/dQw4w9WgXcQ", videoID: "dQw4w9WgXcQ"},
		{
			in:         "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf",
			videoID:    "dQw4w9WgXcQ",
			playlistID: "PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf",
		},
		{
			in:         "https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf",
			playlistID: "PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf",
		},
		{in: "PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", playlistID: "PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf"},
		{in: "https://www.youtube.com/", wantErr: true},
		{in: "https://www.youtube.com/watch?v=tooshort", wantErr: true},
		{in: "https://example.com/watch?v=dQw4w9WgXcQ", wantErr: true},
		{in: "not a url", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		videoID, playlistID, err := ParseURL(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseURL(%q): unexpected error: %v", tt.in, err)
			continue
		}
		if videoID != tt.videoID || playlistID != tt.playlistID {
			t.Errorf("ParseURL(%q) = (%q, %q), want (%q, %q)", tt.in, videoID, playlistID, tt.videoID, tt.playlistID)
		}
	}
}
