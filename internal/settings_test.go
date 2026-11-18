package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		in      string
		want    Size
		wantErr bool
	}{
		{"10", Size{Value: 10}, false},
		{" 25% ", Size{Value: 25, Percent: true}, false},
		{"100%", Size{Value: 100, Percent: true}, false},
		{"0", Size{}, true},
		{"-3", Size{}, true},
		{"101%", Size{}, true},
		{"abc", Size{}, true},
		{"%", Size{}, true},
	}
	for _, tc := range tests {
		got, err := parseSize(tc.in)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("parseSize(%q) = %v, %v; want %v, err=%v", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestSizeResolve(t *testing.T) {
	if got := (Size{Value: 12}).Resolve(60); got != 12 {
		t.Errorf("absolute: got %d, want 12", got)
	}
	if got := (Size{Value: 25, Percent: true}).Resolve(60); got != 15 {
		t.Errorf("percent: got %d, want 15", got)
	}
	if got := (Size{}).Resolve(60); got != 10 {
		t.Errorf("auto, large screen: got %d, want 10", got)
	}
	if got := (Size{}).Resolve(18); got != 6 {
		t.Errorf("auto, small screen: got %d, want 6", got)
	}
}

func writeSettings(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), settingsFileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSettingsFile(t *testing.T) {
	t.Run("missing file uses defaults", func(t *testing.T) {
		got, err := loadSettingsFile(filepath.Join(t.TempDir(), "nope.toml"))
		if err != nil || got != DefaultSettings() {
			t.Fatalf("got %v, %v; want defaults", got, err)
		}
	})

	t.Run("valid file", func(t *testing.T) {
		got, err := loadSettingsFile(writeSettings(t, "[cmdhist]\nmode = \"panel\"\nheight = \"30%\"\n"))
		if err != nil {
			t.Fatal(err)
		}
		want := Settings{CmdHist: CmdHistSettings{Mode: HistModePanel, Height: Size{Value: 30, Percent: true}}}
		if got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("integer height", func(t *testing.T) {
		got, err := loadSettingsFile(writeSettings(t, "[cmdhist]\nheight = 15\n"))
		if err != nil || got.CmdHist.Height != (Size{Value: 15}) || got.CmdHist.Mode != HistModePopup {
			t.Fatalf("got %v, %v", got, err)
		}
	})

	errCases := map[string]string{
		"bad mode":     "[cmdhist]\nmode = \"floating\"\n",
		"bad height":   "[cmdhist]\nheight = \"big\"\n",
		"bad type":     "[cmdhist]\nheight = true\n",
		"unknown key":  "[cmdhist]\nwidth = \"25%\"\n",
		"invalid toml": "[cmdhist\n",
	}
	for name, content := range errCases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadSettingsFile(writeSettings(t, content)); err == nil {
				t.Fatal("expected error")
			} else if !strings.Contains(err.Error(), settingsFileName) {
				t.Errorf("error %q does not name the file", err)
			}
		})
	}
}
