// Code generated by "stringer -type=RESPValueType"; DO NOT EDIT

package main

import "fmt"

const _RESPValueType_name = "ErrorStringSimpleStringBulkStringIntegerArray"

var _RESPValueType_index = [...]uint8{0, 11, 23, 33, 40, 45}

func (i RESPValueType) String() string {
	if i < 0 || i >= RESPValueType(len(_RESPValueType_index)-1) {
		return fmt.Sprintf("RESPValueType(%d)", i)
	}
	return _RESPValueType_name[_RESPValueType_index[i]:_RESPValueType_index[i+1]]
}