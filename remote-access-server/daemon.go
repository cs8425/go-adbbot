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

//	"fmt"

	"../adbbot"
)

var verbosity = flag.Int("v", 3, "verbosity")
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
	bot := adbbot.NewLocalBot(*DEV, *ADB)

	// run on android by adb user(shell)
	bot.IsOnDevice = *OnDevice

	Vln(2, "[adb]", "wait-for-device")
	_, err := bot.Adb("wait-for-device")
	if err != nil {
		Vln(1, "[adb] err", err)
		return
	}

	m := adbbot.NewMonkey(bot, 1080)
	defer m.Close()
	bot.Input = m

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
func handleConn(p1 net.Conn, bot *adbbot.LocalBot, screen *[]byte) {
	evmap := map[int64]adbbot.KeyAction{
		-1: adbbot.KEY_UP,
		0: adbbot.KEY_MV,
		1: adbbot.KEY_DOWN,
	}

	keymap := map[string]string{
		"home": "KEYCODE_HOME",
		"back": "KEYCODE_BACK",
		"task": "KEYCODE_APP_SWITCH",
		"power": "KEYCODE_POWER",
	}

	for {
		todo, err := ReadTagStr(p1)
		if err != nil {
			Vln(2, "[todo]err", err)
			return
		}
		Vln(4, "[todo]", todo)

		switch todo {
		case "Touch":
			x, y, err := readXY(p1)
			if err != nil {
				Vln(2, "[todo][Touch]err", err)
				return
			}
			ev, err := ReadVLen(p1)
			if err != nil {
				Vln(2, "[todo][Touch][Ev]err", err)
				return
			}
			evcode, ok := evmap[ev]
			if !ok {
				Vln(2, "[todo][Touch][EvCode]err", ev)
				return
			}
			Vln(3, "[Touch]", x, y, evcode)
			bot.Touch(image.Pt(x, y), evcode)

		case "Key":
			op, err := ReadTagStr(p1)
			if err != nil {
				Vln(2, "[todo][Key]err", err)
				return
			}
			ev, err := ReadVLen(p1)
			if err != nil {
				Vln(2, "[todo][Key][Ev]err", err)
				return
			}

			evcode, ok := evmap[ev]
			if !ok {
				Vln(2, "[todo][Key][EvCode]err", ev)
				return
			}
			Vln(3, "[Key]", evcode)

			keycode, ok := keymap[op]
			if !ok {
				Vln(2, "[todo][Key][Code]err", op)
				return
			}
			bot.Key(keycode, evcode)

		case "ScreenSize":
			WriteVLen(p1, int64(bot.ScreenBounds.Dx()))
			WriteVLen(p1, int64(bot.ScreenBounds.Dy()))
		case "Screencap":
			//bot.Screencap()
			//bot.GetLastScreencap()
			WriteVTagByte(p1, *screen)
		case "poll":
			Vln(3, "[todo][poll]", p1)
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

func screencap(bot *adbbot.LocalBot, screen *[]byte) {
	var buf bytes.Buffer

	newclients = make(chan io.ReadWriteCloser, 16)
	clients := make(map[io.ReadWriteCloser]io.ReadWriteCloser, 0)

	encoder := png.Encoder{
//		CompressionLevel: png.BestSpeed,
		CompressionLevel: png.NoCompression,
	}

	limit := time.Duration(*reflash) * time.Millisecond

/*	fb, err := FBOpen("/dev/graphics/fb0")
	if err != nil {
		Vln(2, "[screen][framebuffer][err]", err)
		return
	}*/

	for {
		start := time.Now()
		_, err := bot.Screencap()
		if err != nil {
			return
		}
		Vln(4, "[screen][poll]", time.Since(start))

		encoder.Encode(&buf, bot.GetLastScreencap())
//		encoder.Encode(&buf, fb)
		*screen = buf.Bytes()
		buf.Reset()

/*		rawimg, ok := bot.GetLastScreencap().(*image.NRGBA)
		if !ok {
			encoder.Encode(&buf, bot.GetLastScreencap())
			*screen = buf.Bytes()
			buf.Reset()
		}
		*screen = []byte(rawimg.Pix)*/

		Vln(4, "[screen][encode]", len(*screen), time.Since(start))

		for i, c := range clients {
			err := WriteVTagByte(c, *screen)
			if err != nil {
				Vln(2, "[screen][push][err]", err)
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

