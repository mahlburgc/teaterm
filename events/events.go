package events

// defines all shared event messages

// Indicates a message was sent to the serial port.
type SerialTxMsg string

// Indicates data was received from the serial port.
type SerialRxMsg string

// Indicates a command from the command history was selected.
type HistCmdSelected string

// Indicates a command from the command history was executed.
type HistCmdExecuted string
