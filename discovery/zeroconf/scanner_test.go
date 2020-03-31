package zeroconf_test

import (
	"github.com/vishen/go-chromecast/discovery"
	"github.com/vishen/go-chromecast/discovery/zeroconf"
)

// Ensure interface is satisfied
var _ discovery.Scanner = zeroconf.Scanner{}
