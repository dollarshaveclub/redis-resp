package main

import (
	"bytes"
	"net"
	"time"
)

type mockAddr struct{}

func (ma mockAddr) Network() string {
	return ""
}

func (ma mockAddr) String() string {
	return ""
}

type mockConn struct {
	b *bytes.Buffer
}

func (mc mockConn) Read(b []byte) (n int, err error) {
	return mc.b.Read(b)
}

func (mc mockConn) Write(b []byte) (n int, err error) {
	return mc.b.Write(b)
}

func (mc mockConn) Close() error {
	return nil
}

func (mc mockConn) LocalAddr() net.Addr {
	return mockAddr{}
}

func (mc mockConn) RemoteAddr() net.Addr {
	return mockAddr{}
}

func (mc mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (mc mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (mc mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func newmockconn() mockConn {
	return mockConn{
		b: bytes.NewBuffer([]byte{}),
	}
}
