package adbbot

import (
	"fmt"
	"image"
	"net"
	"time"
//	"strconv"
)

type KeyAction int

const (
	KEY_UP      KeyAction = -1
	KEY_MV      KeyAction =  0
	KEY_DOWN    KeyAction =  1
)

var actmap = map[KeyAction]string {
	-1: "up",
	0: "move",
	1: "down",
}

type Input interface {
	Click(loc image.Point) (err error)
	SwipeT(p0,p1 image.Point, time int) (err error)

	Touch(loc image.Point, ty KeyAction) (err error)
	Key(in string, ty KeyAction) (err error)

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
	Port      int
	KeyDelta  time.Duration

	bot       Bot
	conn	  net.Conn
	die       chan struct{}
	started   chan struct{}
	restart   chan struct{}
}

func NewMonkey(b *LocalBot, port int) (*Monkey) {

	m := Monkey{
		Port: port,
		KeyDelta: 100 * time.Millisecond,

		bot: b,
		die: make(chan struct{}),
		started: make(chan struct{}, 1),
		restart: make(chan struct{}, 1),
	}

	go m.wdt()

	<-m.started // wait connection ok

	return &m
}

func (m *Monkey) wdt() {
	for {
		select {
		case <-m.die:
			return
		default:
		}

		ok := m.tryStart()
		if !ok {
			time.Sleep(1500 * time.Millisecond)
			continue
		}
		time.Sleep(500 * time.Millisecond)

		ok = m.tryConn()
		if !ok {
			time.Sleep(1500 * time.Millisecond)
			continue
		}

		<-m.restart
		Vln(3, "[monkey][restart]")
	}
}

func (m *Monkey) tryStart() bool {
	forwardCmd := fmt.Sprintf("forward tcp:%d tcp:%d", m.Port, m.Port)
	m.bot.Adb(forwardCmd)

	// TODO: set env: "EXTERNAL_STORAGE=/data/local/tmp"
	monkeyCmd := fmt.Sprintf("monkey --port %d", m.Port)
	_, err := m.bot.ShellPipe(nil, monkeyCmd, false)
	if err != nil {
		Vln(3, "[monkey][start]err", err)
		return false
	}
	//cmd.Wait()
	//Vln(3, "[monkey][exit]")
	return true
}

func (m *Monkey) tryConn() bool {
	addr := fmt.Sprintf("127.0.0.1:%d", m.Port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		Vln(3, "[monkey][conn]err", err)
		return false
	}

	Vln(5, "[monkey][new]", m.conn, conn)
	m.conn = conn

	select {
	case m.started <- struct{}{}:
	default:
	}

	return true
}

func (m *Monkey) Close() (err error) {
	select {
	case <-m.die:
	default:
		close(m.die)
	}

	m.conn.Write([]byte("done"))
	return m.conn.Close()
}

func (m *Monkey) send(cmd string) (err error) {
	_, err = m.conn.Write([]byte(cmd))
	Vln(4, "[monkey][send]", cmd, err)
	if err != nil {
		m.restart <- struct{}{}
	}
	return
}

func (m *Monkey) Tap(loc image.Point) (err error) {
	loc = m.bot.Remap(loc)
	str := fmt.Sprintf("tap %d %d\n", loc.X, loc.Y)
	err = m.send(str)
	return
}

func (m *Monkey) Text(in string) (err error) {
	str := fmt.Sprintf("type %s\n", in)
	err = m.send(str)
	return
}

func (m *Monkey) Press(in string) (err error) {
	str := fmt.Sprintf("press %s\n", in)
	err = m.send(str)
	return
}

func (m *Monkey) Touch(loc image.Point, ty KeyAction) (err error) {
	loc = m.bot.Remap(loc)
	str := fmt.Sprintf("touch %s %d %d\n", actmap[ty], loc.X, loc.Y)
	err = m.send(str)
	return
}

func (m *Monkey) Key(in string, ty KeyAction) (err error) {
	str := fmt.Sprintf("key %s %s\n", actmap[ty], in)
	err = m.send(str)
	return
}

func (m *Monkey) Click(loc image.Point) (err error) {
	return m.Tap(loc)
/*	err = m.Touch(loc, KEY_DOWN)
	if err != nil {
		return
	}
	time.Sleep(m.KeyDelta)
	return m.Touch(loc, KEY_UP)*/
}

// blocking
func (m *Monkey) SwipeT(p0,p1 image.Point, dtime int) (err error) {
	if dtime <= 0 {
		dtime = 300
	}
	start := time.Now()
	dur := time.Duration(dtime) * time.Millisecond

	err = m.Touch(p0, KEY_DOWN)
	if err != nil {
		return
	}

	pt := image.Pt(0, 0)
	pd := image.Pt(p1.X - p0.X, p1.Y - p0.Y)
	esp := time.Since(start)
	for esp < dur {
		alpha := float64(esp) / float64(dur)
		pt.X = p0.X + int(float64(pd.X) * alpha)
		pt.Y = p0.Y + int(float64(pd.Y) * alpha)

		err = m.Touch(pt, KEY_MV)
		if err != nil {
			return
		}
		time.Sleep(1 * time.Millisecond)

		esp = time.Since(start)
	}

	err = m.Touch(p1, KEY_UP)
	return
}

// blocking
func (m *Monkey) Keyevent(in string) (err error) {
	err = m.Key(in, KEY_DOWN)
	if err != nil {
		return
	}
	time.Sleep(m.KeyDelta)
	return m.Key(in, KEY_UP)
}

func (m *Monkey) KeyHome() (error) {
	return m.Keyevent("KEYCODE_HOME")
}

func (m *Monkey) KeyBack() (error) {
	return m.Keyevent("KEYCODE_BACK")
}

func (m *Monkey) KeySwitch() (error) {
	return m.Keyevent("KEYCODE_APP_SWITCH")
}

func (m *Monkey) KeyPower() (error) {
	return m.Keyevent("KEYCODE_POWER")
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

func (i *CmdInput) Click(loc image.Point) (err error) {
	loc = i.bot.Remap(loc)
	str := fmt.Sprintf("input tap %d %d", loc.X, loc.Y)
	_, err = i.bot.Shell(str)
//	_, err = i.bot.Shell("input tap " + strconv.Itoa(loc.X) + " " + strconv.Itoa(loc.Y))
	return
}

func (i *CmdInput) SwipeT(p0,p1 image.Point, time int) (err error) {
	p0, p1 = i.bot.Remap(p0), i.bot.Remap(p1)
	str := fmt.Sprintf("input swipe %d %d %d %d %d", p0.X, p0.Y, p1.X, p1.Y, time)
	_, err = i.bot.Shell(str)
	return
}

func (i *CmdInput) Touch(loc image.Point, ty KeyAction) (err error) {
	return ErrNotSupport
}

func (i *CmdInput) Key(in string, ty KeyAction) (err error) {
	return ErrNotSupport
}

func (i *CmdInput) Text(in string) (err error) {
	str := fmt.Sprintf("input text \"%s\"", in)
	_, err = i.bot.Shell(str)
	return
}

func (i *CmdInput) Keyevent(in string) (err error) {
	_, err = i.bot.Shell("input keyevent " + in)
	return
}

func (i *CmdInput) KeyHome() (error) {
	return i.bot.Keyevent("KEYCODE_HOME")
}

func (i *CmdInput) KeyBack() (error) {
	return i.bot.Keyevent("KEYCODE_BACK")
}

func (i *CmdInput) KeySwitch() (error) {
	return i.bot.Keyevent("KEYCODE_APP_SWITCH")
}

func (i *CmdInput) KeyPower() (error) {
	return i.bot.Keyevent("KEYCODE_POWER")
}


