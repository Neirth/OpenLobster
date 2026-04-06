// Copyright (c) OpenLobster contributors. See LICENSE for details.

package subprocess

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"syscall"
)

func openSocketpair() (net.Conn, *os.File, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("socketpair: %w", err)
	}

	hostFile := os.NewFile(uintptr(fds[0]), "plugin-host")
	childFile := os.NewFile(uintptr(fds[1]), "plugin-child")
	hostConn, err := net.FileConn(hostFile)
	_ = hostFile.Close()
	if err != nil {
		_ = childFile.Close()
		return nil, nil, fmt.Errorf("socketpair host conn: %w", err)
	}
	return hostConn, childFile, nil
}

type singleConnDialer struct {
	once sync.Once
	conn net.Conn
	err  error
}

func newSingleConnDialer(conn net.Conn) *singleConnDialer {
	return &singleConnDialer{conn: conn}
}

func (d *singleConnDialer) DialContext(context.Context, string) (net.Conn, error) {
	var out net.Conn
	d.once.Do(func() {
		out = d.conn
		d.conn = nil
	})
	if out == nil {
		if d.err != nil {
			return nil, d.err
		}
		return nil, fmt.Errorf("single-conn dialer already used")
	}
	return out, nil
}
