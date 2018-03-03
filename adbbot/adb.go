package adbbot

import (
	"encoding/binary"
	"image"
	"io/ioutil"
	"os/exec"
//	"strconv"
)

type LocalBot struct {
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

//	fb              image.Image

	devstr          string
	width           int
	height          int


	Input


	// shortcuts
	NewTmpl                  func(filename string, reg image.Rectangle) (*Tmpl, error)
	Rect, NewRect            func(x, y, xp, yp int) (image.Rectangle)
	RectAbs, NewRectAbs      func(x, y, x2, y2 int) (image.Rectangle)
	RectAll, NewRectAll      func() (image.Rectangle)

//	Adb             func(parts string) ([]byte, error)
//	Shell           func(parts string) ([]byte, error)
//	Pipe            func(parts string) ([]byte, error)
}

func NewLocalBot(device, exec string) (*LocalBot) {
	b := LocalBot {
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

	input := NewCmdInput(&b)
	b.Input = input

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

func NewLocalBotOnDevice() (*LocalBot) {
	b := NewLocalBot("","")
	b.IsOnDevice = true
//	b.fb, _ = FBOpen("/dev/graphics/fb0")
	return b
}

func (b *LocalBot) Adb(parts string) ([]byte, error) {
	if b.IsOnDevice {
		// nop
		return []byte{}, nil
	} else {
		return Cmd(b.Exec + b.devstr + " " + parts)
	}
}

func (b *LocalBot) Shell(parts string) ([]byte, error) {
	if b.IsOnDevice {
		cmd := []string{"-c", parts}
		return exec.Command("sh", cmd...).Output()
	} else {
		return b.Adb("shell " + parts)
	}
}

func (b *LocalBot) Pipe(parts string) ([]byte, error) {
	if b.IsOnDevice {
		return Cmd(parts)
	} else {
		return b.Adb("exec-out " + parts)
	}
}

func (b *LocalBot) Screencap() (img image.Image, err error){
/*	if b.fb != nil {
		return b.fb, nil
	}*/

	var screencap []byte

	if b.UsePipe {
		screencap, err = b.Pipe("screencap")
	} else {
		screencap, err = b.screencap_file()
	}

	Vln(5, "screen", b.width, b.height, b.Screen, b.TargetScreen)

	b.width = int(binary.LittleEndian.Uint32(screencap[0:4]))
	b.height = int(binary.LittleEndian.Uint32(screencap[4:8]))

	Vln(5, "height = ", b.height)
	Vln(5, "width = ", b.width)
	Vln(5, "length = ", len(screencap[12:]))
//	Vln(5, "dump = ", screencap[12:52])

	if b.Screen == nil {
		b.Screen = &image.Rectangle{image.Pt(0, 0), image.Pt(b.width, b.height)}
//		b.Screen = new(image.Rectangle)
//		b.Screen.Min = image.Pt(0, 0)
//		b.Screen.Max = image.Pt(b.width, b.height)
		Vln(5, "set screen", b.width, b.height, b.Screen)
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

func (b *LocalBot) screencap_file() ([]byte, error){

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


func (b *LocalBot) ScriptScreen(x0, y0, dx, dy int) () {
	b.TargetScreen = &image.Rectangle{image.Pt(x0, y0), image.Pt(dx, dy)}
	Vln(4, "set Script Screen", x0, y0, dx, dy, b.TargetScreen)
}

func (b *LocalBot) Remap(loc image.Point) (image.Point){
	x := loc.X
	y := loc.Y
	Vln(4, "Remap", b.TargetScreen, b.Screen)
	if b.TargetScreen != nil && b.Screen != nil {
		scriptsize := b.TargetScreen.Size()
		screensize := b.Screen.Size()
		x = x * screensize.X / scriptsize.X
		y = y * screensize.X / scriptsize.X
		Vln(4, "Remap to", x, y)
	}
	return image.Pt(x, y)
}

/*func (b *LocalBot) Click(loc image.Point, remap bool) (err error){
	if remap {
		loc = b.Remap(loc)
	}
	_, err = b.Shell("input tap " + strconv.Itoa(loc.X) + " " + strconv.Itoa(loc.Y))
	return
}

func (b *LocalBot) Swipe(p0,p1 image.Point, remap bool) (err error){
	if remap {
		p0 = b.Remap(p0)
		p1 = b.Remap(p1)
	}
	_, err = b.Shell("input swipe " + strconv.Itoa(p0.X) + " " + strconv.Itoa(p0.Y) + " " + strconv.Itoa(p1.X) + " " + strconv.Itoa(p1.Y))
	return
}

func (b *LocalBot) SwipeT(p0,p1 image.Point, time int, remap bool) (err error){
	if remap {
		p0 = b.Remap(p0)
		p1 = b.Remap(p1)
	}
	_, err = b.Shell("input swipe " + strconv.Itoa(p0.X) + " " + strconv.Itoa(p0.Y) + " " + strconv.Itoa(p1.X) + " " + strconv.Itoa(p1.Y) + " " + strconv.Itoa(time))
	return
}

func (b LocalBot) Text(in string) (err error){
	_, err = b.Shell("input text " + in)
	return
}

func (b LocalBot) Textln(in string) (err error){
	err = b.Text(in)
	if err != nil {
		return
	}

	err = b.Keyevent("KEYCODE_ENTER")
	return
}

func (b LocalBot) Keyevent(in string) (err error){
	_, err = b.Shell("input keyevent " + in)
	return
}

func (b LocalBot) KeyHome() (error){
	return b.Keyevent("KEYCODE_HOME")
}

func (b LocalBot) KeyBack() (error){
	return b.Keyevent("KEYCODE_BACK")
}

func (b LocalBot) KeySwitch() (error){
	return b.Keyevent("KEYCODE_APP_SWITCH")
}

func (b LocalBot) KeyPower() (error){
	return b.Keyevent("KEYCODE_POWER")
}*/

func (b LocalBot) StartApp(app string) (err error){
	_, err = b.Shell("monkey -p " + app + " -c android.intent.category.LAUNCHER 1")
	return
}

func (b LocalBot) KillApp(app string) (err error){
	_, err = b.Shell("am force-stop " + app)
	return
}

func (b *LocalBot) SaveScreen(imagefile string) (err error){
	img, err := b.Screencap()
	if err != nil {
		return
	}
	err = SaveImage(img, imagefile)
	return
}

