//Package tcpserver implements a tcp server with simple and clean API.
//
//Copyright 2016 Kaveh Shahbazian (See LICENSE file)
package tcpserver

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"
)

//-----------------------------------------------------------------------------
// definitions

//Error constant error based on http://dave.cheney.net/2016/04/07/constant-errors
type Error string

func (e Error) Error() string { return string(e) }

//EventHandler gets called on each event
//	myEventHandler(
//		event_code,
//		payload, //received or sent
//		error) (payload_to_send, error)
type EventHandler func(EventCode, []byte, error) ([]byte, error)

const (
	//None +
	None EventCode = iota
	//Connected +
	Connected
	//Closed +
	Closed
	//Received +
	Received
	//Sent +
	Sent
)

//EventCode +
type EventCode int

func (c EventCode) String() string {
	switch c {
	case Connected:
		return "connected"
	case Closed:
		return "closed"
	case Received:
		return "received"
	case Sent:
		return "sent"
	case None:
		return "none"
	}

	panic(`not a valid EventCode`)
}

// Errors
const (
	NoHandlerErr    = Error(`NO HANDLER PROVIDED`)
	NoQuitSignalErr = Error(`NO QUIT SIGNAL PROVIDED`)

	//ErrClose return this from handler, means force close connection
	ErrClose = Error("CLOSE")
)

//-----------------------------------------------------------------------------
// server definition

// Init inits a tcp server
func Init(conf Conf) (*Server, error) {
	if conf.AcceptorCount <= 0 {
		cpuNum := runtime.NumCPU()
		if cpuNum < 0 {
			cpuNum = 1
		}
		conf.AcceptorCount = cpuNum
	}

	if conf.EventHandler == nil {
		return nil, NoHandlerErr
	}

	if conf.ClientTimeout <= 0 {
		conf.ClientTimeout = time.Second * 30
	}

	if conf.ClientGroup == nil {
		conf.ClientGroup = &sync.WaitGroup{}
	}

	result := new(Server)
	result.conf = conf

	loop, err := result.loopMaker()
	if err != nil {
		return nil, err
	}

	go loop()

	return result, nil
}

// Server a tcp server
type Server struct {
	conf Conf
}

// Conf tcp server conf
type Conf struct {
	Address       string
	AcceptorCount int
	EventHandler  EventHandler
	QuitSignal    <-chan struct{}
	ClientTimeout time.Duration
	ClientGroup   *sync.WaitGroup
}

//-----------------------------------------------------------------------------
// server methods

// loopMaker returns a loop (should go routine) and probably an error
func (srv *Server) loopMaker() (func(), error) {
	conf := &srv.conf

	listener, err := net.Listen("tcp", conf.Address)
	if err != nil {
		return nil, err
	}

	accepts := make(chan net.Conn, conf.AcceptorCount)

	for i := 0; i < conf.AcceptorCount; i++ {
		go srv.acceptor(accepts)
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
			case <-conf.QuitSignal:
				close(accepts)
				return
			case accepts <- conn:
			}
		}
	}

	return loop, nil
}

func (srv *Server) acceptor(accepts chan net.Conn) {
	for conn := range accepts {
		srv.conf.ClientGroup.Add(1)
		go srv.handler(conn)
	}
}

func (srv *Server) handler(conn net.Conn) {
	conf := &srv.conf
	defer func() {
		conf.ClientGroup.Done()
	}()

	defer func() {
		if r := recover(); r != nil {
			conf.EventHandler(None, nil, Error(fmt.Sprintf("%v", r)))
		}
	}()
	defer srv.closeConnection(conn, nil)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	conf.EventHandler(Connected, nil, nil)

	if conf.ClientTimeout > 0 {
		conn.SetDeadline(time.Now().Add(conf.ClientTimeout))
	}
	for {
		select {
		case <-conf.QuitSignal:
			return
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			sending := true
			outgoingPayload, err := conf.EventHandler(Received, line, nil)
			if err != nil && err == ErrClose {
				return
			}
			select {
			case <-conf.QuitSignal:
				return
			default:
			}

			for sending {
				if outgoingPayload == nil || len(bytes.TrimSpace(outgoingPayload)) == 0 {
					sending = false
					break
				}

				_, sendErr := writer.Write(outgoingPayload)
				if sendErr != nil {
					conf.EventHandler(Sent, outgoingPayload, sendErr)
					return
				}

				sendErr = writer.Flush()
				if sendErr != nil {
					conf.EventHandler(Sent, outgoingPayload, sendErr)
					return
				}

				outgoingPayload, err = conf.EventHandler(Sent, outgoingPayload, nil)
				if err != nil && err == ErrClose {
					return
				}
			}
		}

		if conf.ClientTimeout > 0 {
			conn.SetDeadline(time.Now().Add(conf.ClientTimeout))
		}
	}
}

func (srv *Server) closeConnection(conn net.Conn, err error) {
	conn.Close()
	srv.conf.EventHandler(Closed, nil, err)
}

//-----------------------------------------------------------------------------
