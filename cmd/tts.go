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
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/vishen/go-chromecast/log"
	"github.com/vishen/go-chromecast/tts"
)

// ttsCmd represents the tts command
var ttsCmd = &cobra.Command{
	Use:   "tts <message>",
	Short: "text-to-speech",
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) != 1 || args[0] == "" {
			log.Errorf("expected exactly one argument to convert to speech")
			return
		}

		googleServiceAccount, _ := cmd.Flags().GetString("google-service-account")
		if googleServiceAccount == "" {
			log.Errorf("--google-service-account is required")
			return
		}

		languageCode, _ := cmd.Flags().GetString("language-code")
		voiceName, _ := cmd.Flags().GetString("voice-name")
		speakingRate, _ := cmd.Flags().GetFloat32("speaking-rate")
		pitch, _ := cmd.Flags().GetFloat32("pitch")

		b, err := ioutil.ReadFile(googleServiceAccount)
		if err != nil {
			log.WithError(err).Error("unable to open google service account file")
			return
		}

		app, err := castApplication(cmd, args)
		if err != nil {
			log.WithError(err).Error("unable to get cast application")
			return
		}

		data, err := tts.Create(args[0], b, languageCode, voiceName, speakingRate, pitch)
		if err != nil {
			log.WithError(err).Error("unable to create tts instance")
			return
		}

		f, err := ioutil.TempFile("", "go-chromecast-tts")
		if err != nil {
			log.WithError(err).Error("unable to create temp file")
			return
		}
		defer os.Remove(f.Name())

		if _, err := f.Write(data); err != nil {
			log.WithError(err).Error("unable to write to temp file")
			return
		}
		if err := f.Close(); err != nil {
			log.WithError(err).Error("unable to close temp file")
			return
		}

		if err := app.Load(f.Name(), "audio/mp3", false, false, false); err != nil {
			log.WithError(err).Error("unable to load media to device")
		}
	},
}

func init() {
	rootCmd.AddCommand(ttsCmd)
	ttsCmd.Flags().String("google-service-account", "", "google service account JSON file")
	ttsCmd.Flags().String("language-code", "en-US", "text-to-speech Language Code (de-DE, ja-JP,...)")
	ttsCmd.Flags().String("voice-name", "en-US-Wavenet-G", "text-to-speech Voice (en-US-Wavenet-G, pl-PL-Wavenet-A, pl-PL-Wavenet-B, de-DE-Wavenet-A)")
	ttsCmd.Flags().Float32("speaking-rate", 1.0, "speaking rate")
	ttsCmd.Flags().Float32("pitch", 1.0, "pitch")
}
