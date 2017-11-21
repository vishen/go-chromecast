# Chromecast
This implements a basic number of the google chromecast commands. Can play / load a local mp4 media file on your chromecast. Currently the chromecast default media receiver only supports the following formats:

```
Supported Media formats:
    - AAC
    - MP3
    - MP4
    - WAV
    - WebM
```

## Commands
```
NAME:
   Chromecast - cli to interact with chromecast

USAGE:
   chromecast [global options] command [command options] [arguments...]

VERSION:
   0.0.1

COMMANDS:
     status   current status of the chromecast
     pause    pause current media
     unpause  unpause current media
     reset    reset the current playing media
     end      go to end of current playing media
     seek     seek to a delta in the current playing media
     load     load a mp4 media to play
     repl     repl for running commands
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug, -d    log debug information
   --help, -h     show help
   --version, -v  print the version
```

### Examples
```
$ make build
$ ./chromecast status               # Shows the status of the chromecast
$ ./chromecast pause                # Pauses the current chromecast application
$ ./chromecast unpause              # Unpauses the current chromecast application
$ ./chromecast reset                # Resets the current chromecast application to the beginning
$ ./chromecast end                  # Ends the current chromecast application
$ ./chromecast seek <int>           # Seeks to a playing time in the current chromecast application
$ ./chromecast load <filename.mp4>  # Loads an mp4 media file to play on the chromecast application (only mp4 supported)
$ ./chromecast repl                 # Starts an interactive session
```

## TODO
```
- Cache the dns result of the chromecast
- Store any loaded media that have been played
- Allow only the known media types that the chromecast default media type supports
- Allow mp4 urls to be loaded
- See if the queue next track works?
- Is it possible to convert avi and / or mkv files on the fly to mp4 (or one of the supported formats)?
- Fix logging / debug information
- Add exploratory repl commands to try different things on the media and default connections
```

## Resources
```
- https://github.com/heartszhang/mp4box
- https://github.com/dhowden/tag
- https://github.com/balloob/pychromecast
- https://developers.google.com/cast/docs/reference/receiver/cast.receiver.media
```
