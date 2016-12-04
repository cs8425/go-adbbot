package adbbot

import (
	"encoding/binary"
	"image"
	"io/ioutil"
	"os/exec"
	"strconv"
)

type Bot struct {
	Dev             string
	Exec            string
	UseSU           bool
	UsePipe         bool
	IsOnDevice      bool

	Local_tmp_path  string
	Adb_tmp_path    string

	Last_screencap  image.Image

	Screen          *image.Rectangle
	TargetScreen    *image.Rectangle

	devstr          string
	width           int
	height          int


	// shortcuts
	NewTmpl                  func(filename string, reg image.Rectangle) (*Tmpl, error)
	Rect, NewRect            func(x, y, xp, yp int) (image.Rectangle)
	RectAbs, NewRectAbs      func(x, y, x2, y2 int) (image.Rectangle)
	RectAll, NewRectAll      func() (image.Rectangle)
}

func NewBot(device, exec string) (*Bot) {
	b := Bot{
		Dev: device,
		Exec: exec,
		UseSU: true,
		UsePipe: true,

		Local_tmp_path: "./",
		Adb_tmp_path:  "/data/local/tmp/",
		IsOnDevice: false,
//		devstr: "",

		Screen: nil,
		TargetScreen: nil,

		Rect: NewRect,
		RectAbs: NewRectAbs,
		RectAll: NewRectAll,
	}

	b.NewTmpl = NewTmpl
	b.NewRect = NewRect
	b.NewRectAbs = NewRectAbs
	b.NewRectAll = NewRectAll

	if device != "" {
		b.devstr = " -s " + device
	} else {
		b.devstr = ""
	}

	if exec == "" {
		b.Exec = "adb"
	}

	return &b
}

func NewBotOnDevice() (*Bot) {
	b := NewBot("","")
	b.IsOnDevice = true
	return b
}

func (b *Bot) Adb(parts string) ([]byte, error) {
	if b.IsOnDevice {
		// nop
		return []byte{}, nil
	} else {
		return Cmd(b.Exec + b.devstr + " " + parts)
	}
}

func (b *Bot) Shell(parts string) ([]byte, error) {
	if b.IsOnDevice {
		cmd := []string{"-c", parts}
		return exec.Command("sh", cmd...).Output()
	} else {
		return b.Adb("shell " + parts)
	}
}

func (b *Bot) Pipe(parts string) ([]byte, error) {
	if b.IsOnDevice {
		return Cmd(parts)
	} else {
		return b.Adb("exec-out " + parts)
	}
}

func (b *Bot) Screencap() (img image.Image, err error){
	var screencap []byte

	if b.UsePipe {
		screencap, err = b.Pipe("screencap")
	} else {
		screencap, err = b.screencap_file()
	}

	Vlogln(5, "screen", b.width, b.height, b.Screen)

	b.width = int(binary.LittleEndian.Uint32(screencap[0:4]))
	b.height = int(binary.LittleEndian.Uint32(screencap[4:8]))

	Vlogln(5, "height = ", b.height)
	Vlogln(5, "width = ", b.width)
	Vlogln(5, "length = ", len(screencap[12:]))
//	Vlogln(5, "dump = ", screencap[12:52])

	if b.Screen == nil {
		b.Screen = &image.Rectangle{image.Pt(0, 0), image.Pt(b.width, b.height)}
		Vlogln(6, "set screen", b.width, b.height, b.Screen)
	}

	img = &image.NRGBA{
		Pix: screencap[12:],
		Stride: b.width * 4, // bytes
		Rect: image.Rect(0, 0, b.width, b.height),
	}

	if err == nil {
		b.Last_screencap = img
	}

	return img, err
}

func (b *Bot) screencap_file() ([]byte, error){

	if b.UseSU {
		_, err := b.Shell("su -c screencap /dev/screencap-tmp.raw")
		if err != nil {
			return nil, err
		}
		_, err = b.Shell("su -c chmod 666 /dev/screencap-tmp.raw")
		if err != nil {
			return nil, err
		}
		_, err = b.Adb("pull /dev/screencap-tmp.raw " + b.Local_tmp_path)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := b.Shell("screencap " + b.Adb_tmp_path + "screencap-tmp.raw")
		if err != nil {
			return nil, err
		}
		_, err = b.Adb("pull " + b.Adb_tmp_path + "screencap-tmp.raw " + b.Local_tmp_path)
		if err != nil {
			return nil, err
		}
	}

	screencap, err := ioutil.ReadFile(b.Local_tmp_path + "screencap-tmp.raw")
	if err != nil {
		return nil, err
	}

	return screencap, nil
}


func (b *Bot) ScriptScreen(x0, y0, dx, dy int) () {
	b.TargetScreen = &image.Rectangle{image.Pt(x0, y0), image.Pt(dx, dy)}
	Vlogln(3, "set Script Screen", x0, y0, dx, dy, b.TargetScreen)
}

func (b Bot) Click(loc image.Point) (err error){
	_, err = b.Shell("input tap " + strconv.Itoa(loc.X) + " " + strconv.Itoa(loc.Y))
	return
}

func (b Bot) Swipe(p0,p1 image.Point) (err error){
	_, err = b.Shell("input swipe " + strconv.Itoa(p0.X) + " " + strconv.Itoa(p0.Y) + " " + strconv.Itoa(p1.X) + " " + strconv.Itoa(p1.Y))
	return
}

func (b Bot) Text(in string) (err error){
	_, err = b.Shell("input text " + in)
	return
}

func (b Bot) Textln(in string) (err error){
	err = b.Text(in)
	if err != nil {
		return
	}

	err = b.Keyevent("KEYCODE_ENTER")
	return
}

func (b Bot) Keyevent(in string) (err error){
	_, err = b.Shell("input keyevent " + in)
	return
}

func (b Bot) KeyHome() (error){
	return b.Keyevent("KEYCODE_HOME")
}

func (b Bot) KeyBack() (error){
	return b.Keyevent("KEYCODE_BACK")
}

func (b Bot) KeySwitch() (error){
	return b.Keyevent("KEYCODE_APP_SWITCH")
}

func (b Bot) StartApp(app string) (err error){
	_, err = b.Shell("monkey -p " + app + " -c android.intent.category.LAUNCHER 1")
	return
}

func (b Bot) KillApp(app string) (err error){
	_, err = b.Shell("am force-stop " + app)
	return
}

func (b *Bot) SaveScreen(imagefile string) (err error){
	img, err := b.Screencap()
	if err != nil {
		return
	}
	err = SaveImage(img, imagefile)
	return
}

