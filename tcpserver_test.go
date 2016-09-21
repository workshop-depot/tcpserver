//Copyright 2016 Kaveh Shahbazian (See LICENSE file)

package tcpserver

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDummy2(t *testing.T) {
	quit := make(chan struct{})
	clientGroup := &sync.WaitGroup{}

	var n int64
	h := func(code EventCode, payload []byte, err error) (resp []byte, rerr error) {
		if code == Received && len(payload) > 0 {
			n, _ = strconv.ParseInt(strings.Trim(string(payload), "\n"), 10, 64)
			n++
			resp = []byte(fmt.Sprintf("%d\n", n))
		}

		return
	}

	srv, err := Init(Conf{Address: portString, AcceptorCount: 2, EventHandler: h, QuitSignal: quit, ClientTimeout: time.Millisecond * 1200, ClientGroup: clientGroup})
	if err != nil {
		t.Error(err)
	}
	_ = srv
	defer close(quit)

	done := make(chan struct{})
	go testClient(t, done)
	<-done

	if n != 11 {
		t.Log(n)
		t.Fail()
	}
}

var (
	portString = fmt.Sprintf(":%d", port)
)

const (
	port = 37073
)

func testClient(t *testing.T, done chan struct{}) {
	defer close(done)

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost%s", portString))
	if err != nil {
		t.Error(err)
		t.Fatal("Failed to connect to test server")
		t.Fail()
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	var counter int64
	for counter < 11 {
		msg := fmt.Sprintf("%d\n", counter)
		writer.Write([]byte(msg))
		writer.Flush()

		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			return
		}
		if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
			return
		}

		counter, _ = strconv.ParseInt(strings.Trim(string(line), "\n"), 10, 64)
		counter++
	}
}
