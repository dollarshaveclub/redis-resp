package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strconv"
	"time"
)

//go:generate stringer -type=RESPValueType

// RESPValueType defines the types of values that can be encoded in RESP
// Arrays are implemented as a slice of RESPValue
type RESPValueType int

const (
	ErrorString RESPValueType = iota
	SimpleString
	BulkString
	Integer
)

// RESPValue represents an abstract RESP-encoded value
type RESPValue struct {
	Type  RESPValueType
	Value []byte
}

// RESPHandler is a handler function that processes client commands
// The returned RESPValue is the response delivered to the client
// A non-nil error will cause the connection to terminate
type RESPHandler func([]RESPValue) ([]RESPValue, error)

// RedisRESPConnectionServer is an object that takes a net.Conn and interacts with the client
// via the RESP protocol
type RedisRESPConnectionServer struct {
	timeout time.Duration
	hm      map[string]RESPHandler
}

// NewRedisRESPConnectionServer returns a new RedisRESPConnectionServer using the handler map and
// client timeout value
func NewRedisRESPConnectionServer(hm map[string]RESPHandler, timeout time.Duration) *RedisRESPConnectionServer {
	// copy handler map to our own private copy to prevent race conditions if caller modifies the map
	hm2 := make(map[string]RESPHandler)
	for k, v := range hm {
		hm2[k] = v
	}
	return &RedisRESPConnectionServer{
		timeout: timeout,
		hm:      hm2,
	}
}

// NewConnection blocks processing commands on the supplied connection
func (cs *RedisRESPConnectionServer) NewConnection(conn net.Conn) error {
	var err error
	rs := respConnectionServer{
		conn:    conn,
		hm:      cs.hm,
		timeout: cs.timeout,
	}
	err = rs.setupScanner()
	if err != nil {
		return err
	}
	for {
		err = rs.scanBlock()
		if err != nil {
			return err
		}
		err = rs.command(rs.s.Bytes())
		if err != nil {
			return err
		}
	}
}

type respConnectionServer struct {
	conn    net.Conn
	timeout time.Duration
	hm      map[string]RESPHandler
	s       *bufio.Scanner
}

func (rs *respConnectionServer) setupScanner() error {
	if rs.conn == nil {
		return fmt.Errorf("connection is nil")
	}
	rs.s = bufio.NewScanner(bufio.NewReader(rs.conn))
	rs.s.Split(rs.scanCRLF)
	return nil
}

// process top-level command (must be an array)
func (rs *respConnectionServer) command(cmd []byte) error {
	vals, err := rs.getRESPValue(cmd)
	if err != nil {
		return err
	}
	for i, v := range vals {
		if v.Type != BulkString {
			return newRESPErr(fmt.Errorf("command must be an array of bulk strings: unexpected type: %v: %v", i, v.Type.String()), ProtocolErrorBadArray)
		}
	}
	cmdname := string(vals[0].Value)
	if handler, ok := rs.hm[cmdname]; ok {
		resp, err := handler(vals[1:len(vals)])
		if err != nil {
			return newRESPErr(err, HandlerError)
		}
		err = rs.writeResponse(resp)
		if err != nil {
			return newRESPErr(err, ClientErrorResponseFailure)
		}
		return nil
	}
	return rs.writeResponse([]RESPValue{
		RESPValue{
			Type:  ErrorString,
			Value: []byte("-Error not implemented"),
		},
	})
}

func (rs *respConnectionServer) writeResponse(rsl []RESPValue) error {
	for _, resp := range rsl {
		output := make([]byte, len(resp.Value)+3)
		switch resp.Type {
		case SimpleString:
			output[0] = '+'
			copy(output[1:len(output)-2], resp.Value)
		case ErrorString:
			output[0] = '-'
			copy(output[1:len(output)-2], resp.Value)
		case Integer:
			output[0] = ':'
			copy(output[1:len(output)-2], resp.Value)
		case BulkString:
			lenstr := strconv.Itoa(len(resp.Value))
			output = []byte("$" + lenstr + "\r\n")
			output = append(output, resp.Value...)
			output = append(output, '\r', '\n')
		default:
			return fmt.Errorf("bad type for response: %v", resp.Type)
		}
		output[len(output)-2] = '\r'
		output[len(output)-1] = '\n'
		_, err := rs.conn.Write(output)
		if err != nil {
			return fmt.Errorf("error writing output: %v", err)
		}
	}
	return nil
}

func (rs *respConnectionServer) readBulkString(n int) ([]byte, error) {
	if n == -1 {
		return []byte{}, nil
	}
	err := rs.scanBlock()
	if err != nil {
		return []byte{}, newRESPErr(err, ProtocolErrorBadCRLFBlock)
	}
	v := rs.s.Bytes()
	if len(v) != n {
		return []byte{}, newRESPErr(fmt.Errorf("mismatched bulk string length: %v (expected %v)", len(v), n), ProtocolErrorBadBulkString)
	}
	return v, nil
}

// getRESPValue assumes an input string stripped of CRLF terminator and returns one RESPValue
// for simple/bulk types and multiple RESPValues for an array
func (rs *respConnectionServer) getRESPValue(input []byte) ([]RESPValue, error) {
	output := make([]RESPValue, 1)
	switch input[0] {
	case '+':
		output[0] = RESPValue{
			Type:  SimpleString,
			Value: input[1:len(input)],
		}
	case '-':
		output[0] = RESPValue{
			Type:  ErrorString,
			Value: input[1:len(input)],
		}
	case ':':
		output[0] = RESPValue{
			Type:  Integer,
			Value: input[1:len(input)],
		}
	case '$':
		if len(input) < 2 {
			return nil, newRESPErr(fmt.Errorf("bulk string too short"), ProtocolErrorBadBulkString)
		}
		n, err := strconv.Atoi(string(input[1:len(input)]))
		if err != nil {
			return nil, newRESPErr(fmt.Errorf("error parsing bulk string length: %v", err), ProtocolErrorBadBulkString)
		}
		v, err := rs.readBulkString(n)
		if err != nil {
			return nil, newRESPErr(err, ProtocolErrorBadBulkString)
		}
		output[0] = RESPValue{
			Type:  BulkString,
			Value: v,
		}
	case '*':
		output = []RESPValue{}
		if len(input) < 2 {
			return nil, newRESPErr(fmt.Errorf("array definition too short"), ProtocolErrorBadArray)
		}
		n, err := strconv.Atoi(string(input[1:len(input)]))
		if err != nil {
			return nil, newRESPErr(fmt.Errorf("error parsing array length: %v", err), ProtocolErrorBadArray)
		}
		var v []RESPValue
		var in []byte
		for i := 0; i < n; i++ {
			err = rs.scanBlock()
			if err != nil {
				return nil, newRESPErr(fmt.Errorf("error reading array member: %v", err), ProtocolErrorBadArray)
			}
			in = rs.s.Bytes()
			if in[0] == '*' {
				return nil, newRESPErr(fmt.Errorf("nested arrays are not allowed"), ProtocolErrorBadArray)
			}
			v, err = rs.getRESPValue(in)
			if err != nil {
				return nil, newRESPErr(fmt.Errorf("error deserializing array member: %v", err), ProtocolErrorBadArray)
			}
			if len(v) != 1 {
				return nil, newRESPErr(fmt.Errorf("unexpected multiple values from single array member"), ProtocolErrorBadArray)
			}
			output = append(output, v[0])
		}
	default:
		return nil, newRESPErr(fmt.Errorf("unknown first byte: %v", input[0]), ProtocolErrorUnknownValue)
	}
	return output, nil
}

func (rs *respConnectionServer) scanCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	i := bytes.IndexByte(data, '\n')
	j := bytes.IndexByte(data, '\r')
	if i >= 0 && j >= 0 && i == (j+1) {
		// We have a full CRLF-terminated line.
		return i + 1, data[0:j], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func (rs *respConnectionServer) scanBlock() error {
	rs.conn.SetDeadline(time.Now().Add(rs.timeout))
	if ok := rs.s.Scan(); !ok || rs.s.Err() != nil {
		return rs.s.Err()
	}
	return nil
}

// PackArray takes a slice of RESPValues and serializes them into a single
// RESPValues of type Array suitable for a handler to return as a response to a client
func PackArray(vals []RESPValue) (*RESPValue, error) {
	return nil, nil
}
