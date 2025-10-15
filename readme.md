# teaterm

## Introduction

Teaterm is a simple serial device tool TUI written in GO using the bubbletea framework. It is inspired by tio with a focus on a nice and easy to use user interface.

This version is very alpha! ;)

## Installation

GO

```shell
go install github.com/mahlburgc/teaterm@latest
```

## Planned features
- easy connection to serial devices
- configurable port settings
- configurable line ending detection
- stable connection with automatic reconnect
- TUI with seperated input and output windows
- status bar with current config and status
- store command history between sessions
- create predefined commands for faster communication with serial CLIs
- create different profiles with separate settings, command histories and predefined commands
- mouse support to easily send predefined commands
- auto completion suggestions for send commands based on command history
- timestamp
- log to file

## Next steps
- open editor with message content
- enable mouse scrolling
- enable mouse selection for commands
    - use bubblezone for that https://github.com/lrstanley/bubblezone
- use list bubble for command history
    - make use of fuzzy finding
- add scroll bar
- only auto scroll on new messages if we are at lowest line
- ctrl + page up / down for faster scrolling


## Useful Resources
- https://leg100.github.io/en/posts/building-bubbletea-programs/
- https://github.com/lrstanley/bubblezone
- https://github.com/charmbracelet/bubbletea
- https://github.com/charmbracelet/vhs?tab=readme-ov-file
- https://github.com/spf13/cobra
- https://github.com/charm-and-friends/additional-bubbles?tab=readme-ov-file#additional-bubbles