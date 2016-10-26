package tcpserver

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

//-----------------------------------------------------------------------------
// example usage - echo server

func TestEchoServer(t *testing.T) {
	srv, err := New(portString, newEchoAgent, 100)
	if err != nil {
		t.Error(err)
		return
	}
	err = srv.Start()
	if err != nil {
		t.Error(err)
		return
	}

	for i := 1; i <= 500; i++ {
		i := i
		wg.Add(1)
		go echoClient(t, i*73)
		time.Sleep(time.Millisecond)
	}

	wg.Wait()

	srv.Stop()
	waitSrv := make(chan struct{})
	go func() {
		srv.Wait()
		close(waitSrv)
	}()
	select {
	case <-waitSrv:
	case <-time.After(time.Second * 3):
	}
}

//-----------------------------------------------------------------------------
// our echo agent factory

func newEchoAgent(conn net.Conn, reader *bufio.Reader, writer *bufio.Writer, quit chan struct{}) Agent {
	return &echoAgent{conn, reader, writer, quit}
}

//-----------------------------------------------------------------------------
// our echo agent

type echoAgent struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	quit   chan struct{}
}

func (x *echoAgent) Proceed() error {
	x.conn.SetDeadline(time.Now().Add(time.Second * 3))
	select {
	case <-x.quit:
		return Error(`quit`)
	default:
	}
	line, err := x.reader.ReadBytes('\n')
	if err != nil {
		if err != io.EOF {
			log.Println(`error:`, err)
		}
		return err
	}
	_, err = x.writer.Write(line)
	if err != nil {
		if err != io.EOF {
			log.Println(`error:`, err)
		}
		return err
	}
	err = x.writer.Flush()
	if err != nil {
		if err != io.EOF {
			log.Println(`error:`, err)
		}
		return err
	}

	return nil
}

//-----------------------------------------------------------------------------
// our echo client

func echoClient(t *testing.T, seed int) {
	defer wg.Done()
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost%s", portString))
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	c := seed
	for c < 10 {
		msg := fmt.Sprintf("%d\n", c)
		writer.Write([]byte(msg))
		writer.Flush()

		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			t.Error(err)
			return
		}
		if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
			t.Error(err)
			return
		}

		if string(line) != msg {
			t.Fatal()
		}

		select {
		case <-quit:
			return
		default:
		}

		c++
	}
}

//-----------------------------------------------------------------------------

var (
	quit       = make(chan struct{})
	wg         = &sync.WaitGroup{}
	portString = fmt.Sprintf(":%d", port)
)

const (
	port = 37073
)

//-----------------------------------------------------------------------------
