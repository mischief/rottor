//Usage (in torrc):
//   UseBridges 1
//   Bridge rottor X.X.X.X:YYYY
//   ClientTransportPlugin rottor exec rottor-client
// Because this transport doesn't do anything to the traffic, you can use any
// ordinary relay's ORPort in the Bridge line.

package main

import (
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

import "git.torproject.org/pluggable-transports/goptlib.git"
import "git.torproject.org/pluggable-transports/goptlib.git/socks"
import "github.com/mischief/rottor"

var ptInfo pt.ClientInfo

// When a connection handler starts, +1 is written to this channel; when it
// ends, -1 is written.
var handlerChan = make(chan int)

func copyLoop(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		rottor.FilterCopy(b, rottor.RotNEnc(1), a)
		wg.Done()
	}()
	go func() {
		rottor.FilterCopy(a, rottor.RotNDec(1), b)
		wg.Done()
	}()

	wg.Wait()
}

func handleConnection(local net.Conn) error {
	defer local.Close()

	handlerChan <- 1
	defer func() {
		handlerChan <- -1
	}()

	var remote net.Conn
	err := socks.AwaitSocks4aConnect(local.(*net.TCPConn), func(dest string) (*net.TCPAddr, error) {
		var err error
		// set remote in outer function environment
		remote, err = net.Dial("tcp", dest)
		if err != nil {
			return nil, err
		}
		return remote.RemoteAddr().(*net.TCPAddr), nil
	})
	if err != nil {
		return err
	}
	defer remote.Close()
	copyLoop(local, remote)

	return nil
}

func acceptLoop(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go handleConnection(conn)
	}
	return nil
}

func startListener(addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go acceptLoop(ln)
	return ln, nil
}

func main() {
	var err error

	ptInfo, err = pt.ClientSetup([]string{"rottor"})
	if err != nil {
		os.Exit(1)
	}

	listeners := make([]net.Listener, 0)
	for _, methodName := range ptInfo.MethodNames {
		ln, err := startListener("127.0.0.1:0")
		if err != nil {
			pt.CmethodError(methodName, err.Error())
			continue
		}
		pt.Cmethod(methodName, "socks4", ln.Addr())
		listeners = append(listeners, ln)
	}
	pt.CmethodsDone()

	var numHandlers int = 0
	var sig os.Signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// wait for first signal
	sig = nil
	for sig == nil {
		select {
		case n := <-handlerChan:
			numHandlers += n
		case sig = <-sigChan:
		}
	}
	for _, ln := range listeners {
		ln.Close()
	}

	if sig == syscall.SIGTERM {
		return
	}

	// wait for second signal or no more handlers
	sig = nil
	for sig == nil && numHandlers != 0 {
		select {
		case n := <-handlerChan:
			numHandlers += n
		case sig = <-sigChan:
		}
	}
}
