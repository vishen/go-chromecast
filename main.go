package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	cli "github.com/urfave/cli"
)

const (
	version = "0.0.1"
)

var (
	castApplication *CastApplication
)

func initialise(ctx *cli.Context) error {
	castConn := NewCastConnection()
	castConn.debug = ctx.GlobalBool("debug")
	castConn.connect()
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
	}
	app.Commands = []cli.Command{
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
				for _, filename := range playlist {
					contentType, _ := getLikelyContentType(filename)
					fmt.Printf("Playing '%s'\n", filename)
					err := castApplication.PlayMedia(folder+filename, contentType)
					fmt.Printf("error: %v\n", err)
				}
				return nil

			},
			Before: initialise,
			After:  shutdown,
		},
		{
			Name:  "load",
			Usage: "load a mp4 media to play",
			Action: func(c *cli.Context) error {
				filenameOrUrl := c.Args().Get(0)
				contentType := c.Args().Get(1)
				if contentType == "" {
					var err error
					contentType, err = getLikelyContentType(filenameOrUrl)
					if err != nil {
						log.Printf("Unable to find content type: %s", err)
						return err
					}
				}
				return castApplication.PlayMedia(filenameOrUrl, contentType)
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
