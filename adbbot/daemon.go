package adbbot

import (
	"net"
	"time"

//	"io"
	"bytes"
	"image"
	"image/png"
)

type Daemon struct {
	Reflash     time.Duration

	ln          net.Listener
	bot         Bot
	compress    bool

	captime     time.Time // lock?

//	newclients  chan io.ReadWriteCloser
}

func NewDaemon(ln net.Listener, bot Bot, comp bool) (*Daemon, error) {
	d := Daemon {
		ln: ln,
		bot: bot,
		compress: comp,
		captime: time.Now(),
		Reflash: 500 * time.Millisecond,
	}

	return &d, nil
}

func (d *Daemon) Listen() {
	defer d.ln.Close()

	for {
		conn, err := d.ln.Accept()
		if err != nil {
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() && netErr.Temporary() {
				continue
			}
			Vln(2, "[Daemon]Error Accept:", err)
			return
		}
		if d.compress {
//			conn = NewFlateStream(conn, 1)
			conn = NewCompStream(conn, 1)
		}
		go d.handleConn(conn)
	}

}

func (d *Daemon) Close() (error) {
	return d.ln.Close()
}

func (d *Daemon) handleConn(p1 net.Conn) {
	var buf bytes.Buffer

	encoder := png.Encoder{
//		CompressionLevel: png.BestSpeed,
		CompressionLevel: png.NoCompression,
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
			Vln(3, "[Touch]", x, y, ev)
			d.bot.Touch(image.Pt(x, y), KeyAction(ev))

		case "Key":
			keycode, err := ReadTagStr(p1)
			if err != nil {
				Vln(2, "[todo][Key]err", err)
				return
			}
			ev, err := ReadVLen(p1)
			if err != nil {
				Vln(2, "[todo][Key][Ev]err", err)
				return
			}
			d.bot.Key(keycode, KeyAction(ev))

/*		case "ScreenSize":
			WriteVLen(p1, int64(d.bot.Screen.Dx()))
			WriteVLen(p1, int64(d.bot.Screen.Dy()))*/
		case "Screencap":
			if time.Since(d.captime) > d.Reflash { // keep away from impossible screencap frequency
				d.captime = time.Now()
				d.bot.TriggerScreencap()
			}

		case "GetScreen":
			encoder.Encode(&buf, d.bot.GetLastScreencap())
			WriteVTagByte(p1, buf.Bytes())
			buf.Reset()

/*		case "poll":
			Vln(3, "[todo][poll]", p1)
//			conn := NewCompStream(p1, 1)
			conn := NewFlateStream(p1, 1)
			d.newclients <- conn*/
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

/*func (d *Daemon) pollScreen(screen *[]byte) {
	var buf bytes.Buffer

	d.newclients = make(chan io.ReadWriteCloser, 16)
	clients := make(map[io.ReadWriteCloser]io.ReadWriteCloser, 0)

	encoder := png.Encoder{
//		CompressionLevel: png.BestSpeed,
		CompressionLevel: png.NoCompression,
	}
	limit := d.Reflash

	for {
		start := time.Now()
		_, err := d.bot.Screencap()
		if err != nil {
			return
		}
		Vln(4, "[screen][poll]", time.Since(start))

		encoder.Encode(&buf, d.bot.GetLastScreencap())
		*screen = buf.Bytes()
		buf.Reset()
		Vln(4, "[screen][encode]", len(*screen), time.Since(start))

		for i, c := range clients {
			err := WriteVTagByte(c, *screen)
			if err != nil {
				Vln(2, "[screen][push][err]", err)
				c.Close()
				delete(clients, i)
			}
		}
		for len(d.newclients) > 0 {
			client := <- d.newclients
			clients[client] = client
		}

		if time.Since(start) < limit {
			time.Sleep(limit - time.Since(start))
		}
	}
}*/



