package adbbot

import (
	"image"
)

//            | on computer | on devices
//  localBot  |         adb | shell
//  remoteBot |  adb+daemon | shell+daemon
//  all with/without monkey
type Bot interface {

	Adb(parts string) ([]byte, error)
	Shell(parts string) ([]byte, error)

	Screencap() (img image.Image, err error)
	SaveScreen(imagefile string) (err error)

//	GetLastScreencap() (image.Image)
//	Screen()           (*image.Rectangle)

	// input
	Input

	/*Click(loc image.Point, remap bool) (err error)
	Swipe(p0,p1 image.Point, remap bool) (err error)
	SwipeT(p0,p1 image.Point, time int, remap bool) (err error)

	Text(in string) (err error)

	Keyevent(in string) (err error)
	KeyHome() (error)
	KeyBack() (error)
	KeySwitch() (error)
	KeyPower() (error)*/

	StartApp(app string) (err error)
	KillApp(app string) (err error)
}


