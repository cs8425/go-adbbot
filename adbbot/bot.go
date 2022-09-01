package adbbot

import (
	"errors"
	"image"
	"io"
	"os/exec"
)

var (
	ErrNotImpl    = errors.New("not implemented")
	ErrNotSupport = errors.New("not supported")

	ErrTriggerFirst = errors.New("need trigger screencap first")
)

//            | on computer | on devices
//  localBot  |         adb | shell
//  remoteBot |  adb+daemon | shell+daemon
//  all with/without monkey
type Bot interface {
	Adb(parts string) ([]byte, error)
	Shell(parts string) ([]byte, error)

	// will block untill sh exit
	ShellPipe(p1 io.ReadWriteCloser, cmds string, blocking bool) (*exec.Cmd, error)

	TriggerScreencap() (err error)
	GetLastScreencap() image.Image // cached image
	Screencap() (img image.Image, err error)
	PullScreenByte() ([]byte, error) // raw byte (png or RGBA), may not cached

	SaveScreen(imagefile string) (err error)

	// ScreenBounds()           (image.Rectangle)
	Remap(loc image.Point) image.Point

	ImgCompLv(lv int)

	// input
	Input

	StartApp(app string) (err error)
	KillApp(app string) (err error)
}
