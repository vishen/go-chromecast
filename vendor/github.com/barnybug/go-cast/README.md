[![Build status](https://secure.travis-ci.org/barnybug/go-cast.png?branch=master)](https://secure.travis-ci.org/barnybug/go-cast)

# go-cast

A command line tool to control Google Chromecast devices.

## Installation

Download the latest binaries from:
https://github.com/barnybug/go-cast/releases/latest

    $ sudo mv cast-my-platform /usr/local/bin/cast
    $ sudo chmod +x /usr/local/bin/cast

## Usage

	$ cast help

Play a media file:

	$ cast --name Hifi media play http://url/file.mp3

Stop playback:

	$ cast --name Hifi media stop

Set volume:

	$ cast --name Hifi volume 0.5

Close app on the Chromecast:

	$ cast --name Hifi quit

## Bug reports

Please open a github issue including cast version number `cast --version`.

## Pull requests

Pull requests are gratefully received!

- please 'gofmt' the code.

## Credits

Based on go library port by [ninjasphere](https://github.com/ninjasphere/node-cast)
