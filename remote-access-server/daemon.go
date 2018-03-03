// go build -o daemon daemon.go packet.go framebuffer.go
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

	"fmt"

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

	m := NewMonkey(bot, 1080)
	defer m.Close()

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
		go handleConn(conn, bot, &screen, m)
	}

}

var newclients chan io.ReadWriteCloser
func handleConn(p1 net.Conn, bot *adbbot.LocalBot, screen *[]byte, m *Monkey) {
	evmap := map[int64]string{
		-1: "up",
		0: "move",
		1: "down",
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
			evstr, ok := evmap[ev]
			if !ok {
				Vln(2, "[todo][Touch][EvCode]err", ev)
				return
			}
			Vln(3, "[Touch]", x, y, evstr)
			m.Touch(image.Pt(x, y), evstr)

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

			evstr, ok := evmap[ev]
			if !ok {
				Vln(2, "[todo][Key][EvCode]err", ev)
				return
			}
			Vln(3, "[Key]", evstr)

			keycode, ok := keymap[op]
			if !ok {
				Vln(2, "[todo][Key][Code]err", op)
				return
			}
			m.Key(keycode, evstr)

		case "ScreenSize":
			WriteVLen(p1, int64(bot.Screen.Dx()))
			WriteVLen(p1, int64(bot.Screen.Dy()))
		case "Screencap":
			//bot.Screencap()
			//bot.Last_screencap
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

		encoder.Encode(&buf, bot.Last_screencap)
//		encoder.Encode(&buf, fb)
		*screen = buf.Bytes()
		buf.Reset()

/*		rawimg, ok := bot.Last_screencap.(*image.NRGBA)
		if !ok {
			encoder.Encode(&buf, bot.Last_screencap)
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

type Monkey struct {
	Port    int
	conn	net.Conn
}

func NewMonkey(b adbbot.Bot, port int) (*Monkey) {

	forwardCmd := fmt.Sprintf("forward tcp:%d tcp:%d", port, port)
	b.Adb(forwardCmd)

	monkeyCmd := fmt.Sprintf("monkey --port %d", port)
	go b.Shell(monkeyCmd) // in background


	addr := fmt.Sprintf("127.0.0.1:%d", port)

TRYCONN:
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		Vln(3, "[monkey][conn]err", err)
		time.Sleep(1000 * time.Millisecond)
		goto TRYCONN
	}
	m := Monkey{
		Port: port,
		conn: conn,
	}

	return &m
}

func (m *Monkey) Close() (err error){
	m.conn.Write([]byte("done"))
	return m.conn.Close()
}

func (m *Monkey) send(cmd string) (err error){
	_, err = m.conn.Write([]byte(cmd))
/*	if err != nil {
		return
	}
	buf := make([]byte, 2)
	n, err = m.conn.Read(buf)
*/
	return
}

func (m *Monkey) Tap(loc image.Point) (err error){
	str := fmt.Sprintf("tap %d %d\n", loc.X, loc.Y)
	err = m.send(str)
	return
}

func (m *Monkey) Text(in string) (err error){
	str := fmt.Sprintf("type %s\n", in)
	err = m.send(str)
	return
}

func (m *Monkey) Press(in string) (err error){
	str := fmt.Sprintf("press %s\n", in)
	err = m.send(str)
	return
}

func (m *Monkey) Key(in string, ty string) (err error){
	str := fmt.Sprintf("key %s %s\n", ty, in)
	err = m.send(str)
	return
}

func (m *Monkey) Touch(loc image.Point, ty string) (err error){
	str := fmt.Sprintf("touch %s %d %d\n", ty, loc.X, loc.Y)
	err = m.send(str)
	return
}

