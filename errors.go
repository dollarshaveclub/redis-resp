package main

// RESPErrorType defines the type of errors that can occur
type RESPErrorType int

const (
	Unknown RESPErrorType = iota
	ProtocolErrorNotAnArray
	ProtocolErrorBadArray
	ProtocolErrorBadBulkString
	ProtocolErrorBadCRLFBlock
	ProtocolErrorUnknownValue
	ClientErrorUnexpectedArray
	ClientErrorResponseFailure
	HandlerError
)

type respErr struct {
	err error
	t   RESPErrorType
}

func (re respErr) Error() string {
	return re.err.Error()
}

func (re respErr) RESPErrorType() RESPErrorType {
	return re.t
}

func newRESPErr(err error, t RESPErrorType) error {
	return respErr{
		err: err,
		t:   t,
	}
}

type respError interface {
	RESPErrType() RESPErrorType
}

// GetErrorType returns the type for a given error value, if any
func GetErrorType(err error) RESPErrorType {
	switch v := err.(type) {
	case respError:
		return v.RESPErrType()
	default:
		return Unknown
	}
}
