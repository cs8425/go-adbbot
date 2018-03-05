package adbbot

import (
	"net"
	"time"
//	"sync"

//	"io"
	"bytes"
	"image"
	"image/png"
//	"image/jpeg"
)

type Daemon struct {
	Reflash     time.Duration

	ln          net.Listener
	bot         Bot
	compress    bool

	captime     time.Time // lock?
	triggerCh   chan struct{}
	screenBuf   bytes.Buffer
	bufReady    chan struct{}

//	newclients  chan io.ReadWriteCloser
}

func NewDaemon(ln net.Listener, bot Bot, comp bool) (*Daemon, error) {
	d := Daemon {
		ln: ln,
		bot: bot,
		compress: comp,
		triggerCh: make(chan struct{}, 1),
		bufReady: make(chan struct{}),
		Reflash: 500 * time.Millisecond,
	}

	go d.screenCoder()

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

func (d *Daemon) screenCoder() {
	// jpg option
//	option := &jpeg.Options{100}

	encBuf := &pngBuf{}
	encoder := png.Encoder{
//		CompressionLevel: png.BestSpeed,
		CompressionLevel: png.NoCompression,
		BufferPool: png.EncoderBufferPool(encBuf),
	}

	for {
		_, ok := <- d.triggerCh
		if !ok {
			return
		}
		if time.Since(d.captime) >= d.Reflash { // keep away from impossible screencap frequency
			d.captime = time.Now()
			d.bot.TriggerScreencap()
			Vln(4, "[screen][trigger]", time.Since(d.captime))
			d.screenBuf.Reset()
			encoder.Encode(&d.screenBuf, d.bot.GetLastScreencap())
//			jpeg.Encode(&d.screenBuf, d.bot.GetLastScreencap(), option)
			Vln(4, "[screen][encode]", time.Since(d.captime))

			select {
			case <- d.bufReady:
			default:
				close(d.bufReady)
			}
		}
	}

}

func (d *Daemon) Close() (error) {
	select {
	case _, ok := <- d.triggerCh:
		if ok {
			close(d.triggerCh)
		}
	default:
		close(d.triggerCh)
	}

	return d.ln.Close()
}

type pngBuf png.EncoderBuffer
func (b *pngBuf) Get() (*png.EncoderBuffer) {
	return (*png.EncoderBuffer)(b)
}
func (b *pngBuf) Put(*png.EncoderBuffer) { }

func (d *Daemon) handleConn(p1 net.Conn) {

	screenCh := make(chan struct{}, 1)
	defer close(screenCh)
	go func (p1 net.Conn, ch chan struct{}) {
		<- d.bufReady
		for {
			_, ok := <- ch
			if !ok {
				return
			}
			WriteVTagByte(p1, d.screenBuf.Bytes())
		}
	}(p1, screenCh)

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

		case "Screencap":
			select {
			case d.triggerCh <- struct{}{}:
			default:
			}

/*		case "ScreenSize":
			WriteVLen(p1, int64(d.bot.ScreenBounds.Dx()))
			WriteVLen(p1, int64(d.bot.ScreenBounds.Dy()))*/
		case "GetScreen":
			//WriteVTagByte(p1, buf.Bytes())
			screenCh <- struct{}{}

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


