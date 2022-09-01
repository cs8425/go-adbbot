package adbbot

import (
	"encoding/binary"
	"image"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
)

type LocalBot struct {
	Dev        string
	Exec       string
	UseSU      bool
	UsePipe    bool
	IsOnDevice bool

	Local_tmp_path string
	Adb_tmp_path   string

	lastScreencap image.Image

	// Screen          *image.Rectangle
	TargetScreen *image.Rectangle

	ScreenBounds image.Rectangle

	scale float64

	devstr string
	width  int
	height int

	Input
}

func NewLocalBot(device, exec string) *LocalBot {
	b := LocalBot{
		Dev:     device,
		Exec:    exec,
		UseSU:   true,
		UsePipe: true,

		Local_tmp_path: "./",
		Adb_tmp_path:   "/data/local/tmp/",
		IsOnDevice:     false,

		// Screen: nil,
		TargetScreen: nil,
		scale:        1.0,
	}

	input := NewCmdInput(&b)
	b.Input = input

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

func NewLocalBotOnDevice() *LocalBot {
	b := NewLocalBot("", "")
	b.IsOnDevice = true
	return b
}

func (b *LocalBot) Adb(parts string) ([]byte, error) {
	if b.IsOnDevice {
		// nop
		return []byte{}, nil
	} else {
		parts := strings.Fields(b.Exec + b.devstr + " " + parts)
		head := parts[0]
		parts = parts[1:len(parts)]

		return exec.Command(head, parts...).Output()
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

func (b *LocalBot) ShellPipe(p1 io.ReadWriteCloser, cmds string, blocking bool) (*exec.Cmd, error) {
	var parts []string

	if b.IsOnDevice {
		parts = []string{"sh", "-c", cmds}
	} else {
		parts = strings.Fields(b.Exec + b.devstr + " shell " + cmds)
	}

	head := parts[0]
	parts = parts[1:len(parts)]

	// Vln(4, "[bot][ShellPipe]", p1, head, parts)

	cmd := exec.Command(head, parts...)
	cmd.Stdout = p1
	cmd.Stderr = p1
	cmd.Stdin = p1
	err := cmd.Start()
	if err != nil && p1 != nil {
		p1.Write([]byte(err.Error()))
	}

	if blocking {
		err = cmd.Wait()
		if err != nil && p1 != nil {
			p1.Write([]byte(err.Error()))
		}
	}

	return cmd, err
}

func (b *LocalBot) Pipe(parts string) ([]byte, error) {
	if b.IsOnDevice {
		return Cmd(parts)
	} else {
		return b.Adb("exec-out " + parts)
	}
}

func (b *LocalBot) ImgCompLv(lv int) {

}

func (b *LocalBot) TriggerScreencap() (err error) {
	var screencap []byte

	if b.UsePipe {
		screencap, err = b.Pipe("screencap")
	} else {
		screencap, err = b.screencap_file()
	}

	Vln(5, "screen", b.width, b.height, b.ScreenBounds, b.TargetScreen)

	width := int(binary.LittleEndian.Uint32(screencap[0:4]))
	height := int(binary.LittleEndian.Uint32(screencap[4:8]))

	Vln(5, "height = ", height)
	Vln(5, "width = ", width)
	Vln(5, "length = ", len(screencap[12:]))
	// Vln(5, "dump = ", screencap[12:52])

	if b.ScreenBounds.Empty() {
		b.ScreenBounds = image.Rectangle{image.Pt(0, 0), image.Pt(b.width, b.height)}
		Vln(5, "set screen", b.width, b.height, b.ScreenBounds)
	}

	// for screen rotation change
	if height != b.height || width != b.width {
		b.width = width
		b.height = height
		b.ScreenBounds.Max.X = width
		b.ScreenBounds.Max.X = height
	}

	img := &image.NRGBA{
		Pix:    screencap[12:],
		Stride: b.width * 4, // bytes
		Rect:   image.Rect(0, 0, b.width, b.height),
	}

	if b.scale != 1.0 {
		newX := int(float64(width) * b.scale)
		// img = Resize(img, newX, 0, Lanczos)
		// img = Resize(img, newX, 0, Box)
		img = Resize(img, newX, 0, NearestNeighbor)
	}

	if err == nil {
		b.lastScreencap = img
	}

	return
}

func (b *LocalBot) Screencap() (img image.Image, err error) {
	err = b.TriggerScreencap()
	if err != nil {
		return nil, err
	}
	return b.lastScreencap, err
}

func (b *LocalBot) screencap_file() ([]byte, error) {

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

func (b *LocalBot) PullScreenByte() ([]byte, error) {
	if b.lastScreencap == nil {
		return nil, ErrTriggerFirst
	}
	return b.lastScreencap.(*image.NRGBA).Pix, nil
}

func (b *LocalBot) GetLastScreencap() image.Image {
	return b.lastScreencap
}

func (b *LocalBot) SaveScreen(imagefile string) (err error) {
	img, err := b.Screencap()
	if err != nil {
		return
	}
	err = SaveImage(img, imagefile)
	return
}

func (b *LocalBot) StartApp(app string) (err error) {
	_, err = b.Shell("monkey -p " + app + " -c android.intent.category.LAUNCHER 1")
	return
}

func (b *LocalBot) KillApp(app string) (err error) {
	_, err = b.Shell("am force-stop " + app)
	return
}

func (b *LocalBot) SetScale(scale float64) {
	b.scale = scale
}

func (b *LocalBot) Remap(loc image.Point) image.Point {
	if b.scale != 1.0 {
		return image.Pt(int(float64(loc.X)/b.scale), int(float64(loc.Y)/b.scale))
	}
	return loc
}

func (b *LocalBot) ScriptScreen(x0, y0, dx, dy int) {
	b.TargetScreen = &image.Rectangle{image.Pt(x0, y0), image.Pt(dx, dy)}
	Vln(4, "set Script Screen", x0, y0, dx, dy, b.TargetScreen)
}
