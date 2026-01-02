package keymap

import (
	"reflect"

	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines a set of keybindings. To work for help it must satisfy
// key.Map. It could also very easily be a map[string]key.Binding.
type KeyMap struct {
	// Navigation Group
	HistUpKey      key.Binding `group:"Navigation"`
	HistDownKey    key.Binding `group:"Navigation"`
	LogUpKey       key.Binding `group:"Navigation"`
	LogDownKey     key.Binding `group:"Navigation"`
	LogLeftKey     key.Binding `group:"Navigation"`
	LogRightKey    key.Binding `group:"Navigation"`
	LogUpFastKey   key.Binding `group:"Navigation"`
	LogDownFastKey key.Binding `group:"Navigation"`
	LogTopKey      key.Binding `group:"Navigation"`
	LogBottomKey   key.Binding `group:"Navigation"`

	// Actions Group
	ToggleHistKey    key.Binding `group:"Actions"`
	OpenEditorKey    key.Binding `group:"Actions"`
	ClearLogKey      key.Binding `group:"Actions"`
	DeleteCmdKey     key.Binding `group:"Actions"`
	ResetKey         key.Binding `group:"Actions"`
	SendKey          key.Binding `group:"Actions"`
	ToggleSessionKey key.Binding `group:"Actions"`
	HelpKey          key.Binding `group:"Actions"`
	QuitKey          key.Binding `group:"Actions"`
	CloseKey         key.Binding `group:"Actions"`
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.HelpKey, k.QuitKey, k.ToggleHistKey, k.OpenEditorKey}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k KeyMap) FullHelp() [][]key.Binding {
	// Define the buckets for your groups
	var (
		navigation []key.Binding
		actions    []key.Binding
		other      []key.Binding // For keys without a tag
	)

	v := reflect.ValueOf(k)
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)
		fieldType := t.Field(i)

		// Ensure the field is a key.Binding
		if binding, ok := fieldVal.Interface().(key.Binding); ok {
			// Check the tag value
			tag := fieldType.Tag.Get("group")

			switch tag {
			case "Navigation":
				navigation = append(navigation, binding)
			case "Actions":
				actions = append(actions, binding)
			default:
				other = append(other, binding)
			}
		}
	}

	// Return the groups in the order you want them displayed in the UI
	// Filter out empty groups if necessary
	return [][]key.Binding{
		navigation,
		actions,
		other,
	}
}

// Default contains the default keybindings for the application.
var Default = KeyMap{
	HistUpKey: key.NewBinding(
		key.WithKeys("up", "ctrl+k"),
		key.WithHelp("↑/ctrl+k", "scroll commands"),
	),
	HistDownKey: key.NewBinding(
		key.WithKeys("down", "ctrl+j"),
		key.WithHelp("↓/ctrl+j", "scroll commands"),
	),
	LogUpKey: key.NewBinding(
		key.WithKeys("ctrl+up", "alt+k"),
		key.WithHelp("ctrl+↑/alt+k", "scroll log up"),
	),
	LogDownKey: key.NewBinding(
		key.WithKeys("ctrl+down", "alt+j"),
		key.WithHelp("ctrl+↓/alt+j", "scroll log down"),
	),
	LogLeftKey: key.NewBinding(
		key.WithKeys("ctrl+left", "alt+h"),
		key.WithHelp("ctrl+left/alt+h", "scroll log left"),
	),
	LogRightKey: key.NewBinding(
		key.WithKeys("ctrl+right", "alt+l"),
		key.WithHelp("ctrl+right/alt+l", "scroll log right"),
	),
	LogUpFastKey: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("PgUp", "scroll log up fast"),
	),
	LogDownFastKey: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("PgDn", "scroll log down fast"),
	),
	LogTopKey: key.NewBinding(
		key.WithKeys("home"),
		key.WithHelp("home", "log goto top"),
	),
	LogBottomKey: key.NewBinding(
		key.WithKeys("end"),
		key.WithHelp("end", "log goto bottom"),
	),
	ToggleHistKey: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "show commads"),
	),
	OpenEditorKey: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "open editor"),
	),
	QuitKey: key.NewBinding(
		key.WithKeys("ctrl+q"),
		key.WithHelp("ctrl+q", "quit"),
	),
	ClearLogKey: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("ctrl+l", "clear log"),
	),
	HelpKey: key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("ctrl+o", "show help"),
	),
	DeleteCmdKey: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "delete command"),
	),
	ResetKey: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "reset input"),
	),
	CloseKey: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close menu"),
	),
	SendKey: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "send data"),
	),
	ToggleSessionKey: key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("ctrl+x", "open/close port"),
	),
}
