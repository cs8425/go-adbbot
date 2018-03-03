package adbbot

import (
	"fmt"
	"image"
	"net"
	"time"
	"strconv"
)

type Input interface {
	Click(loc image.Point) (err error)
	SwipeT(p0,p1 image.Point, time int) (err error)

	Text(in string) (err error)

	Keyevent(in string) (err error)
	KeyHome() (error)
	KeyBack() (error)
	KeySwitch() (error)
	KeyPower() (error)
}


// input by monkey
// NOTE: "Accounts" in "Settings" will disappear!! You can NOT setup your accounts!! And can NOT set keyboard app!!
type Monkey struct {
	Port    int
	conn	net.Conn
}

func NewMonkey(b *LocalBot, port int) (*Monkey) {

	forwardCmd := fmt.Sprintf("forward tcp:%d tcp:%d", port, port)
	b.Adb(forwardCmd)

	// set env: "EXTERNAL_STORAGE=/data/local/tmp"
	monkeyCmd := fmt.Sprintf("monkey --port %d", port)
	go b.Shell(monkeyCmd) // in background


	addr := fmt.Sprintf("127.0.0.1:%d", port)

TRYCONN:
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		Vln(3, "[monkey][conn]err", err)
		time.Sleep(1000 * time.Millisecond)
		goto TRYCONN
	}
	m := Monkey{
		Port: port,
		conn: conn,
	}

	return &m
}

func (m *Monkey) Close() (err error){
	m.conn.Write([]byte("done"))
	return m.conn.Close()
}

func (m *Monkey) send(cmd string) (err error){
	_, err = m.conn.Write([]byte(cmd))
	return
}

func (m *Monkey) Tap(loc image.Point) (err error){
	str := fmt.Sprintf("tap %d %d\n", loc.X, loc.Y)
	err = m.send(str)
	return
}

func (m *Monkey) Text(in string) (err error){
	str := fmt.Sprintf("type %s\n", in)
	err = m.send(str)
	return
}

func (m *Monkey) Press(in string) (err error){
	str := fmt.Sprintf("press %s\n", in)
	err = m.send(str)
	return
}

func (m *Monkey) Key(in string, ty string) (err error){
	str := fmt.Sprintf("key %s %s\n", ty, in)
	err = m.send(str)
	return
}

func (m *Monkey) Touch(loc image.Point, ty string) (err error){
	str := fmt.Sprintf("touch %s %d %d\n", ty, loc.X, loc.Y)
	err = m.send(str)
	return
}


// input by cmd command
type CmdInput struct {
	bot Bot
}

func NewCmdInput(b Bot) (*CmdInput) {
	i := CmdInput {
		bot: b,
	}
	return &i
}

func (i *CmdInput) Click(loc image.Point) (err error){
	_, err = i.bot.Shell("input tap " + strconv.Itoa(loc.X) + " " + strconv.Itoa(loc.Y))
	return
}

func (i *CmdInput) SwipeT(p0,p1 image.Point, time int) (err error){
	_, err = i.bot.Shell("input swipe " + strconv.Itoa(p0.X) + " " + strconv.Itoa(p0.Y) + " " + strconv.Itoa(p1.X) + " " + strconv.Itoa(p1.Y) + " " + strconv.Itoa(time))
	return
}

func (i *CmdInput) Text(in string) (err error){
	_, err = i.bot.Shell("input text " + in)
	return
}

func (i *CmdInput) Keyevent(in string) (err error){
	_, err = i.bot.Shell("input keyevent " + in)
	return
}

func (i *CmdInput) KeyHome() (error){
	return i.bot.Keyevent("KEYCODE_HOME")
}

func (i *CmdInput) KeyBack() (error){
	return i.bot.Keyevent("KEYCODE_BACK")
}

func (i *CmdInput) KeySwitch() (error){
	return i.bot.Keyevent("KEYCODE_APP_SWITCH")
}

func (i *CmdInput) KeyPower() (error){
	return i.bot.Keyevent("KEYCODE_POWER")
}


