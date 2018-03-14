package adbbot

import (
	"bytes"
	"image"
	"net"
	"io"
	"time"
)

type task struct {
	Type      int
	Op        string
	X0        int
	Y0        int
	Ev        KeyAction
}

type RemoteBot struct {

	ScreenBounds    image.Rectangle
	lastScreencap   image.Image

	conn            net.Conn // cmd

	FindOnDaemon    bool // TODO
	compress        bool
	op              chan task

	KeyDelta  time.Duration

	Input
}

func NewRemoteBot(conn net.Conn, comp bool) (*RemoteBot, error) {
	if comp {
		//conn = NewFlateStream(conn, 1)
		conn = NewCompStream(conn, 1)
	}

	b := RemoteBot {
		compress: comp,
		conn: conn,
		op: make(chan task, 4),
		KeyDelta: 100 * time.Millisecond,
	}

	go b.pushworker()

	return &b, nil
}

func (b *RemoteBot) pushworker() {
	var err error

	for {
		todo := <- b.op

		switch todo.Type {
		case OP_TOUCH:
			err = WriteVLen(b.conn, OP_TOUCH)
			if err != nil {
				Vln(2, "[send][Touch]err", err, todo)
				return
			}
			WriteVLen(b.conn, int64(todo.X0))
			WriteVLen(b.conn, int64(todo.Y0))
			WriteVLen(b.conn, int64(todo.Ev))

		case OP_KEY:
			err = WriteVLen(b.conn, OP_KEY)
			if err != nil {
				Vln(2, "[send][Key]err", err, todo)
				return
			}
			WriteTagStr(b.conn, todo.Op)
			WriteVLen(b.conn, int64(todo.Ev))

		case OP_TEXT:
			fallthrough
		case OP_CMD:
			err = WriteVLen(b.conn, int64(todo.Type))
			if err != nil {
				Vln(2, "[send][Str]err", err, todo)
				return
			}
			WriteVTagByte(b.conn, []byte(todo.Op))

		case OP_CAP: // trigger screencap
			err = WriteVLen(b.conn, OP_CAP)
			if err != nil {
				Vln(2, "[send][Screencap]err", err, todo)
				return
			}

		case OP_PULL: // pull screen byte
			err = WriteVLen(b.conn, OP_PULL)
			if err != nil {
				Vln(2, "[send][GetScreen]err", err, todo)
				return
			}
		}
	}
}


func (b *RemoteBot) Adb(parts string) ([]byte, error) { return []byte{}, ErrNotSupport } // nop

func (b *RemoteBot) Shell(parts string) ([]byte, error) {
	t := task {
		Type: OP_CMD,
		Op: parts,
	}
	b.op <- t
	return []byte{}, nil
}

func (b *RemoteBot) ShellPipe(p1 io.ReadWriteCloser) (error) {
	// create new connection, and switch to pipe mode
	return ErrNotImpl
}

func (b *RemoteBot) TriggerScreencap() (err error) {
	t := task {
		Type: OP_CAP,
	}
	b.op <- t
	return
}

func (b *RemoteBot) PullScreenByte() ([]byte, error) {
	t := task {
		Type: OP_PULL,
	}
	b.op <- t

	pngByte, err := ReadVTagByte(b.conn)
	if err != nil {
		Vln(2, "[read][GetScreen]err", err)
		return nil, err
	}

	if len(pngByte) == 0 {
		return nil, ErrTriggerFirst
	}

	// decode
	r := bytes.NewReader(pngByte)
	img, err := Decode(r)
	if err != nil {
		return nil, err
	}

	// save Screen info
	b.ScreenBounds = img.Bounds()
	b.lastScreencap = img

	return pngByte, nil
}

func (b *RemoteBot) Screencap() (img image.Image, err error) {
	// trgger cap
	err = b.TriggerScreencap()
	if err != nil {
		return nil, err
	}

	// pull
	_, err = b.PullScreenByte()
	if err != nil {
		return nil, err
	}

	return b.lastScreencap, err
}

func (b *RemoteBot) GetLastScreencap() (image.Image) {
	return b.lastScreencap
}

func (b *RemoteBot) SaveScreen(imagefile string) (err error) {
	img, err := b.Screencap()
	if err != nil {
		return
	}
	err = SaveImage(img, imagefile)
	return
}

func (b *RemoteBot) StartApp(app string) (err error) {
	_, err = b.Shell("monkey -p " + app + " -c android.intent.category.LAUNCHER 1")
	return
}

func (b *RemoteBot) KillApp(app string) (err error) {
	_, err = b.Shell("am force-stop " + app)
	return
}

func (b *RemoteBot) Remap(loc image.Point) (image.Point) {
	return loc
}


// inputs
func (b *RemoteBot) Tap(loc image.Point) (err error) {
	return ErrNotImpl
}

func (b *RemoteBot) Text(in string) (err error) {
	t := task {
		Type: OP_TEXT,
		Op: in,
	}
	Vln(4, "[text]", t)
	b.op <- t
	return
}

func (b *RemoteBot) Press(in string) (err error) {
	return ErrNotImpl
}

func (b *RemoteBot) Touch(loc image.Point, ty KeyAction) (err error) {
	t := task {
		Type: OP_TOUCH,
		X0: loc.X,
		Y0: loc.Y,
		Ev: ty,
	}
	Vln(4, "[key]", t)
	b.op <- t
	return
}

func (b *RemoteBot) Key(in string, ty KeyAction) (err error) {
	t := task {
		Type: OP_KEY,
		Op: in,
		Ev: ty,
	}
	Vln(4, "[key]", t)
	b.op <- t
	return
}

func (b *RemoteBot) Click(loc image.Point) (err error) {
	err = b.Touch(loc, KEY_DOWN)
	if err != nil {
		return
	}
	time.Sleep(b.KeyDelta)
	return b.Touch(loc, KEY_UP)
}

func (b *RemoteBot) SwipeT(p0,p1 image.Point, dtime int) (err error) {
	if dtime <= 0 {
		dtime = 300
	}
	start := time.Now()
	dur := time.Duration(dtime) * time.Millisecond

	err = b.Touch(p0, KEY_DOWN)
	if err != nil {
		return
	}

	pt := image.Pt(0, 0)
	pd := image.Pt(p1.X - p0.X, p1.Y - p0.Y)
	esp := time.Since(start)
	for esp < dur {
		alpha := float64(esp) / float64(dur)
		pt.X = p0.X + int(float64(pd.X) * alpha)
		pt.Y = p0.Y + int(float64(pd.Y) * alpha)

		err = b.Touch(pt, KEY_MV)
		if err != nil {
			return
		}
		time.Sleep(1 * time.Millisecond)

		esp = time.Since(start)
	}

	err = b.Touch(p1, KEY_UP)
	return
}

func (b *RemoteBot) Keyevent(in string) (err error) {
	err = b.Key(in, KEY_DOWN)
	if err != nil {
		return
	}
	time.Sleep(b.KeyDelta)
	return b.Key(in, KEY_UP)
}

func (b *RemoteBot) KeyHome() (error) {
	return b.Keyevent("KEYCODE_HOME")
}

func (b *RemoteBot) KeyBack() (error) {
	return b.Keyevent("KEYCODE_BACK")
}

func (b *RemoteBot) KeySwitch() (error) {
	return b.Keyevent("KEYCODE_APP_SWITCH")
}

func (b *RemoteBot) KeyPower() (error) {
	return b.Keyevent("KEYCODE_POWER")
}


