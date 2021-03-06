// Code generated by "stringer -type=RESPErrorType"; DO NOT EDIT

package main

import "fmt"

const _RESPErrorType_name = "UnknownProtocolErrorNotAnArrayProtocolErrorBadArrayProtocolErrorBadBulkStringProtocolErrorBadCRLFBlockProtocolErrorUnknownValueClientErrorUnexpectedArrayClientErrorResponseFailureHandlerError"

var _RESPErrorType_index = [...]uint8{0, 7, 30, 51, 77, 102, 127, 153, 179, 191}

func (i RESPErrorType) String() string {
	if i < 0 || i >= RESPErrorType(len(_RESPErrorType_index)-1) {
		return fmt.Sprintf("RESPErrorType(%d)", i)
	}
	return _RESPErrorType_name[_RESPErrorType_index[i]:_RESPErrorType_index[i+1]]
}
