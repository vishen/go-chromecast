# Chromecast
This implements a basic number of the google chromecast commands. Other than the basic commands, it also allows you to play media files from your computer either individually or in a playlist; the `playlist` command will look at all the files in a folder and play them, if it is a known media type, on at a time

Can play / load a local mp4 media file on your chromecast. Currently the chromecast default media receiver only supports the following formats:

```
Supported Media formats:
    - MKV
    - MP4
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
     status    current status of the chromecast
     pause     pause current media
     unpause   unpause current media
     reset     reset the current playing media
     end       go to end of current playing media
     seek      seek to a delta in the current playing media
     playlist  loads a playlist and plays the media
     load      load a mp4 media to play
     repl      repl for running commands
     help, h   Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug, -d    log debug information
   --help, -h     show help
   --version, -v  print the version
```

### Examples
```
$ make build
$ ./chromecast status                       # Shows the status of the chromecast
$ ./chromecast pause                        # Pauses the current chromecast application
$ ./chromecast unpause                      # Unpauses the current chromecast application
$ ./chromecast reset                        # Resets the current chromecast application to the beginning
$ ./chromecast end                          # Ends the current chromecast application
$ ./chromecast seek <int>                   # Seeks to a playing time in the current chromecast application
$ ./chromecast load <filename.mp4>          # Loads a media file to play on the chromecast application
$ ./chromecast load <filename> video/mp4    # Loads a media file with content type 'mp4' to play on the chromecast application
$ ./chromecast load <url> video/mp4         # The chromecast will play this media file from the url
$ ./chromecast playlist <folder>            # This will loop through all the files in a folder and play them one-by-one
$ ./chromecast repl                         # Starts an interactive session
```

## TODO
```
- Try: video/avi and video/msvideo for avi files - https://github.com/xat/castnow/blob/master/plugins/transcode.js
- Add bindings to convert a folders media files to mp4 via ffmpeg, and when first one is done start playing?
- Add flag to go into interactive mode after running command
- Add metadata to loaded media
- add sorting to playlist order
- Fix logging / debug information
- Add exploratory repl commands to try different things on the media and default connections
- Cache the dns result of the chromecast
- Store any loaded media that have been played
- Is it possible to convert avi files on the fly to mp4 or mkv (or one of the supported formats)?
```

## Resources
```
- https://github.com/xat/castnow
- https://github.com/trenskow/stream-transcoder.js/blob/master/lib/transcoder.js
- https://github.com/heartszhang/mp4box
- https://github.com/dhowden/tag
- https://github.com/balloob/pychromecast
- https://developers.google.com/cast/docs/reference/receiver/cast.receiver.media
- https://creativcoders.wordpress.com/2014/12/12/stream-live-mp4-video-with-ffmpeg-while-encoding/
- https://rigor.com/blog/2016/01/optimizing-mp4-video-for-fast-streaming
```
