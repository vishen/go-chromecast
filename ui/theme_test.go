package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestColourEscape(t *testing.T) {
	tests := map[string]string{
		"blue":       "\033[0;34m",
		"bold blue":  "\033[0;34;1m",
		"Bold  RED":  "\033[0;31;1m",
		"default":    "\033[0m",
		"bold":       "\033[0;1m",
		"":           "\033[0m",
		"white bold": "\033[0;37;1m",
	}
	for colour, want := range tests {
		got, err := colourEscape(colour)
		if err != nil {
			t.Errorf("colourEscape(%q): %v", colour, err)
		}
		if got != want {
			t.Errorf("colourEscape(%q) = %q, want %q", colour, got, want)
		}
	}

	for _, colour := range []string{"pink", "red blue", "bold bright red"} {
		if _, err := colourEscape(colour); err == nil {
			t.Errorf("colourEscape(%q): expected an error", colour)
		}
	}
}

func TestLoadTheme(t *testing.T) {
	write := func(content string) string {
		path := filepath.Join(t.TempDir(), "theme.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	// The colours that aren't in the file come from the default theme.
	theme, err := LoadTheme(write(`{"text": "default", "value": "bold"}`))
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	want := DefaultTheme()
	want.Text = "default"
	want.Value = "bold"
	if theme != want {
		t.Errorf("LoadTheme = %+v, want %+v", theme, want)
	}

	if _, err := LoadTheme(write(`{"progress": "pink"}`)); err == nil {
		t.Error("expected an error with an unknown colour")
	}
	if _, err := LoadTheme(write(`not json`)); err == nil {
		t.Error("expected an error with an invalid file")
	}
	if _, err := LoadTheme(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("expected an error with a missing file")
	}
}

func TestSetTheme(t *testing.T) {
	t.Cleanup(func() { SetTheme(DefaultTheme()) })

	if err := SetTheme(Theme{Text: "default", Value: "bold", Volume: "green", VolumeMuted: "bold green", Progress: "cyan"}); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}
	if normalTextColour != "\033[0m" || boldTextColour != "\033[0;1m" || progressColour != "\033[0;36m" {
		t.Errorf("unexpected colours: %q %q %q", normalTextColour, boldTextColour, progressColour)
	}

	// An invalid theme doesn't change anything.
	if err := SetTheme(Theme{Text: "pink"}); err == nil {
		t.Error("expected an error with an unknown colour")
	}
	if normalTextColour != "\033[0m" {
		t.Errorf("the colours changed with an invalid theme: %q", normalTextColour)
	}
}
