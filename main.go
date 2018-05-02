package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cli "github.com/urfave/cli"
)

const (
	version = "0.0.1"
)

var (
	castApplication *CastApplication
)

func initialise(ctx *cli.Context) error {
	log.Println("Initalising")
	// if uuid is not specified, the first chromecast is used
	cUuid := ctx.GlobalString("uuid")
	castConn := NewCastConnection(cUuid, ctx.GlobalBool("debug"))
	if castConn == nil {
		if cUuid == "" {
			log.Fatal("no chromecast found")
		} else {
			log.Fatalf("no chromecast found with uuid: %s", cUuid)
		}
	}
	log.Println("Got cast connection")
	castConn.connect()
	log.Println("Finished connecting")
	go castConn.receiveLoop()

	castApplication = NewCastApplication(castConn)
	if err := castApplication.Start(); err != nil {
		log.Fatalf("error starting app: %s", err)
	}

	return nil

}

func shutdown(ctx *cli.Context) error {
	castApplication.Close()
	return nil
}

func printStatus() {
	castApplication.Update()

	a := castApplication.application
	m := castApplication.media

	if m != (Media{}) {
		metadata := m.Media.Metadata
		fmt.Printf("%s - %s (%+v) current_time=%0.2f [volume=%v]\n", m.PlayerState, a.DisplayName, metadata, m.CurrentTime, m.Volume)
	} else {
		fmt.Printf("Chromecast is idle - [volume=%v]\n", castApplication.volume)
	}
}

func repl(c *cli.Context) error {

	commands := []string{
		"exit", "close", "status", "pause", "play", "unpause",
		"load", "seek", "reset", "end",
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Repl for communicating with your chromecast")
	fmt.Printf("Available commands: %s\n", strings.Join(commands, ", "))
	for {
		fmt.Printf(">> ")
		scanned := scanner.Scan()
		if !scanned {
			return nil
		}

		line := scanner.Text()
		lineSplit := strings.Split(line, " ")

		switch lineSplit[0] {
		case "exit", "close":
			return nil
		case "status":
			printStatus()
		case "pause":
			castApplication.Pause()
		case "unpause", "play":
			castApplication.Unpause()
		case "load":
			fmt.Println(lineSplit[0:])
		case "seek":
			value, err := strconv.Atoi(lineSplit[1])
			if err != nil {
				fmt.Printf("Error converting '%s' to integer: %s\n", lineSplit[1], err)
				continue
			}
			castApplication.Seek(value)
		case "reset":
			castApplication.Seek(0)
		case "end":
			castApplication.Seek(100000)
		default:
			fmt.Printf("Unknown command '%s'\n", lineSplit[0])
		}
	}
	return nil
}

func main() {
	log.Println("Starting new app")
	app := cli.NewApp()
	app.Version = version
	app.Name = "Chromecast"
	app.HelpName = "chromecast"
	app.Usage = "cli to interact with chromecast"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "log debug information",
		},
		cli.StringFlag{
			Name:  "uuid, u",
			Usage: "specify chromecast uuid",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "list",
			Usage: "list available chromecasts",
			Action: func(c *cli.Context) error {
				printCastEntries()
				return nil
			},
		},
		{
			Name:  "status",
			Usage: "current status of the chromecast",
			Action: func(c *cli.Context) error {
				printStatus()
				return nil
			},
			Before: initialise,
			After:  shutdown,
		},
		{
			Name:  "pause",
			Usage: "pause current media",
			Action: func(c *cli.Context) error {
				castApplication.Pause()
				return nil
			},
			Before: initialise,
			After:  shutdown,
		},
		{
			Name:  "unpause",
			Usage: "unpause current media",
			Action: func(c *cli.Context) error {
				castApplication.Unpause()
				return nil
			},
			Before: initialise,
			After:  shutdown,
		},
		{
			Name:  "reset",
			Usage: "reset the current playing media",
			Action: func(c *cli.Context) error {
				castApplication.Seek(0)
				return nil
			},
			Before: initialise,
			After:  shutdown,
		},
		{
			Name:  "end",
			Usage: "go to end of current playing media",
			Action: func(c *cli.Context) error {
				castApplication.Seek(100000)
				return nil
			},
			Before: initialise,
			After:  shutdown,
		},
		{
			Name:  "seek",
			Usage: "seek to a delta in the current playing media",
			Action: func(c *cli.Context) error {
				delta := c.Args().First()
				value, err := strconv.Atoi(delta)
				if err != nil {
					fmt.Printf("Error converting '%s' to integer", delta)
					return err
				}
				castApplication.Seek(value)
				return nil
			},
			Before: initialise,
			After:  shutdown,
		},
		{
			Name:  "playlist",
			Usage: "loads a playlist and plays the media",
			Action: func(c *cli.Context) error {
				folder := c.Args().Get(0)
				files, err := ioutil.ReadDir(folder)
				if err != nil {
					log.Fatal(err)
				}

				playlist := make([]string, 0, len(files))

				for _, file := range files {
					if _, err := getLikelyContentType(file.Name()); err == nil {
						playlist = append(playlist, file.Name())
					}
				}

				fmt.Printf("Found '%d' valid media files and will play in the following order\n", len(playlist))
				for i, filename := range playlist {
					fmt.Printf("%d) %s\n", i, filename)
				}

				// TODO(vishen): Should ask if this is playlist order is alright
				// TODO(vishen): Allow for different ordering?
				// TODO(vishen): When we have finished the playlist, we should
				//		check to see if there is any new media added
				for _, filename := range playlist {
					contentType, _ := getLikelyContentType(filename)
					fmt.Printf("Playing '%s'\n", filename)
					err := castApplication.PlayMedia(folder+filename, contentType, true)
					fmt.Printf("error: %v\n", err)
				}
				return nil

			},
			Before: initialise,
			After:  shutdown,
		},
		{
			Name:  "load",
			Usage: "load media to play",
			Action: func(c *cli.Context) error {
				filenameOrUrl := c.Args().Get(0)
				contentType := c.Args().Get(1)
				fmt.Println(filenameOrUrl, contentType)

				if contentType == "" {
					var err error
					contentType, err = getLikelyContentType(filenameOrUrl)
					if err != nil {
						log.Printf("Unable to find content type: %s", err)

						// For now only allow .avi files to be transcoded to .mp4
						if ext := path.Ext(filenameOrUrl); ext != ".avi" {
							log.Printf("Not able to transcode '%s' files to mp4\n", ext)
							return nil
						}

						// Start transcoding ffmpeg to a file, and attempt to get the server to
						// serve the file as it is transcoding
						tmpDir, err := ioutil.TempDir("", "chromecast")
						if err != nil {
							log.Fatal(err)
						}
						defer os.RemoveAll(tmpDir)

						writeFilepath := filepath.Join(tmpDir, fmt.Sprintf("%s-%d.mp4", "holder", time.Now().Unix()))
						cmd := exec.Command(
							"ffmpeg",
							"-i", filenameOrUrl,
							"-vcodec", "h264",
							"-f", "mp4",
							"-movflags", "frag_keyframe+faststart",
							"-strict", "-experimental",
							writeFilepath,
						)

						fmt.Printf("Starting transcoding\n")
						if err := cmd.Start(); err != nil {
							log.Fatal(err)
						}

						go func() {
							sigc := make(chan os.Signal, 1)
							signal.Notify(sigc, os.Interrupt, os.Kill)
							defer signal.Stop(sigc)
							<-sigc

							fmt.Println("Killing running ffmpeg...")
							if err := cmd.Process.Kill(); err != nil {
								fmt.Printf("Unable to kill ffmpeg process: %s\n", err)
							}

						}()

						filenameOrUrl = writeFilepath
						contentType = "video/mp4"

						// Give it some time to start transcoding the media
						time.Sleep(time.Second * 10)

					}
				}
				if err := castApplication.PlayMedia(filenameOrUrl, contentType, true); err != nil {
					fmt.Printf("Error: %s\n", err)
				}
				return nil
			},
			Before: initialise,
			After:  shutdown,
		},
		{
			Name:   "repl",
			Usage:  "repl for running commands",
			Action: repl,
			Before: initialise,
			After:  shutdown,
		},
	}

	app.Run(os.Args)

}
