// go build -o daemon daemon.go packet.go
package main

import (
	"net"
	"flag"
	"log"
	"runtime"
	"time"

	"io"
	"bytes"
	"image"
	"image/png"

	"../adbbot"
)

var verbosity = flag.Int("v", 2, "verbosity")
var ADB = flag.String("adb", "adb", "adb exec path")
var DEV = flag.String("dev", "", "select device")

var OnDevice = flag.Bool("od", false, "run on device")

var bindAddr = flag.String("l", ":6900", "")

var reflash = flag.Int("r", 1000, "update screen minimum time (ms)")

func main() {

	log.SetFlags(log.Ldate|log.Ltime)
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	adbbot.Verbosity = *verbosity
	bot := adbbot.NewBot(*DEV, *ADB)

	// run on android by adb user(shell)
	bot.IsOnDevice = *OnDevice

	Vln(2, "[adb]", "wait-for-device")
	_, err := bot.Adb("wait-for-device")
	if err != nil {
		Vln(1, "[adb] err", err)
		return
	}

	screen := make([]byte, 0)
	go screencap(bot, &screen)

	ln, err := net.Listen("tcp", *bindAddr)
	if err != nil {
		Vln(2, "[server]Error listening:", err)
		return
	}
	defer ln.Close()
	Vln(1, "daemon start at", *bindAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleConn(conn, bot, &screen)
	}

}

var newclients chan io.ReadWriteCloser
func handleConn(p1 net.Conn, bot *adbbot.Bot, screen *[]byte) {
	for {
		todo, err := ReadTagStr(p1)
		if err != nil {
			Vln(2, "[todo]err", err)
			return
		}
		Vln(3, "[todo]", todo)

		switch todo {
		case "Click":
			x, y, err := readXY(p1)
			if err != nil {
				Vln(2, "[todo][Click]err", err)
				return
			}
				Vln(2, "[Click]", x, y)
			bot.Click(image.Pt(x, y), false)
		case "Swipe":
			x0, y0, err := readXY(p1)
			if err != nil {
				Vln(2, "[todo][Swipe]err1", err)
				return
			}

			x1, y1, err := readXY(p1)
			if err != nil {
				Vln(2, "[todo][Swipe]err2", err)
				return
			}

			dt, err := ReadVLen(p1)
			if err != nil {
				Vln(2, "[dt]err", err)
				return
			}

			Vln(2, "[Swipe]", dt, x0, y0, ">>", x1, y1)
			bot.SwipeT(image.Pt(x0, y0), image.Pt(x1, y1), int(dt), false)
		case "Key":
			op, err := ReadTagStr(p1)
			if err != nil {
				Vln(2, "[todo][Key]err", err)
				return
			}
			switch op {
			case "home":
				bot.KeyHome()
			case "back":
				bot.KeyBack()
			case "task":
				bot.KeySwitch()
			case "power":
				bot.KeyPower()
			}
		case "ScreenSize":
			WriteVLen(p1, int64(bot.Screen.Dx()))
			WriteVLen(p1, int64(bot.Screen.Dy()))
		case "Screencap":
			//bot.Screencap()
			//bot.Last_screencap
			WriteVTagByte(p1, *screen)
		case "poll":
			Vln(2, "[todo][poll]", p1)
			conn := NewCompStream(p1, 1)
//			conn := NewFlateStream(p1, 1)
			newclients <- conn
		}
	}
}

func readXY(p1 net.Conn) (x, y int, err error) {
	var x0, y0 int64
	x0, err = ReadVLen(p1)
	if err != nil {
		Vln(2, "[x]err", err)
		return
	}
	y0, err = ReadVLen(p1)
	if err != nil {
		Vln(2, "[y]err", err)
		return
	}
	return int(x0), int(y0), nil
}

func screencap(bot *adbbot.Bot, screen *[]byte) {
	var buf bytes.Buffer

	newclients = make(chan io.ReadWriteCloser, 16)
	clients := make(map[io.ReadWriteCloser]io.ReadWriteCloser, 0)

	encoder := png.Encoder{
//		CompressionLevel: png.BestSpeed,
		CompressionLevel: png.NoCompression,
	}

	limit := time.Duration(*reflash) * time.Millisecond

	for {
		start := time.Now()
		_, err := bot.Screencap()
		if err != nil {
			return
		}
		Vln(2, "poll screen ok", time.Since(start))

		encoder.Encode(&buf, bot.Last_screencap)
		*screen = buf.Bytes()
		buf.Reset()

/*		rawimg, ok := bot.Last_screencap.(*image.NRGBA)
		if !ok {
			encoder.Encode(&buf, bot.Last_screencap)
			*screen = buf.Bytes()
			buf.Reset()
		}
		*screen = []byte(rawimg.Pix)*/

		Vln(2, "encode screen ok", len(*screen), time.Since(start))

		for i, c := range clients {
			err := WriteVTagByte(c, *screen)
			if err != nil {
				Vln(2, "write:", err)
				c.Close()
				delete(clients, i)
			}
		}
		for len(newclients) > 0 {
			client := <- newclients
			clients[client] = client
		}

		if time.Since(start) < limit {
			time.Sleep(limit - time.Since(start))
		}
	}
}

func Vln(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Println(v...)
	}
}

