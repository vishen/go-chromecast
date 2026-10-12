package application

import (
	"context"
	"time"

	"github.com/buger/jsonparser"
	"github.com/pkg/errors"
	"github.com/vishen/go-chromecast/cast"
	pb "github.com/vishen/go-chromecast/cast/proto"
	"github.com/vishen/go-chromecast/youtube"
)

const (
	// How long to wait for the YouTube app to give us its screen id, and
	// how often to ask for it. The app can take a few seconds to be ready
	// after being launched.
	youtubeScreenIDTimeout = 15 * time.Second
	youtubeScreenIDRetry   = 2 * time.Second
)

// LoadYouTube plays a YouTube video (and/or playlist) on the YouTube app,
// replacing whatever was in the queue. One of videoID or playlistID must be
// set.
func (a *Application) LoadYouTube(videoID, playlistID string) error {
	session, err := a.youtubeSession()
	if err != nil {
		return err
	}
	return errors.Wrap(session.Play(context.Background(), videoID, playlistID), "unable to play youtube video")
}

// QueueYouTube adds a YouTube video to the queue of the YouTube app. If
// playNext is true it is added just after the current video, otherwise it is
// added to the end of the queue.
func (a *Application) QueueYouTube(videoID string, playNext bool) error {
	session, err := a.youtubeSession()
	if err != nil {
		return err
	}
	if playNext {
		err = session.PlayNext(context.Background(), videoID)
	} else {
		err = session.Add(context.Background(), videoID)
	}
	return errors.Wrap(err, "unable to queue youtube video")
}

// youtubeSession makes sure the YouTube app is running on the device and
// returns a lounge session that can control it.
func (a *Application) youtubeSession() (*youtube.Session, error) {
	if a.youtube != nil && a.application != nil && a.application.AppId == youtube.AppID {
		return a.youtube, nil
	}
	if err := a.ensureIsAppID(youtube.AppID); err != nil {
		return nil, errors.Wrap(err, "unable to change to the youtube app")
	}
	screenID, err := a.youtubeScreenID()
	if err != nil {
		return nil, err
	}
	a.log("youtube screen id: %s", screenID)
	a.youtube = youtube.NewSession(screenID, youtube.WithLogger(a.log))
	return a.youtube, nil
}

// youtubeScreenID asks the running YouTube app for its screen id.
func (a *Application) youtubeScreenID() (string, error) {
	if a.application == nil {
		return "", ErrApplicationNotSet
	}

	// The response doesn't include our request id, so we need to listen
	// to all the messages for it.
	screenIDChan := make(chan string, 1)
	a.AddMessageFunc(func(msg *pb.CastMessage) {
		if msg.GetNamespace() != youtube.Namespace {
			return
		}
		payload := []byte(msg.GetPayloadUtf8())
		if messageType, _ := jsonparser.GetString(payload, "type"); messageType != youtube.MessageTypeScreenID {
			return
		}
		screenID, _ := jsonparser.GetString(payload, "data", "screenId")
		if screenID == "" {
			return
		}
		select {
		case screenIDChan <- screenID:
		default:
		}
	})

	if err := a.sendMediaConn(&cast.ConnectHeader); err != nil {
		return "", errors.Wrap(err, "unable to connect to the youtube app")
	}

	timeout := time.After(youtubeScreenIDTimeout)
	retry := time.NewTicker(youtubeScreenIDRetry)
	defer retry.Stop()
	for {
		payload := &cast.PayloadHeader{Type: youtube.MessageTypeGetScreenID}
		if _, err := a.send(payload, defaultSender, a.application.TransportId, youtube.Namespace); err != nil {
			return "", errors.Wrap(err, "unable to request the youtube screen id")
		}
		select {
		case screenID := <-screenIDChan:
			return screenID, nil
		case <-retry.C:
		case <-timeout:
			return "", errors.New("timed out waiting for the youtube screen id")
		}
	}
}
