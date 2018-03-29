package adbbot

import (
	"net"
	"time"
	"sync"

	"bytes"
	"image"
	"image/png"
//	"image/jpeg"
)

const (
	OP_CLICK  = iota
	OP_SWIPE

	OP_TOUCH
	OP_KEY
	OP_TEXT

	OP_CAP
	OP_PULL

	OP_CMD    // no return data
	OP_SHELL  // pipe
)

type Daemon struct {
	Reflash     time.Duration

	ln          net.Listener
	bot         Bot
	compress    bool

	captime     time.Time // lock?
	triggerCh   chan struct{}
	screenBuf   bytes.Buffer

	screenReq   map[(chan struct{})](chan struct{})
	screenReqMx sync.Mutex
}

func NewDaemon(ln net.Listener, bot Bot, comp bool) (*Daemon, error) {
	d := Daemon {
		ln: ln,
		bot: bot,
		compress: comp,

		triggerCh: make(chan struct{}, 1),
		screenReq: make(map[(chan struct{})](chan struct{})),

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
//		if time.Since(d.captime) >= d.Reflash { // keep away from impossible screencap frequency
			d.captime = time.Now()
			d.bot.TriggerScreencap()
			Vln(4, "[screen][trigger]", time.Since(d.captime))
			d.screenBuf.Reset()
			encoder.Encode(&d.screenBuf, d.bot.GetLastScreencap())
//			jpeg.Encode(&d.screenBuf, d.bot.GetLastScreencap(), option)
			Vln(4, "[screen][encode]", time.Since(d.captime))

			d.screenReqMx.Lock()
			for _, req := range d.screenReq {
				select {
				case <- req:
				default:
					req <- struct{}{}
				}
			}
			d.screenReq = make(map[(chan struct{})](chan struct{}))
			d.screenReqMx.Unlock()
//		}
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
		for {
			_, ok := <- ch
			if !ok {
				return
			}
			WriteVTagByte(p1, d.screenBuf.Bytes())
		}
	}(p1, screenCh)

	for {
		todo, err := ReadVLen(p1)
		if err != nil {
			Vln(2, "[todo]err", err)
			return
		}
		Vln(4, "[todo]", todo)

		switch todo {
		case OP_TOUCH:
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
			err = d.bot.Touch(image.Pt(x, y), KeyAction(ev))

		case OP_KEY:
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
			err = d.bot.Key(keycode, KeyAction(ev))

		case OP_TEXT:
			text, err := ReadVTagByte(p1)
			if err != nil {
				Vln(2, "[todo][Text]err", err)
				return
			}
			err = d.bot.Text(string(text))
			if err != nil {
				Vln(2, "[run][Text]err", err)
			}

		case OP_CMD:
			text, err := ReadVTagByte(p1)
			if err != nil {
				Vln(2, "[todo][CMD]err", err)
				return
			}
			_, err = d.bot.Shell(string(text))
			if err != nil {
				Vln(2, "[run][CMD]err", err)
			}

		case OP_CAP:
			select {
			case d.triggerCh <- struct{}{}:
			default:
			}

/*		case "ScreenSize":
			WriteVLen(p1, int64(d.bot.ScreenBounds.Dx()))
			WriteVLen(p1, int64(d.bot.ScreenBounds.Dy()))*/
		case OP_PULL:
			d.screenReqMx.Lock()
			_, ok := d.screenReq[screenCh]
			if !ok {
				d.screenReq[screenCh] = screenCh
			}
			d.screenReqMx.Unlock()

		case OP_SHELL: // pipe shell
			go d.bot.ShellPipe(p1, "sh", true)
			return

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


