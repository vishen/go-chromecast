// Package youtube implements enough of the (unofficial) YouTube Lounge
// API to control the queue of the YouTube app running on a cast device.
//
// The YouTube receiver app doesn't play videos via the usual cast media
// 'LOAD' command. Instead the cast device gives us a "screen id", which
// can be exchanged for a "lounge token" that is used to talk to the YouTube
// servers, which in turn control the device. This is the same mechanism
// the YouTube mobile apps use, and is based on the behaviour of
// https://github.com/ur1katz/casttube.
package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// AppID is the cast application id of the YouTube receiver app.
	AppID = "233637DE"

	// Namespace is the cast namespace used to talk to the YouTube
	// receiver app.
	Namespace = "urn:x-cast:com.google.youtube.mdx"

	// MessageTypeGetScreenID is the message type to send on Namespace to
	// request the screen id, and MessageTypeScreenID is the type of the
	// response.
	MessageTypeGetScreenID = "getMdxSessionStatus"
	MessageTypeScreenID    = "mdxSessionStatus"

	defaultBaseURL = "https://www.youtube.com/"

	loungeTokenPath = "api/lounge/pairing/get_lounge_token_batch"
	bindPath        = "api/lounge/bc/bind"

	loungeTokenHeader = "X-YouTube-LoungeId-Token"

	actionSetPlaylist = "setPlaylist"
	actionAdd         = "addVideo"
	actionInsert      = "insertVideo"
	actionRemove      = "removeVideo"
	actionClear       = "clearPlaylist"
)

var (
	sidRegex        = regexp.MustCompile(`"c","(.*?)","`)
	gsessionIDRegex = regexp.MustCompile(`"S","(.*?)"]`)
)

// Session is a YouTube Lounge session bound to a single screen (cast device).
// It is not safe for concurrent use.
type Session struct {
	screenID string

	baseURL    string
	httpClient *http.Client

	loungeToken string
	sid         string
	gsessionID  string

	// Optional function used to log the requests made.
	logf func(format string, args ...interface{})

	// Request id, incremented on every request to the bind endpoint.
	rid int
	// Number of commands sent in the current session.
	reqCount int
}

type SessionOption func(*Session)

// WithHTTPClient sets the http client to use for the requests.
func WithHTTPClient(c *http.Client) SessionOption {
	return func(s *Session) { s.httpClient = c }
}

// WithLogger sets a function used to log the requests made, useful for
// debugging.
func WithLogger(logf func(format string, args ...interface{})) SessionOption {
	return func(s *Session) { s.logf = logf }
}

// WithBaseURL overrides the YouTube url, used for testing.
func WithBaseURL(baseURL string) SessionOption {
	return func(s *Session) {
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		s.baseURL = baseURL
	}
}

// NewSession returns a session for the screen id. No requests are made until
// the first command is sent.
func NewSession(screenID string, opts ...SessionOption) *Session {
	s := &Session{
		screenID:   screenID,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Play replaces the current queue and starts playing the video. If
// playlistID is set the playlist is loaded and videoID (which can be empty)
// is the video in that playlist to start from.
func (s *Session) Play(ctx context.Context, videoID, playlistID string) error {
	// The queue of the screen outlives the videos (and even the app), so
	// without clearing it first the video is just added to whatever was
	// there before.
	if err := s.Clear(ctx); err != nil {
		return err
	}
	return s.command(ctx, url.Values{
		"__sc":          {actionSetPlaylist},
		"_videoId":      {videoID},
		"_listId":       {playlistID},
		"_currentTime":  {"0"},
		"_currentIndex": {"-1"},
		"_audioOnly":    {"false"},
		"count":         {"1"},
	})
}

// Add adds the video to the end of the queue.
func (s *Session) Add(ctx context.Context, videoID string) error {
	return s.queueAction(ctx, actionAdd, videoID)
}

// PlayNext adds the video to the queue just after the current video.
func (s *Session) PlayNext(ctx context.Context, videoID string) error {
	return s.queueAction(ctx, actionInsert, videoID)
}

// Remove removes the video from the queue.
func (s *Session) Remove(ctx context.Context, videoID string) error {
	return s.queueAction(ctx, actionRemove, videoID)
}

// Clear removes all the videos from the queue.
func (s *Session) Clear(ctx context.Context) error {
	return s.command(ctx, url.Values{
		"__sc":  {actionClear},
		"count": {"1"},
	})
}

func (s *Session) queueAction(ctx context.Context, action, videoID string) error {
	return s.command(ctx, url.Values{
		"__sc":     {action},
		"_videoId": {videoID},
		"count":    {"1"},
	})
}

// command sends a command to the screen, starting (or restarting) the
// session if required.
func (s *Session) command(ctx context.Context, params url.Values) error {
	if s.sid == "" {
		if err := s.start(ctx); err != nil {
			return err
		}
	}

	status, err := s.sendCommand(ctx, params)
	if err != nil {
		return err
	}
	// A 404 or 400 means the session has expired, so start a new one and
	// try again.
	if status == http.StatusNotFound || status == http.StatusBadRequest {
		if err := s.start(ctx); err != nil {
			return err
		}
		if status, err = s.sendCommand(ctx, params); err != nil {
			return err
		}
	}
	if status != http.StatusOK {
		return fmt.Errorf("youtube: unexpected status %d sending command", status)
	}
	return nil
}

func (s *Session) sendCommand(ctx context.Context, params url.Values) (int, error) {
	// Any key starting with '_' is specific to this command and needs to
	// be prefixed with the number of the command in the session.
	prefix := "req" + strconv.Itoa(s.reqCount)
	data := url.Values{}
	for k, v := range params {
		if strings.HasPrefix(k, "_") {
			k = prefix + k
		}
		data[k] = v
	}

	query := url.Values{
		"SID":        {s.sid},
		"gsessionid": {s.gsessionID},
		"RID":        {strconv.Itoa(s.rid)},
		"VER":        {"8"},
		"CVER":       {"1"},
	}
	status, _, err := s.post(ctx, bindPath, query, data, true)
	if err != nil {
		return 0, err
	}
	if status == http.StatusOK {
		s.reqCount++
		s.rid++
	}
	return status, nil
}

// start gets a lounge token for the screen and binds a new session.
func (s *Session) start(ctx context.Context) error {
	if err := s.getLoungeToken(ctx); err != nil {
		return err
	}
	return s.bind(ctx)
}

func (s *Session) getLoungeToken(ctx context.Context) error {
	status, body, err := s.post(ctx, loungeTokenPath, nil, url.Values{"screen_ids": {s.screenID}}, false)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("youtube: unexpected status %d getting lounge token", status)
	}

	resp := struct {
		Screens []struct {
			LoungeToken string `json:"loungeToken"`
		} `json:"screens"`
	}{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("youtube: unable to parse lounge token response: %w", err)
	}
	if len(resp.Screens) == 0 || resp.Screens[0].LoungeToken == "" {
		return fmt.Errorf("youtube: no lounge token returned for screen %q", s.screenID)
	}
	s.loungeToken = resp.Screens[0].LoungeToken
	return nil
}

func (s *Session) bind(ctx context.Context) error {
	s.rid = 0
	s.reqCount = 0
	s.sid = ""
	s.gsessionID = ""

	query := url.Values{
		"RID":  {strconv.Itoa(s.rid)},
		"VER":  {"8"},
		"CVER": {"1"},
	}
	data := url.Values{
		"device":       {"REMOTE_CONTROL"},
		"id":           {"aaaaaaaaaaaaaaaaaaaaaaaaaa"},
		"name":         {"go-chromecast"},
		"mdx-version":  {"3"},
		"pairing_type": {"cast"},
		"app":          {"android-phone-13.14.55"},
	}
	status, body, err := s.post(ctx, bindPath, query, data, true)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("youtube: unexpected status %d binding session", status)
	}

	sid := sidRegex.FindSubmatch(body)
	gsessionID := gsessionIDRegex.FindSubmatch(body)
	if sid == nil || gsessionID == nil {
		return fmt.Errorf("youtube: unable to find session ids in bind response")
	}
	s.sid = string(sid[1])
	s.gsessionID = string(gsessionID[1])
	s.rid++
	return nil
}

func (s *Session) post(ctx context.Context, path string, query, data url.Values, withToken bool) (int, []byte, error) {
	u := s.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(data.Encode()))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", defaultBaseURL)
	if withToken {
		req.Header.Set(loungeTokenHeader, s.loungeToken)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("youtube: request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, nil, fmt.Errorf("youtube: unable to read response from %s: %w", path, err)
	}
	if s.logf != nil {
		s.logf("youtube: POST %s %s -> %d (%d bytes)", path, describeForm(data), resp.StatusCode, len(body))
	}
	return resp.StatusCode, body, nil
}

// describeForm returns the form values in a stable order for logging.
func describeForm(data url.Values) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + data.Get(k)
	}
	return "[" + strings.Join(parts, " ") + "]"
}
