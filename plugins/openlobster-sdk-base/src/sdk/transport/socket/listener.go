// Copyright (c) OpenLobster contributors. See LICENSE for details.

package socket

import (
	"fmt"
	"net"
	"sync"
)

// SingleConnListener wraps an already-connected net.Conn as a net.Listener.
// It accepts exactly one connection, then returns errors on subsequent Accept calls.
type SingleConnListener struct {
	conn net.Conn
	once sync.Once
	ch   chan net.Conn
}

func NewSingleConnListener(conn net.Conn) *SingleConnListener {
	ch := make(chan net.Conn, 1)
	if conn != nil {
		ch <- conn
	}
	return &SingleConnListener{conn: conn, ch: ch}
}

func (l *SingleConnListener) Accept() (net.Conn, error) {
	if l == nil {
		return nil, fmt.Errorf("socket: listener is nil")
	}
	conn, ok := <-l.ch
	if !ok || conn == nil {
		return nil, net.ErrClosed
	}
	return conn, nil
}

func (l *SingleConnListener) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		close(l.ch)
	})
	if l.conn != nil {
		return l.conn.Close()
	}
	return nil
}

func (l *SingleConnListener) Addr() net.Addr {
	if l != nil && l.conn != nil {
		return l.conn.LocalAddr()
	}
	return dummyAddr("socketpair")
}

type dummyAddr string

func (d dummyAddr) Network() string { return "unix" }
func (d dummyAddr) String() string  { return string(d) }
