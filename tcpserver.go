//Package tcpserver implements a tcp server with simple and clean API.
//
//Copyright 2016 Kaveh Shahbazian (See LICENSE file)
package tcpserver

import (
	"bufio"
	"log"
	"net"
	"runtime"
	"sync"
)

//-----------------------------------------------------------------------------

// Handler handles
type Handler interface {
	Handle()
}

// Server /
type Server interface {
	Start() error
	Stop() error
	Wait()
	// Handler factory
	Make(net.Conn, *bufio.Reader, *bufio.Writer, chan struct{}) Handler
}

//-----------------------------------------------------------------------------

// Error constant error based on http://dave.cheney.net/2016/04/07/constant-errors
type Error string

func (e Error) Error() string { return string(e) }

//-----------------------------------------------------------------------------

// Errors
const (
	ErrNoHandlerFactory = Error(`NO HANDLER FACTORY PROVIDED`)
)

//-----------------------------------------------------------------------------

// tcpServer a tcp server - ? acceptor count
type tcpServer struct {
	address        string
	handlerFactory func(net.Conn, *bufio.Reader, *bufio.Writer, chan struct{}) Handler
	quit           chan struct{}
	handlerGroup   *sync.WaitGroup
}

// Start implements Server
func (x *tcpServer) Start() error {
	loop, err := x.loopMaker()
	if err != nil {
		return err
	}

	go loop()

	return nil
}

// Stop implements Server
func (x *tcpServer) Stop() error {
	close(x.quit)
	return nil
}

// Wait implements Server
func (x *tcpServer) Wait() { x.handlerGroup.Wait() }

// Make implements Server
func (x *tcpServer) Make(conn net.Conn, reader *bufio.Reader, writer *bufio.Writer, quit chan struct{}) Handler {
	return x.handlerFactory(conn, reader, writer, quit)
}

//-----------------------------------------------------------------------------

// New /
func New(address string, handlerFactory func(net.Conn, *bufio.Reader, *bufio.Writer, chan struct{}) Handler) (Server, error) {
	if handlerFactory == nil {
		return nil, ErrNoHandlerFactory
	}

	result := new(tcpServer)
	result.address = address
	result.handlerFactory = handlerFactory
	result.quit = make(chan struct{})
	result.handlerGroup = &sync.WaitGroup{}

	return result, nil
}

//-----------------------------------------------------------------------------

// loopMaker returns a loop (should go routine) and probably an error
func (x *tcpServer) loopMaker() (func(), error) {
	listener, err := net.Listen("tcp", x.address)
	if err != nil {
		return nil, err
	}

	cpuNum := runtime.NumCPU()
	acceptorCount := cpuNum
	if acceptorCount <= 0 {
		acceptorCount = 1
	}
	accepts := make(chan net.Conn, acceptorCount)

	for i := 0; i < acceptorCount; i++ {
		go x.acceptor(accepts)
	}

	loop := func() {
		defer listener.Close()

		for {
			conn, errConn := listener.Accept()
			if errConn != nil {
				runtime.Gosched()
				continue
			}

			select {
			case <-x.quit:
				close(accepts)
				return
			case accepts <- conn:
			}
		}
	}

	return loop, nil
}

func (x *tcpServer) acceptor(accepts chan net.Conn) {
	for conn := range accepts {
		x.handlerGroup.Add(1)
		go x.handler(conn)
	}
}

func (x *tcpServer) handler(conn net.Conn) {
	defer x.handlerGroup.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Println(`error:`, `handler parent go-routine`, r)
		}
	}()
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	h := x.Make(conn, reader, writer, x.quit)
	h.Handle()
}

//-----------------------------------------------------------------------------
