# Chromecast

Implements a basic number of the google chromecast commands. Other than the basic commands, it also allows you to play media files from your computer either individually or in a playlist; the `playlist` command will look at all the files in a folder and play them sorted by numerically.

## Media Content Playable

Can play / load a local media file on your chromecast.

```
Supported Media formats:
    - MP3
    - AVI
    - MKV
    - MP4
    - WebM
```

If an unknown file is found, it will use `ffmpeg` to transcode it to MP4, and stream it to the chromecast.

## Cast DNS Lookup

A DNS multicast is used to determine the Chromecast and Google Home devices.

The cast DNS entry is also cached, this means that if you pass through the device name, `-n <name>`, or the
device uuid, `-u <uuid>`, the results will be cached and it will connect to the chromecast device instanly.

## Playlist

There is support for playing media items as a playlist.

If playing from a playlist, you are able to pass though the `--select` flag, and this will allow you to select
the media to start playing from. This is useful if you have already played some of the media and want to start
from one you haven't played yet.

A cache is kept of played media, so if you are playing media from a playlist, it will check to see what
media files you have recently played and play the next one from the playlist. `--continue=false` can be passed
through and this will start the playlist from the start.

## Watching a Device

If you would like to see what a device is sending, you are able to `watch` the protobuf messages being sent from your device:

```
$ go-chromecast watch
```

## Installing

### Install binaries
https://github.com/vishen/go-chromecast/releases

### Install the usual Go way:

```
$ go get -u github.com/vishen/go-chromecast
```

## Commands

```
Control your Google Chromecast or Google Home Mini from the
command line.

Usage:
  go-chromecast [command]

Available Commands:
  help        Help about any command
  load        Load and play media on the chromecast
  ls          List devices
  next        Play the next available media
  pause       Pause the currently playing media on the chromecast
  playlist    Load and play media on the chromecast
  previous    Play the previous available media
  restart     Restart the currently playing media
  rewind      Rewind by seconds the currently playing media
  seek        Seek by seconds into the currently playing media
  status      Current chromecast status
  stop        Stop casting
  unpause     Unpause the currently playing media on the chromecast
  watch       Watch all events sent from a chromecaset device

Flags:
      --debug                debug logging
  -d, --device string        chromecast device, ie: 'Chromecast' or 'Google Home Mini'
  -n, --device-name string   chromecast device name
      --disable-cache        disable the cache
  -h, --help                 help for go-chromecast
  -u, --uuid string          chromecast device uuid

Use "go-chromecast [command] --help" for more information about a command.
```
