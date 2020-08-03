package main

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	{
		// NOTE: Mostly copied from cmd/simulator go generate directive
		cmd := exec.Command(
			"openssl",
			"req", "-new", "-nodes", "-x509",
			"-out", "/tmp/server.pem",
			"-keyout", "/tmp/server.key",
			"-days", "1",
			"-subj", "/C=DE/ST=NRW/L=Earth/O=Chromecast/OU=IT/CN=chromecast/emailAddress=chromecast",
		)
		if err := cmd.Run(); err != nil {
			panic(err)
		}
	}
	if os.Getenv("TESTSCRIPT_COMMAND") == "" {
		cmd := exec.Command("go", "build",
			"-o", "/tmp/simulator",
			"./cmd/simulator",
		)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			panic(err)
		}
	}
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"go-chromecast": main1,
	}))
}

func TestCommands(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
	})
}

func TestCommandsWithSimulation(t *testing.T) {
	// Only run simulation on Linux, can't seem to get mdns to work
	// on mac, and haven't tested on windows.
	if runtime.GOOS != "linux" {
		t.Skip("can only test simulation on linux")
		return

	}
	testscript.Run(t, testscript.Params{
		Dir: "testdata/simulation",
	})
}
