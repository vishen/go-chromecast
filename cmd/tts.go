// Copyright © 2019 Jonathan Pentecost <pentecostjonathan@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/tts"
)

// ttsCmd represents the tts command
var ttsCmd = &cobra.Command{
	Use:   "tts <message>",
	Short: "text-to-speech",
	Run: func(cmd *cobra.Command, args []string) {
		GoogleServiceAccount, _ := cmd.Flags().GetString("google-service-account")
		LanguageCode, _ := cmd.Flags().GetString("language-code")
		VoiceName, _ := cmd.Flags().GetString("voice-name")
		SpeakingRate, _ := cmd.Flags().GetFloat32("speaking-rate")
		Pitch, _ := cmd.Flags().GetFloat32("pitch")
		Ssml, _ := cmd.Flags().GetBool("ssml")

		app := NewCast(cmd)
		app.Tts(TTSOpts{
			GoogleServiceAccount,
			LanguageCode,
			VoiceName,
			SpeakingRate,
			Pitch,
			Ssml,
		}, args)
	},
}

type TTSOpts struct {
	GoogleServiceAccount string
	LanguageCode         string
	VoiceName            string
	SpeakingRate         float32
	Pitch                float32
	Ssml                 bool
}

// Tts exports the tts command
func (a *App) Tts(opts TTSOpts, args []string) {
	if len(args) != 1 || args[0] == "" {
		exit("expected exactly one argument to convert to speech")
		return
	}

	if opts.GoogleServiceAccount == "" {
		exit("--google-service-account is required")
		return
	}

	b, err := os.ReadFile(opts.GoogleServiceAccount)
	if err != nil {
		exit("unable to open google service account file: %v", err)
	}

	app, err := a.castApplication()
	if err != nil {
		exit("unable to get cast application: %v", err)
	}

	data, err := tts.Create(args[0], b, opts.LanguageCode, opts.VoiceName, opts.SpeakingRate, opts.Pitch, opts.Ssml)
	if err != nil {
		exit("unable to create tts: %v", err)
	}

	f, err := os.CreateTemp("", "go-chromecast-tts")
	if err != nil {
		exit("unable to create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	if _, err := f.Write(data); err != nil {
		exit("unable to write to temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		exit("unable to close temp file: %v", err)
	}

	if err := app.Load(f.Name(), 0, "audio/mp3", false, false, false); err != nil {
		exit("unable to load media to device: %v", err)
	}
}

func init() {
	rootCmd.AddCommand(ttsCmd)
	ttsCmd.Flags().String("google-service-account", "", "google service account JSON file")
	ttsCmd.Flags().String("language-code", "en-US", "text-to-speech Language Code (de-DE, ja-JP,...)")
	ttsCmd.Flags().String("voice-name", "en-US-Wavenet-G", "text-to-speech Voice (en-US-Wavenet-G, pl-PL-Wavenet-A, pl-PL-Wavenet-B, de-DE-Wavenet-A)")
	ttsCmd.Flags().Float32("speaking-rate", 1.0, "speaking rate")
	ttsCmd.Flags().Float32("pitch", 1.0, "pitch")
	ttsCmd.Flags().Bool("ssml", false, "use SSML")
}
