package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Theme holds the colours of the UI. Each one is the name of a colour
// ("default", "black", "red", "green", "yellow", "blue", "magenta", "cyan" or
// "white", "default" being the foreground colour of the terminal), optionally
// preceded by "bold": ie "blue", "bold red" or just "bold".
type Theme struct {
	// Text is the colour of the labels:
	Text string `json:"text"`
	// Value is the colour of the values and of the keys:
	Value string `json:"value"`
	// Volume is the colour of the volume bar:
	Volume string `json:"volume"`
	// VolumeMuted is the colour of the volume when it is muted:
	VolumeMuted string `json:"volume_muted"`
	// Progress is the colour of the progress bar:
	Progress string `json:"progress"`
}

// DefaultTheme returns the colours used when no theme is set:
func DefaultTheme() Theme {
	return Theme{
		Text:        "blue",
		Value:       "bold blue",
		Volume:      "red",
		VolumeMuted: "bold red",
		Progress:    "yellow",
	}
}

// DefaultThemePath returns where the theme is looked for when no other file
// is given: "go-chromecast/theme.json" in the config directory of the user.
func DefaultThemePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "go-chromecast", "theme.json"), nil
}

// LoadTheme reads a theme from a JSON file. The colours that aren't in the
// file keep the value they have in the default theme.
func LoadTheme(path string) (Theme, error) {
	theme := DefaultTheme()

	b, err := os.ReadFile(path)
	if err != nil {
		return theme, err
	}
	if err := json.Unmarshal(b, &theme); err != nil {
		return theme, fmt.Errorf("invalid theme %q: %w", path, err)
	}
	if _, err := theme.colours(); err != nil {
		return theme, fmt.Errorf("invalid theme %q: %w", path, err)
	}
	return theme, nil
}

// SetTheme changes the colours of the UI:
func SetTheme(theme Theme) error {
	colours, err := theme.colours()
	if err != nil {
		return err
	}

	normalTextColour = colours[0]
	boldTextColour = colours[1]
	volumeColour = colours[2]
	volumeMutedColour = colours[3]
	progressColour = colours[4]
	return nil
}

// colours returns the escape sequences of the colours of the theme, in the
// order of the fields.
func (t Theme) colours() ([]string, error) {
	fields := []struct {
		name, colour string
	}{
		{"text", t.Text},
		{"value", t.Value},
		{"volume", t.Volume},
		{"volume_muted", t.VolumeMuted},
		{"progress", t.Progress},
	}

	colours := make([]string, len(fields))
	for i, field := range fields {
		colour, err := colourEscape(field.colour)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field.name, err)
		}
		colours[i] = colour
	}
	return colours, nil
}

// colourCodes are the SGR codes of the foreground colours:
var colourCodes = map[string]string{
	"default": "",
	"black":   ";30",
	"red":     ";31",
	"green":   ";32",
	"yellow":  ";33",
	"blue":    ";34",
	"magenta": ";35",
	"cyan":    ";36",
	"white":   ";37",
}

// colourEscape returns the escape sequence of a colour of a theme:
func colourEscape(colour string) (string, error) {
	// Always reset first, as the attributes add up to the current ones:
	escape := "\033[0"
	bold := ""
	seenColour := false

	for _, word := range strings.Fields(strings.ToLower(colour)) {
		if word == "bold" {
			bold = ";1"
			continue
		}
		code, ok := colourCodes[word]
		if !ok || seenColour {
			return "", fmt.Errorf("unknown colour %q", colour)
		}
		seenColour = true
		escape += code
	}
	return escape + bold + "m", nil
}
