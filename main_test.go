package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	listenAddr = "/tmp/redis-resp.sock"
	listenType = "unix"
)

func pingHandler([]RESPValue) ([]RESPValue, error) {
	return []RESPValue{
		RESPValue{
			Type:  SimpleString,
			Value: []byte("PONG"),
		},
	}, nil
}

func fooHandler(args []RESPValue) ([]RESPValue, error) {
	ss := []string{}
	for _, rv := range args {
		ss = append(ss, string(rv.Value))
	}
	return []RESPValue{
		RESPValue{
			Type:  BulkString,
			Value: []byte(strings.Join(ss, ";")),
		},
	}, nil
}

func runserver(hm map[string]RESPHandler, done <-chan struct{}, aborted chan struct{}) {
	if listenType == "unix" {
		os.Remove(listenAddr)
	}

	srv := NewRedisRESPConnectionServer(hm, 30*time.Second)

	go func() {
		defer close(aborted)
		ln, err := net.Listen(listenType, listenAddr)
		if err != nil {
			fmt.Printf("error listening: %v\n", err)
			return
		}
		for {
			select {
			case <-done:
				return
			default:
			}
			conn, err := ln.Accept()
			if err != nil {
				fmt.Printf("error accepting: %v\n", err)
				return
			}
			if conn == nil {
				fmt.Printf("conn is nil!\n")
				return
			}
			err = srv.NewConnection(conn)
			if err != nil {
				fmt.Printf("error in NewConnection: %v\n", err)
				return
			}
		}
	}()
}

func TestPing(t *testing.T) {
	hm := map[string]RESPHandler{
		"PING": pingHandler,
	}

	done := make(chan struct{})
	aborted := make(chan struct{})
	runserver(hm, done, aborted)
	defer close(done)
	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial(listenType, listenAddr)
	if err != nil {
		t.Fatalf("error connecting to socket: %v", err)
	}

	resp := make([]byte, 7)
	conn.Write([]byte("*1\r\n$4\r\nPING\r\n"))
	time.Sleep(10 * time.Millisecond)
	select {
	case <-aborted:
		t.Fatalf("server aborted")
	default:
	}
	n, err := conn.Read(resp)
	if err != nil {
		t.Fatalf("error reading response: %v", err)
	}
	if n != 7 {
		t.Errorf("unexpected read count: %v (expected 7)", n)
	}
	if string(resp) != "+PONG\r\n" {
		t.Errorf("incorrect response: %v", string(resp))
	}
}

func TestCommandWithArgs(t *testing.T) {
	hm := map[string]RESPHandler{
		"FOO": fooHandler,
	}

	done := make(chan struct{})
	aborted := make(chan struct{})
	runserver(hm, done, aborted)
	defer close(done)
	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial(listenType, listenAddr)
	if err != nil {
		t.Fatalf("error connecting to socket: %v", err)
	}

	resp := make([]byte, 21)
	conn.Write([]byte("*4\r\n$3\r\nFOO\r\n$4\r\ncats\r\n$4\r\ndogs\r\n$4\r\nfish\r\n"))
	time.Sleep(10 * time.Millisecond)
	select {
	case <-aborted:
		t.Fatalf("server aborted")
	default:
	}
	n, err := conn.Read(resp)
	if err != nil {
		t.Fatalf("error reading response: %v", err)
	}
	if n != 21 {
		t.Errorf("unexpected read count: %v (expected 7)", n)
	}
	if string(resp) != "$14\r\ncats;dogs;fish\r\n" {
		t.Errorf("incorrect response: %v", string(resp))
	}
}

func TestRESPValueSimpleString(t *testing.T) {
	conn := newmockconn()
	csrv := respConnectionServer{
		conn:    conn,
		timeout: 30 * time.Second,
		hm:      map[string]RESPHandler{},
	}
	err := csrv.setupScanner()
	if err != nil {
		t.Fatalf("error setting up scanner: %v", err)
	}
	raw := []byte("+REDIS IS COOL")
	vs, err := csrv.getRESPValue(raw)
	if err != nil {
		t.Errorf("error getting value: %v", err)
	}
	if len(vs) != 1 {
		t.Errorf("incorrect values length: %v", len(vs))
	}
	if vs[0].Type != SimpleString {
		t.Errorf("incorrect value type: %v", vs[0].Type.String())
	}
	if !bytes.Equal(vs[0].Value, []byte("REDIS IS COOL")) {
		t.Errorf("incorrect value: %v", string(vs[0].Value))
	}
}

func TestRESPValueErrorString(t *testing.T) {
	conn := newmockconn()
	csrv := respConnectionServer{
		conn:    conn,
		timeout: 30 * time.Second,
		hm:      map[string]RESPHandler{},
	}
	err := csrv.setupScanner()
	if err != nil {
		t.Fatalf("error setting up scanner: %v", err)
	}
	raw := []byte("-Error doing something")
	vs, err := csrv.getRESPValue(raw)
	if err != nil {
		t.Errorf("error getting value: %v", err)
	}
	if len(vs) != 1 {
		t.Errorf("incorrect values length: %v", len(vs))
	}
	v := vs[0]
	if v.Type != ErrorString {
		t.Errorf("incorrect value type: %v", v.Type.String())
	}
	if !bytes.Equal(v.Value, []byte("Error doing something")) {
		t.Errorf("incorrect value: %v", string(v.Value))
	}
}

func TestRESPValueInteger(t *testing.T) {
	conn := newmockconn()
	csrv := respConnectionServer{
		conn:    conn,
		timeout: 30 * time.Second,
		hm:      map[string]RESPHandler{},
	}
	err := csrv.setupScanner()
	if err != nil {
		t.Fatalf("error setting up scanner: %v", err)
	}
	raw := []byte(":65535")
	vs, err := csrv.getRESPValue(raw)
	if err != nil {
		t.Errorf("error getting value: %v", err)
	}
	if len(vs) != 1 {
		t.Errorf("incorrect values length: %v", len(vs))
	}
	v := vs[0]
	if v.Type != Integer {
		t.Errorf("incorrect value type: %v", v.Type.String())
	}
	n, err := strconv.Atoi(string(v.Value))
	if err != nil {
		t.Errorf("error parsing integer: %v", err)
	}
	if n != 65535 {
		t.Errorf("incorrect value: %v", string(v.Value))
	}
}

func TestRESPValueBulkString(t *testing.T) {
	conn := newmockconn()
	csrv := respConnectionServer{
		conn:    conn,
		timeout: 30 * time.Second,
		hm:      map[string]RESPHandler{},
	}
	err := csrv.setupScanner()
	if err != nil {
		t.Fatalf("error setting up scanner: %v", err)
	}
	_, err = conn.Write([]byte("$21\r\nThis is a bulk string\r\n"))
	if err != nil {
		t.Errorf("error writing to mock conn: %v", err)
	}
	err = csrv.scanBlock()
	if err != nil {
		t.Errorf("scanBlock error: %v", err)
	}
	vs, err := csrv.getRESPValue(csrv.s.Bytes())
	if err != nil {
		t.Errorf("error getting value: %v", err)
	}
	if len(vs) != 1 {
		t.Errorf("incorrect values length: %v", len(vs))
	}
	v := vs[0]
	if v.Type != BulkString {
		t.Errorf("incorrect value type: %v", v.Type.String())
	}
	if !bytes.Equal(v.Value, []byte("This is a bulk string")) {
		t.Errorf("incorrect value: %v", string(v.Value))
	}
}

func TestRESPValueNullBulkString(t *testing.T) {
	conn := newmockconn()
	csrv := respConnectionServer{
		conn:    conn,
		timeout: 30 * time.Second,
		hm:      map[string]RESPHandler{},
	}
	err := csrv.setupScanner()
	if err != nil {
		t.Fatalf("error setting up scanner: %v", err)
	}
	_, err = conn.Write([]byte("$-1\r\n"))
	if err != nil {
		t.Errorf("error writing to mock conn: %v", err)
	}
	err = csrv.scanBlock()
	if err != nil {
		t.Errorf("scanBlock error: %v", err)
	}
	vs, err := csrv.getRESPValue(csrv.s.Bytes())
	if err != nil {
		t.Errorf("error getting value: %v", err)
	}
	if len(vs) != 1 {
		t.Errorf("incorrect values length: %v", len(vs))
	}
	v := vs[0]
	if v.Type != BulkString {
		t.Errorf("incorrect value type: %v", v.Type.String())
	}
	if !bytes.Equal(v.Value, []byte{}) {
		t.Errorf("incorrect value: %v", string(v.Value))
	}
}

func TestRESPValueArray(t *testing.T) {
	conn := newmockconn()
	csrv := respConnectionServer{
		conn:    conn,
		timeout: 30 * time.Second,
		hm:      map[string]RESPHandler{},
	}
	err := csrv.setupScanner()
	if err != nil {
		t.Fatalf("error setting up scanner: %v", err)
	}
	_, err = conn.Write([]byte("*4\r\n:365\r\n+simple string\r\n-error string\r\n$11\r\nbulk string\r\n"))
	if err != nil {
		t.Errorf("error writing to mock conn: %v", err)
	}
	err = csrv.scanBlock()
	if err != nil {
		t.Errorf("scanBlock error: %v", err)
	}
	vs, err := csrv.getRESPValue(csrv.s.Bytes())
	if err != nil {
		t.Errorf("error getting value: %v", err)
	}
	if len(vs) != 4 {
		t.Errorf("incorrect values length: %v", len(vs))
	}
	if vs[0].Type != Integer {
		t.Errorf("incorrect value type 1: %v", vs[0].Type.String())
	}
	n, err := strconv.Atoi(string(vs[0].Value))
	if err != nil {
		t.Errorf("error parsing integer: %v", err)
	}
	if n != 365 {
		t.Errorf("incorrect value 1: %v", string(vs[0].Value))
	}
	if vs[1].Type != SimpleString {
		t.Errorf("incorrect value type 2: %v", vs[1].Type.String())
	}
	if !bytes.Equal(vs[1].Value, []byte("simple string")) {
		t.Errorf("incorrect value 2: %v", string(vs[1].Value))
	}
	if vs[2].Type != ErrorString {
		t.Errorf("incorrect value type 3: %v", vs[2].Type.String())
	}
	if !bytes.Equal(vs[2].Value, []byte("error string")) {
		t.Errorf("incorrect value 3: %v", string(vs[2].Value))
	}
	if vs[3].Type != BulkString {
		t.Errorf("incorrect value type 4: %v", vs[3].Type.String())
	}
	if !bytes.Equal(vs[3].Value, []byte("bulk string")) {
		t.Errorf("incorrect value 4: %v", string(vs[3].Value))
	}
}
