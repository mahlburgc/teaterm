package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const settingsFileName = "teaterm.toml"

// Command history display modes.
const (
	// HistModePopup: ctrl+r opens the history, enter selects the highlighted
	// command into the input and closes the history.
	HistModePopup = "popup"
	// HistModePanel: ctrl+r shows/hides the history. It stays open while
	// working, enter sends the input and up/down recall commands shell-style.
	HistModePanel = "panel"
)

// Border styles.
const (
	// BordersFull draws a rounded box around every component.
	BordersFull = "full"
	// BordersMinimal separates components only by a single line.
	BordersMinimal = "minimal"
)

// Size is a size given either as absolute cells (rows/columns) or as a
// percentage of the available space. The zero value means "auto".
type Size struct {
	Value   int
	Percent bool
}

// parseSize parses "10" (absolute) or "30%" (percentage).
func parseSize(s string) (Size, error) {
	s = strings.TrimSpace(s)
	percent := strings.HasSuffix(s, "%")
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(s, "%")))
	if err != nil || n <= 0 || (percent && n > 100) {
		return Size{}, fmt.Errorf("invalid size %q: use a positive number (e.g. \"10\") or a percentage (e.g. \"30%%\")", s)
	}
	return Size{Value: n, Percent: percent}, nil
}

// IsAuto reports whether no explicit size is set.
func (s Size) IsAuto() bool {
	return s.Value == 0
}

// Resolve returns the size in cells for the given total available cells.
// Auto resolves to 10 cells, but at most a third of the total.
func (s Size) Resolve(total int) int {
	if s.IsAuto() {
		return min(10, total/3)
	}
	if s.Percent {
		return total * s.Value / 100
	}
	return s.Value
}

// CmdHistSettings configures the command history.
type CmdHistSettings struct {
	Mode   string
	Height Size
}

// LayoutSettings configures the general look of the TUI.
type LayoutSettings struct {
	Borders string
}

// Settings holds the user settings read from teaterm.toml.
type Settings struct {
	CmdHist CmdHistSettings
	Layout  LayoutSettings
}

// DefaultSettings returns the settings used if no settings file exists or
// a value is not set.
func DefaultSettings() Settings {
	return Settings{
		CmdHist: CmdHistSettings{
			Mode:   HistModePopup,
			Height: Size{}, // auto
		},
		Layout: LayoutSettings{
			Borders: BordersFull,
		},
	}
}

// settingsFile mirrors the TOML file layout. Pointers distinguish unset
// values from zero values.
type settingsFile struct {
	CmdHist struct {
		Mode   *string `toml:"mode"`
		Height any     `toml:"height"` // int (rows) or string ("10", "30%")
	} `toml:"cmdhist"`
	Layout struct {
		Borders *string `toml:"borders"`
	} `toml:"layout"`
}

// LoadSettings reads ~/.config/teaterm/teaterm.toml. A missing file is not
// an error, defaults are used instead.
func LoadSettings() (Settings, error) {
	return loadSettingsFile(filepath.Join(configDir(), settingsFileName))
}

func loadSettingsFile(path string) (Settings, error) {
	settings := DefaultSettings()

	var file settingsFile
	md, err := toml.DecodeFile(path, &file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, nil
		}
		return settings, fmt.Errorf("%s: %w", path, err)
	}

	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return settings, fmt.Errorf("%s: unknown setting %q", path, undecoded[0].String())
	}

	if file.CmdHist.Mode != nil {
		switch *file.CmdHist.Mode {
		case HistModePopup, HistModePanel:
			settings.CmdHist.Mode = *file.CmdHist.Mode
		default:
			return settings, fmt.Errorf("%s: cmdhist.mode: invalid value %q, allowed: %q, %q",
				path, *file.CmdHist.Mode, HistModePopup, HistModePanel)
		}
	}

	if file.Layout.Borders != nil {
		switch *file.Layout.Borders {
		case BordersFull, BordersMinimal:
			settings.Layout.Borders = *file.Layout.Borders
		default:
			return settings, fmt.Errorf("%s: layout.borders: invalid value %q, allowed: %q, %q",
				path, *file.Layout.Borders, BordersFull, BordersMinimal)
		}
	}

	if file.CmdHist.Height != nil {
		var raw string
		switch v := file.CmdHist.Height.(type) {
		case int64:
			raw = strconv.FormatInt(v, 10)
		case string:
			raw = v
		default:
			return settings, fmt.Errorf("%s: cmdhist.height: invalid value %v", path, v)
		}
		height, err := parseSize(raw)
		if err != nil {
			return settings, fmt.Errorf("%s: cmdhist.height: %w", path, err)
		}
		settings.CmdHist.Height = height
	}

	return settings, nil
}
