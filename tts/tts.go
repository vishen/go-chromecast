package tts

import (
	"context"
	"time"

	"github.com/pkg/errors"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"google.golang.org/api/option"
	texttospeechpb "google.golang.org/genproto/googleapis/cloud/texttospeech/v1"
)

const (
	timeout = time.Second * 10
)

func Create(sentence string, serviceAccountKey []byte, languageCode string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := texttospeech.NewClient(ctx, option.WithCredentialsJSON(serviceAccountKey))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create texttospeech client")
	}

	req := texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: sentence},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: languageCode,
			SsmlGender:   texttospeechpb.SsmlVoiceGender_NEUTRAL,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: texttospeechpb.AudioEncoding_MP3,
		},
	}

	resp, err := client.SynthesizeSpeech(ctx, &req)
	if err != nil {
		return nil, errors.Wrap(err, "unable to synthesize speech")
	}
	return resp.AudioContent, nil
}
