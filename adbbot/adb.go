package adb

import (
	"encoding/binary"
	"image"
	"io/ioutil"
	"strconv"
)

var dev = ""
var adbexec = "adb"

var local_tmp_path = "./"
var adb_tmp_path = "/data/local/tmp/"

var adb_su = true
var adb_pipe = true

var Last_screencap image.Image

var width = 0
var height = 0

func SetDev(device string) {
	if device != "" {
		dev = " -s " + device
	} else {
		dev = ""
	}
}

func SetAdb(Adb string) {
	adbexec = Adb
}

func SetMod(pipe bool, su bool) {
	adb_pipe = pipe
	adb_su = su
}

func Run(parts string) ([]byte, error) {
	return Cmd(adbexec + dev + " " + parts)
}

func Screencap() (img image.Image, err error){
	if adb_pipe {
		img, err = screencap_pipe()
	} else {
		img, err = screencap_file()
	}

	if err == nil {
		Last_screencap = img
	}

	return img, err
}

func screencap_pipe() (image.Image, error){
	screencap, err := Run("exec-out screencap")
	if err != nil {
		return nil, err
	}

	width = int(binary.LittleEndian.Uint32(screencap[0:4]))
	height = int(binary.LittleEndian.Uint32(screencap[4:8]))

	Vlogln(5, "height = ", height)
	Vlogln(5, "width = ", width)
	Vlogln(5, "length = ", len(screencap[12:]))
//	Vlogln(5, "dump = ", screencap[12:52])

	img := &image.NRGBA{
		Pix: screencap[12:],
		Stride: width * 4, // bytes
		Rect: image.Rect(0, 0, width, height),
	}

	return img, nil
}

func screencap_file() (image.Image, error){

	if adb_su {
		_, err := Run("shell su -c screencap /dev/screencap-tmp.raw")
		if err != nil {
			return nil, err
		}
		_, err = Run("shell su -c chmod 666 /dev/screencap-tmp.raw")
		if err != nil {
			return nil, err
		}
		_, err = Run("pull /dev/screencap-tmp.raw " + local_tmp_path)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := Run("shell screencap " + adb_tmp_path + "screencap-tmp.raw")
		if err != nil {
			return nil, err
		}
		_, err = Run("pull " + adb_tmp_path + "screencap-tmp.raw " + local_tmp_path)
		if err != nil {
			return nil, err
		}
	}

	screencap, err := ioutil.ReadFile(local_tmp_path + "screencap-tmp.raw")
	if err != nil {
		return nil, err
	}

	width = int(binary.LittleEndian.Uint32(screencap[0:4]))
	height = int(binary.LittleEndian.Uint32(screencap[4:8]))

	Vlogln(5, "height = ", height)
	Vlogln(5, "width = ", width)
	Vlogln(5, "length = ", len(screencap[12:]))

	img := &image.NRGBA{
		Pix: screencap[12:],
		Stride: width * 4, // bytes
		Rect: image.Rect(0, 0, width, height),
	}

	return img, nil
}

/*func Click(x, y int) (err error){
	_, err := Run("shell input tap " + strconv.Itoa(x) + " " + strconv.Itoa(y))
	return
}*/

func Click(loc image.Point) (err error){
	_, err = Run("shell input tap " + strconv.Itoa(loc.X) + " " + strconv.Itoa(loc.Y))
	return
}

func Swipe(a,b image.Point) (err error){
	_, err = Run("shell input swipe " + strconv.Itoa(a.X) + " " + strconv.Itoa(a.Y) + " " + strconv.Itoa(b.X) + " " + strconv.Itoa(b.Y))
	return
}

func Text(in string) (err error){
	_, err = Run("shell input text " + in)
	return
}

func Textln(in string) (err error){
	err = Text(in)
	if err != nil {
		return
	}

	_, err = Run("shell input keyevent KEYCODE_ENTER")
	return
}

func Keyevent(in string) (err error){
	_, err = Run("shell input keyevent " + in)
	return
}

func KeyHome() (err error){
	_, err = Run("shell input keyevent KEYCODE_HOME")
	return
}

func StartApp(app string) (err error){
	_, err = Run("shell monkey -p " + app + " -c android.intent.category.LAUNCHER 1")
	return
}

func KillApp(app string) (err error){
	_, err = Run("shell am force-stop " + app)
	return
}

func SaveScreen(imagefile string) (err error){
	img, err := Screencap()
	if err != nil {
		return
	}
	err = SaveImage(img, imagefile)
	return
}

