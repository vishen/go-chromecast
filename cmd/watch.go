// Copyright © 2018 Jonathan Pentecost <pentecostjonathan@gmail.com>
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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	"github.com/spf13/cobra"

	"github.com/vishen/go-chromecast/application"
	pb "github.com/vishen/go-chromecast/cast/proto"
	"github.com/vishen/go-chromecast/log"
)

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch all events sent from a chromecast device",
	Run: func(cmd *cobra.Command, args []string) {
		interval, _ := cmd.Flags().GetInt("interval")
		retries, _ := cmd.Flags().GetInt("retries")
		output, _ := cmd.Flags().GetString("output")

		o := outputNormal
		if strings.ToLower(output) == "json" {
			o = outputJSON
		}

		for i := 0; i < retries; i++ {
			retry := false
			app, err := castApplication(cmd, args)
			if err != nil {
				log.WithError(err).Info("unable to get cast application")
				time.Sleep(time.Second * 10)
				continue
			}
			done := make(chan struct{}, 1)
			go func() {
				for {
					if err := app.Update(); err != nil {
						log.WithError(err).Info("unable to update cast application")
						retry = true
						close(done)
						return
					}
					outputStatus(app, o)
					time.Sleep(time.Second * time.Duration(interval))
				}
			}()

			app.AddMessageFunc(func(msg *pb.CastMessage) {
				protocolVersion := msg.GetProtocolVersion()
				sourceID := msg.GetSourceId()
				destID := msg.GetDestinationId()
				namespace := msg.GetNamespace()

				payload := msg.GetPayloadUtf8()
				payloadBytes := []byte(payload)
				requestID, _ := jsonparser.GetInt(payloadBytes, "requestId")
				messageType, _ := jsonparser.GetString(payloadBytes, "type")
				// Only log requests that are broadcasted from the chromecast.
				if requestID != 0 {
					return
				}

				switch o {
				case outputJSON:
					json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
						"type":           messageType,
						"proto_version":  protocolVersion,
						"namespace":      namespace,
						"source_id":      sourceID,
						"destination_id": destID,
						"payload":        payload,
					})
				case outputNormal:
					log.Infof("CHROMECAST BROADCAST MESSAGE: type=%s proto=%s (namespace=%s) %s -> %s | %s", messageType, protocolVersion, namespace, sourceID, destID, payload)
				}
			})
			<-done
			if retry {
				// Sleep a little bit in-between retries
				log.Info("attempting a retry...")
				time.Sleep(time.Second * 10)
			}
		}
	},
}

type outputType int

const (
	outputNormal outputType = iota
	outputJSON
)

func outputStatus(app application.Application, outputType outputType) {
	castApplication, castMedia, castVolume := app.Status()

	switch outputType {
	case outputJSON:
		json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"application": castApplication,
			"media":       castMedia,
			"volume":      castVolume,
		})
	case outputNormal:
		if castApplication == nil {
			log.Infof("Idle, volume=%0.2f muted=%t", castVolume.Level, castVolume.Muted)
		} else if castApplication.IsIdleScreen {
			log.Infof("Idle (%s), volume=%0.2f muted=%t", castApplication.DisplayName, castVolume.Level, castVolume.Muted)
		} else if castMedia == nil {
			log.Infof("Idle (%s), volume=%0.2f muted=%t", castApplication.DisplayName, castVolume.Level, castVolume.Muted)
		} else {
			metadata := "unknown"
			if castMedia.Media.Metadata.Title != "" {
				md := castMedia.Media.Metadata
				metadata = fmt.Sprintf("title=%q, artist=%q", md.Title, md.Artist)
			}
			switch castMedia.Media.ContentType {
			case "x-youtube/video":
				metadata = fmt.Sprintf("id=\"%s\", %s", castMedia.Media.ContentId, metadata)
			}
			log.Infof(">> %s (%s), %s, time remaining=%.2fs/%.2fs, volume=%0.2f, muted=%t", castApplication.DisplayName, castMedia.PlayerState, metadata, castMedia.CurrentTime, castMedia.Media.Duration, castVolume.Level, castVolume.Muted)
		}
	}
}

func init() {
	watchCmd.Flags().Int("interval", 10, "interval between status poll in seconds")
	watchCmd.Flags().Int("retries", 10, "times to retry when losing chromecast connection")
	watchCmd.Flags().String("output", "normal", "output format: normal or json")
	rootCmd.AddCommand(watchCmd)
}
