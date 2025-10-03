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

## Useful Resources
- https://leg100.github.io/en/posts/building-bubbletea-programs/
- https://github.com/lrstanley/bubblezone
- https://github.com/charmbracelet/bubbletea