package internal

type ErrMsg struct{ err error }

// For messages that contain errors it's often handy to also implement the
func (e ErrMsg) Error() string { return e.err.Error() }
