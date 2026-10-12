package youtube

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// fakeLounge is a minimal fake of the YouTube Lounge API.
type fakeLounge struct {
	t *testing.T

	binds    int
	tokens   int
	commands []url.Values // Form of each command received.
	queries  []url.Values // Query of each command received.

	// Number of commands to reject with a 400 before accepting them.
	expireCommands int
}

func (f *fakeLounge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		f.t.Fatalf("unable to parse form: %v", err)
	}
	switch r.URL.Path {
	case "/" + loungeTokenPath:
		f.tokens++
		if got := r.PostForm.Get("screen_ids"); got != "screen-1" {
			f.t.Errorf("screen_ids = %q, want %q", got, "screen-1")
		}
		fmt.Fprintf(w, `{"screens":[{"screenId":"screen-1","loungeToken":"token-%d"}]}`, f.tokens)
	case "/" + bindPath:
		if got, want := r.Header.Get(loungeTokenHeader), fmt.Sprintf("token-%d", f.tokens); got != want {
			f.t.Errorf("lounge token header = %q, want %q", got, want)
		}
		if r.URL.Query().Get("SID") == "" {
			f.binds++
			if got := r.URL.Query().Get("RID"); got != "0" {
				f.t.Errorf("bind RID = %q, want 0", got)
			}
			fmt.Fprintf(w, "271\n[[0,[\"c\",\"sid-%d\",\"\",8]]\n,[1,[\"S\",\"gsession-%d\"]]\n,[2,[\"loungeStatus\",{}]]\n]\n", f.binds, f.binds)
			return
		}
		if f.expireCommands > 0 {
			f.expireCommands--
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.commands = append(f.commands, r.PostForm)
		f.queries = append(f.queries, r.URL.Query())
	default:
		f.t.Errorf("unexpected request to %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func newFakeLounge(t *testing.T) (*fakeLounge, *Session) {
	f := &fakeLounge{t: t}
	server := httptest.NewServer(f)
	t.Cleanup(server.Close)
	return f, NewSession("screen-1", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
}

func TestSessionPlayAndQueue(t *testing.T) {
	f, s := newFakeLounge(t)
	ctx := context.Background()

	if err := s.Play(ctx, "video-1", "list-1"); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if err := s.Add(ctx, "video-2"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.PlayNext(ctx, "video-3"); err != nil {
		t.Fatalf("PlayNext: %v", err)
	}

	if f.tokens != 1 || f.binds != 1 {
		t.Errorf("tokens = %d, binds = %d, want 1 of each", f.tokens, f.binds)
	}
	if len(f.commands) != 4 {
		t.Fatalf("received %d commands, want 4", len(f.commands))
	}

	want := []map[string]string{
		{"req0__sc": actionClear, "count": "1"},
		{"req1__sc": actionSetPlaylist, "req1_videoId": "video-1", "req1_listId": "list-1", "req1_currentIndex": "-1", "count": "1"},
		{"req2__sc": actionAdd, "req2_videoId": "video-2", "count": "1"},
		{"req3__sc": actionInsert, "req3_videoId": "video-3", "count": "1"},
	}
	for i, w := range want {
		for k, v := range w {
			if got := f.commands[i].Get(k); got != v {
				t.Errorf("command %d: %s = %q, want %q", i, k, got, v)
			}
		}
		q := f.queries[i]
		if q.Get("SID") != "sid-1" || q.Get("gsessionid") != "gsession-1" {
			t.Errorf("command %d: unexpected session ids: %v", i, q)
		}
		if got, want := q.Get("RID"), fmt.Sprint(i+1); got != want {
			t.Errorf("command %d: RID = %q, want %q", i, got, want)
		}
	}
}

func TestSessionRebindsWhenExpired(t *testing.T) {
	f, s := newFakeLounge(t)
	ctx := context.Background()

	if err := s.Play(ctx, "video-1", ""); err != nil {
		t.Fatalf("Play: %v", err)
	}
	f.expireCommands = 1
	if err := s.Add(ctx, "video-2"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if f.binds != 2 {
		t.Errorf("binds = %d, want 2", f.binds)
	}
	if len(f.commands) != 3 {
		t.Fatalf("received %d commands, want 3", len(f.commands))
	}
	// The command counter starts again in the new session.
	if got := f.commands[2].Get("req0_videoId"); got != "video-2" {
		t.Errorf("req0_videoId = %q, want %q", got, "video-2")
	}
	if got := f.queries[2].Get("SID"); got != "sid-2" {
		t.Errorf("SID = %q, want %q", got, "sid-2")
	}
}

func TestSessionErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"screens":[]}`)
	}))
	defer server.Close()

	s := NewSession("screen-1", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err := s.Play(context.Background(), "video-1", ""); err == nil {
		t.Error("expected an error when no lounge token is returned")
	}
}
