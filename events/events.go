package events

// defines all shared event messages

// Indicates a message was sent to the serial port.
type SerialTxMsg string

// Indicates a message is typed into the input field.
type PartialTxMsg string

// Indicates a input is suggested.
type InputSuggestion string

// Indicates data was received from the serial port.
type SerialRxMsgReceived string

// Indicates a command from the command history was selected.
type HistCmdSelected string

// Indicates that a messages should be transmitted
type SendMsg struct {
	Data        string
	FromCmdHist bool
} // TODO find better naming

// Indicates that an error occured
type ErrMsg error

// Indicates an info that should be displayed
type InfoMsg string

// ConnectionStatus represents the state of the serial connection
type ConnectionStatus int

const (
	Disconnected ConnectionStatus = iota
	Connecting
	Connected
)

// ConnectionStatusMsg is sent when the connection state changes
type ConnectionStatusMsg struct {
	Status ConnectionStatus
}
