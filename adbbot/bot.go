package adbbot

import (
	"image"
	"errors"
)

var (
	ErrNotImpl       = errors.New("not implemented")
	ErrNotSupport    = errors.New("not supported")

	ErrTriggerFirst  = errors.New("need trigger screencap first")
)

//            | on computer | on devices
//  localBot  |         adb | shell
//  remoteBot |  adb+daemon | shell+daemon
//  all with/without monkey
type Bot interface {

	Adb(parts string)    ([]byte, error)
	Shell(parts string)  ([]byte, error)

	TriggerScreencap()   (err error)
	GetLastScreencap()   (image.Image)
	Screencap()          (img image.Image, err error)
	PullScreenByte()     ([]byte, error)

	SaveScreen(imagefile string) (err error)

//	ScreenBounds()           (image.Rectangle)
	Remap(loc image.Point)   (image.Point)

	// input
	Input


	StartApp(app string) (err error)
	KillApp(app string) (err error)
}



