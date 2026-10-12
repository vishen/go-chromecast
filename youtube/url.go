package youtube

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	videoIDRegex    = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	playlistIDRegex = regexp.MustCompile(`^(PL|UU|LL|RD|OL|FL)[A-Za-z0-9_-]{10,}$`)
)

// ParseURL extracts the video and/or playlist id from a YouTube url. A
// bare video id (eg: "dQw4w9WgXcQ") or playlist id (eg: "PL...") is also
// accepted. At least one of the returned ids will be non-empty when there
// is no error.
func ParseURL(s string) (videoID, playlistID string, err error) {
	s = strings.TrimSpace(s)
	if videoIDRegex.MatchString(s) {
		return s, "", nil
	}
	if playlistIDRegex.MatchString(s) {
		return "", s, nil
	}

	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", "", fmt.Errorf("unable to parse youtube url %q: %w", s, err)
	}

	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	host = strings.TrimPrefix(host, "music.")

	path := strings.Trim(u.Path, "/")
	query := u.Query()
	playlistID = query.Get("list")

	switch host {
	case "youtu.be":
		videoID = strings.SplitN(path, "/", 2)[0]
	case "youtube.com", "youtube-nocookie.com":
		parts := strings.Split(path, "/")
		switch parts[0] {
		case "watch", "playlist":
			videoID = query.Get("v")
		case "shorts", "live", "embed", "v":
			if len(parts) > 1 {
				videoID = parts[1]
			}
		}
	default:
		return "", "", fmt.Errorf("%q is not a youtube url", s)
	}

	if videoID != "" && !videoIDRegex.MatchString(videoID) {
		return "", "", fmt.Errorf("invalid youtube video id %q", videoID)
	}
	if videoID == "" && playlistID == "" {
		return "", "", fmt.Errorf("unable to find a video or playlist id in %q", s)
	}
	return videoID, playlistID, nil
}
