//Package tcpserver implements a tcp server with simple and clean API.
package tcpserver

import (
	"bufio"
	"log"
	"net"
	"runtime"
	"sync"
)

//-----------------------------------------------------------------------------

// Processor is where our protocol is implemented (Thanks to William @goinggodotnet for name suggestion)
type Processor interface {
	// Process will get called in a loop as long as no errors got returned
	Process() error
}

//-----------------------------------------------------------------------------

// Error constant error based on http://dave.cheney.net/2016/04/07/constant-errors
type Error string

func (e Error) Error() string { return string(e) }

//-----------------------------------------------------------------------------

// Errors
const (
	ErrNoProcessorFactory = Error(`NO PROCESSOR FACTORY PROVIDED`)
)

//-----------------------------------------------------------------------------

// TCPServer a tcp server
type TCPServer struct {
	address          string
	processorFactory func(net.Conn, *bufio.Reader, *bufio.Writer, chan struct{}) Processor
	quit             chan struct{}
	handlerGroup     *sync.WaitGroup
	acceptorCount    int
}

// Start starts the server
func (x *TCPServer) Start() error {
	loop, err := x.loopMaker()
	if err != nil {
		return err
	}

	go loop()

	return nil
}

// Stop stops the server
func (x *TCPServer) Stop() error {
	close(x.quit)
	return nil
}

// Wait waits for all handlers to exit
func (x *TCPServer) Wait() { x.handlerGroup.Wait() }

//-----------------------------------------------------------------------------

// New creates a new *TCPServer
func New(
	address string,
	processorFactory func(net.Conn, *bufio.Reader, *bufio.Writer, chan struct{}) Processor,
	acceptorCount ...int) (*TCPServer, error) {
	if processorFactory == nil {
		return nil, ErrNoProcessorFactory
	}

	result := new(TCPServer)
	result.address = address
	result.processorFactory = processorFactory
	result.quit = make(chan struct{})
	result.handlerGroup = &sync.WaitGroup{}

	if len(acceptorCount) > 0 {
		result.acceptorCount = acceptorCount[0]
	}
	if result.acceptorCount <= 0 {
		cpuNum := runtime.NumCPU()
		result.acceptorCount = cpuNum
	}

	return result, nil
}

//-----------------------------------------------------------------------------

// loopMaker returns a loop (should go routine) and probably an error
func (x *TCPServer) loopMaker() (func(), error) {
	listener, err := net.Listen("tcp", x.address)
	if err != nil {
		return nil, err
	}

	accepts := make(chan net.Conn, x.acceptorCount)

	for i := 0; i < x.acceptorCount; i++ {
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

//-----------------------------------------------------------------------------

func (x *TCPServer) acceptor(accepts chan net.Conn) {
	for conn := range accepts {
		x.handlerGroup.Add(1)
		go x.handler(conn)
	}
}

func (x *TCPServer) handler(conn net.Conn) {
	defer x.handlerGroup.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Println(`error:`, `handler parent go-routine`, r)
		}
	}()
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	processor := x.processorFactory(conn, reader, writer, x.quit)
	for err := processor.Process(); err == nil; {
		runtime.Gosched()
	}
}

//-----------------------------------------------------------------------------
