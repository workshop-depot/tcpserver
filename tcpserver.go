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

// Agent is a statefull agent that manages a client - where our protocol is implemented.
type Agent interface {
	// Proceed will get called in a loop as long as no errors got returned.
	// To stop must return an error - like ErrStop.
	Proceed() error
}

//-----------------------------------------------------------------------------

// Error constant error based on http://dave.cheney.net/2016/04/07/constant-errors
type Error string

func (e Error) Error() string { return string(e) }

//-----------------------------------------------------------------------------

// Errors
const (
	ErrNoAgentFactory = Error(`NO AGENT FACTORY PROVIDED`)
	ErrStop           = Error(`STOP`)
)

//-----------------------------------------------------------------------------

// TCPServer a tcp server
type TCPServer struct {
	address       string
	agentFactory  func(net.Conn, *bufio.Reader, *bufio.Writer, chan struct{}) Agent
	quit          chan struct{}
	handlerGroup  *sync.WaitGroup
	acceptorCount int
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
	agentFactory func(net.Conn, *bufio.Reader, *bufio.Writer, chan struct{}) Agent,
	acceptorCount ...int) (*TCPServer, error) {
	if agentFactory == nil {
		return nil, ErrNoAgentFactory
	}

	result := new(TCPServer)
	result.address = address
	result.agentFactory = agentFactory
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
	agent := x.agentFactory(conn, reader, writer, x.quit)
	var err error
	for err = agent.Proceed(); err == nil; err = agent.Proceed() {
		runtime.Gosched()
	}
}

//-----------------------------------------------------------------------------
