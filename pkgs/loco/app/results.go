package app

// CVRead holds the result of reading one configuration variable.
type CVRead struct {
	Number uint16
	Value  int
	Err    error
}

// CVBitWriteResult describes one read-modify-write on a CV bit field.
type CVBitWriteResult struct {
	Number uint16
	Before int
	After  int
}
