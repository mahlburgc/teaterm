# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Teaterm is a serial terminal TUI (like `tio`) built with the Bubble Tea framework.

## Commands

```shell
go build ./...      # build
go vet ./...        # vet
go test ./...       # run tests (internal/tui_test.go simulates the bubbletea event loop)

# Run with mocked serial port (no hardware needed) and debug logging:
export TEATERM_DBG_LOG=1    # writes to ./debug.log (tail -f debug.log in a second shell)
export TEATERM_MOCK_PORT=1  # use mockport.go instead of a real port
go run .
```

CLI flags: `-l` (list ports), `-p <port>` (default `/dev/ttyUSB0`), `-t` (timestamps), `-log` / `-logpath` (serial logfile), `-e` (show escape chars).

Note: stdlib `log` output is discarded unless `TEATERM_DBG_LOG` is set, so `log.Print*` is the debug-logging mechanism throughout the code.

## Architecture

Standard Elm-architecture Bubble Tea app. Entry point `teaterm.go` parses flags/config, opens the serial port (real or mock), then calls `internal.RunTui`.

### Component composition

`internal/tui.go` holds the root `model`, which composes independent sub-models, each its own package with the usual `New`/`Init`/`Update`/`View` shape:

- `internal/msglog` — received-message viewport (scrolling, filtering, timestamps, external editor)
- `internal/cmdhist` — command history popup with fzf-style fuzzy filtering (`sahilm/fuzzy`)
- `internal/input` — textarea for typing commands, autocomplete suggestions
- `internal/session` — serial connection lifecycle: port reading loop, auto-reconnect with context cancellation, connect/disconnect status
- `internal/footer` — status bar
- `internal/help-overlay` — help popup rendered via `bubbletea-overlay`

The root `Update` broadcasts every `tea.Msg` to **all** sub-models unconditionally, then handles layout/global keys itself. Sub-models communicate exclusively through shared message types defined in the top-level `events` package (e.g. `events.SendMsg`, `events.SerialRxMsgReceived`, `events.HistCmdSelected`, `events.ConnectionStatusMsg`) — never by direct calls between sub-models. New cross-component interactions should add an event type there.

Order matters in the root `Update`: e.g. Enter is intercepted before the input sub-model sees it when the cmd-history popup is open.

### Key conventions

- **Keybindings** are centralized in `internal/keymap` (`keymap.Default`); sub-models match keys with `key.Matches(msg, keymap.Default.XxxKey)`. The `group:"..."` struct tags drive the help overlay grouping via reflection.
- **Styles** are centralized in `internal/styles`.
- **Mouse support** uses `bubblezone`: components mark clickable regions with `zone.Mark`, and the root `View` wraps output in `zone.Scan`.
- **Serial port** is passed around as `*io.ReadWriteCloser` (pointer, because reconnects replace the underlying port object; the deferred close in `main` must close the *current* port). The mock port (`internal/mockport.go`) implements the same interface.
- **App restart loop**: `RunTui` runs the Bubble Tea program in a loop; returning from an external editor sets `restartApp` and quits, then a fresh program is started (workaround for a bubbletea v1 mouse-restore bug).
- **Layout** is computed manually in `updateLayout()` (root model) by subtracting component heights from the window height — adjust there when adding/resizing components.
- **Persistence**: command history is the only persisted config, stored at `~/.config/teaterm/cmdhistroy.conf` (sic), capped at 500 entries, written on quit via `StoreConfig`.
- **Version** (`internal/version.go`) must stay a `var` with a string-literal initializer so `-ldflags -X` can override it; otherwise it resolves from build info.
