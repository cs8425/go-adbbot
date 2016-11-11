package adbbot

import (
	"encoding/binary"
	"image"
	"io/ioutil"
	"strconv"
)

type Bot struct {
	Dev             string
	Exec            string
	UseSU           bool
	UsePipe         bool

	Local_tmp_path  string
	Adb_tmp_path    string

	Last_screencap  image.Image

	devstr          string
	width           int
	height          int

	// shortcuts
	NewTmpl                  func(filename string, reg image.Rectangle) (*Tmpl, error)
	Rect, NewRect            func(x, y, xp, yp int) (image.Rectangle)
	RectAbs, NewRectAbs      func(x, y, x2, y2 int) (image.Rectangle)
	RectAll, NewRectAll      func() (image.Rectangle)
}

func NewBot() (*Bot) {
	b := Bot{
		Dev: "",
		Exec: "adb",
		UseSU: true,
		UsePipe: true,

		Local_tmp_path: "./",
		Adb_tmp_path:  "/data/local/tmp/",
		devstr: "",

		Rect: NewRect,
		RectAbs: NewRectAbs,
		RectAll: NewRectAll,
	}

	b.NewTmpl = NewTmpl
	b.NewRect = NewRect
	b.NewRectAbs = NewRectAbs
	b.NewRectAll = NewRectAll

	return &b
}

func (b Bot) SetDev(device string) {
	b.Dev = device
	if device != "" {
		b.devstr = " -s " + device
	} else {
		b.devstr = ""
	}
}

func (b Bot) Run(parts string) ([]byte, error) {
	return Cmd(b.Exec + b.devstr + " " + parts)
}

func (b Bot) Screencap() (img image.Image, err error){
	var screencap []byte

	if b.UsePipe {
		screencap, err = b.screencap_pipe()
	} else {
		screencap, err = b.screencap_file()
	}

	b.width = int(binary.LittleEndian.Uint32(screencap[0:4]))
	b.height = int(binary.LittleEndian.Uint32(screencap[4:8]))

	Vlogln(5, "height = ", b.height)
	Vlogln(5, "width = ", b.width)
	Vlogln(5, "length = ", len(screencap[12:]))
//	Vlogln(5, "dump = ", screencap[12:52])

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

func (b Bot) screencap_pipe() ([]byte, error){
	screencap, err := b.Run("exec-out screencap")
	if err != nil {
		return nil, err
	}

	return screencap, nil
}

func (b Bot) screencap_file() ([]byte, error){

	if b.UseSU {
		_, err := b.Run("shell su -c screencap /dev/screencap-tmp.raw")
		if err != nil {
			return nil, err
		}
		_, err = b.Run("shell su -c chmod 666 /dev/screencap-tmp.raw")
		if err != nil {
			return nil, err
		}
		_, err = b.Run("pull /dev/screencap-tmp.raw " + b.Local_tmp_path)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := b.Run("shell screencap " + b.Adb_tmp_path + "screencap-tmp.raw")
		if err != nil {
			return nil, err
		}
		_, err = b.Run("pull " + b.Adb_tmp_path + "screencap-tmp.raw " + b.Local_tmp_path)
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


func (b Bot) Click(loc image.Point) (err error){
	_, err = b.Run("shell input tap " + strconv.Itoa(loc.X) + " " + strconv.Itoa(loc.Y))
	return
}

func (b Bot) Swipe(p0,p1 image.Point) (err error){
	_, err = b.Run("shell input swipe " + strconv.Itoa(p0.X) + " " + strconv.Itoa(p0.Y) + " " + strconv.Itoa(p1.X) + " " + strconv.Itoa(p1.Y))
	return
}

func (b Bot) Text(in string) (err error){
	_, err = b.Run("shell input text " + in)
	return
}

func (b Bot) Textln(in string) (err error){
	err = b.Text(in)
	if err != nil {
		return
	}

	_, err = b.Run("shell input keyevent KEYCODE_ENTER")
	return
}

func (b Bot) Keyevent(in string) (err error){
	_, err = b.Run("shell input keyevent " + in)
	return
}

func (b Bot) KeyHome() (err error){
	_, err = b.Run("shell input keyevent KEYCODE_HOME")
	return
}

func (b Bot) KeyBack() (err error){
	_, err = b.Run("shell input keyevent KEYCODE_BACK")
	return
}

func (b Bot) KeySwitch() (err error){
	_, err = b.Run("shell input keyevent KEYCODE_APP_SWITCH")
	return
}

func (b Bot) StartApp(app string) (err error){
	_, err = b.Run("shell monkey -p " + app + " -c android.intent.category.LAUNCHER 1")
	return
}

func (b Bot) KillApp(app string) (err error){
	_, err = b.Run("shell am force-stop " + app)
	return
}

func (b Bot) SaveScreen(imagefile string) (err error){
	img, err := b.Screencap()
	if err != nil {
		return
	}
	err = SaveImage(img, imagefile)
	return
}

