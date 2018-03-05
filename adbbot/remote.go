package adbbot

import (
	"bytes"
	"image"
	"net"
//	"io"
	"time"
)

type task struct {
	Type      int	// 0 >> Key, 1 >> touch, 2 >> screencap, 3 >> shell
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
		case 0:
			err = WriteTagStr(b.conn, "Key")
			if err != nil {
				Vln(2, "[send][Key]err", err, todo)
				return
			}
			WriteTagStr(b.conn, todo.Op)
			WriteVLen(b.conn, int64(todo.Ev))

		case 1:
			err = WriteTagStr(b.conn, "Touch")
			if err != nil {
				Vln(2, "[send][Touch]err", err, todo)
				return
			}
			WriteVLen(b.conn, int64(todo.X0))
			WriteVLen(b.conn, int64(todo.Y0))
			WriteVLen(b.conn, int64(todo.Ev))

		case 2: // trigger screencap
			err = WriteTagStr(b.conn, "Screencap")
			if err != nil {
				Vln(2, "[send][Screencap]err", err, todo)
				return
			}

		case 3: // pull screen byte
			err = WriteTagStr(b.conn, "GetScreen")
			if err != nil {
				Vln(2, "[send][GetScreen]err", err, todo)
				return
			}
		}
	}
}


func (b *RemoteBot) Adb(parts string) ([]byte, error) { return []byte{}, nil } // nop

func (b *RemoteBot) Shell(parts string) ([]byte, error) {
	t := task {
		Type: 4,
		Op: parts,
	}
	b.op <- t
	return []byte{}, nil
}

func (b *RemoteBot) TriggerScreencap() (err error) {
	t := task {
		Type: 2,
	}
	b.op <- t
	return
}

func (b *RemoteBot) PullScreenByte() ([]byte, error) {
	t := task {
		Type: 3,
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

	return pngByte, nil
}

func (b *RemoteBot) Screencap() (img image.Image, err error) {
	// trgger cap
	err = b.TriggerScreencap()
	if err != nil {
		return nil, err
	}

	// pull
	pngByte, err := b.PullScreenByte()
	if err != nil {
		return nil, err
	}

	// decode
	r := bytes.NewReader(pngByte)
	img, err = Decode(r)
	if err != nil {
		return
	}

	// save Screen info
	b.ScreenBounds = img.Bounds()
	b.lastScreencap = img

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


// inputs
func (b *RemoteBot) Tap(loc image.Point) (err error) {
	return ErrNotImpl
}

func (b *RemoteBot) Text(in string) (err error) {
	return ErrNotImpl
}

func (b *RemoteBot) Press(in string) (err error) {
	return ErrNotImpl
}

func (b *RemoteBot) Touch(loc image.Point, ty KeyAction) (err error) {
	t := task {
		Type: 1,
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
		Type: 0,
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

func (b *RemoteBot) SwipeT(p0,p1 image.Point, time int) (err error) {
//	_, err = i.bot.Shell("input swipe " + strconv.Itoa(p0.X) + " " + strconv.Itoa(p0.Y) + " " + strconv.Itoa(p1.X) + " " + strconv.Itoa(p1.Y) + " " + strconv.Itoa(time))
	return ErrNotImpl
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


