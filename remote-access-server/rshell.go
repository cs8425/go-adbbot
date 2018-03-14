// go build -o rshell rshell.go
package main

import (
	"io"
	"net"
	"os"

	"log"
	"flag"

	"../adbbot"
)

var daemonAddr = flag.String("t", "127.0.0.1:6900", "")

var compress = flag.Bool("comp", false, "compress connection")

var reflash = flag.Int("r", 1000, "update screen minimum time (ms)")

var verbosity = flag.Int("v", 3, "verbosity")

func main() {
	log.SetFlags(log.Ldate|log.Ltime)
	flag.Parse()


	conn, err := net.Dial("tcp", *daemonAddr)
	if err != nil {
		Vln(1, "error connct to", *daemonAddr)
		return
	}
	Vln(1, "connct", *daemonAddr, "ok!")

	if *compress {
		//conn = adbbot.NewFlateStream(conn, 1)
		conn = adbbot.NewCompStream(conn, 1)
	}

	err = adbbot.WriteVLen(conn, adbbot.OP_SHELL)
	if err != nil {
		Vln(2, "[shell]err", err)
		return
	}

	Cp3(os.Stdin, conn, os.Stdout)
	
}

// p1 >> p0 >> p2
func Cp3(p1 io.Reader, p0 net.Conn, p2 io.Writer) {
	p1die := make(chan struct{})
	go func() {
		io.Copy(p0, p1) // p0 << p1
		close(p1die)
	}()

	p2die := make(chan struct{})
	go func() {
		io.Copy(p2, p0) // p2 << p0
		close(p2die)
	}()

	// wait for tunnel termination
	select {
	case <-p1die:
	case <-p2die:
	}
}

func Vln(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Println(v...)
	}
}


