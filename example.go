package main

import (
	"flag"
	"log"
	"runtime"
	"time"

	"./adbbot"
)

var verbosity = flag.Int("v", 2, "verbosity")
var ADB = flag.String("adb", "adb", "adb exec path")
var DEV = flag.String("dev", "", "select device")

var OnDevice = flag.Bool("od", true, "run on device")

var APP = flag.String("app", "com.android.vending", "app package name")
var TMPL = flag.String("tmpl", "tmpl.png", "template image")

func main() {

	log.SetFlags(log.Ldate|log.Ltime)
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	adbbot.Verbosity = *verbosity
	bot := adbbot.NewLocalBot(*DEV, *ADB)

	// run on android by adb user(shell)
	bot.IsOnDevice = *OnDevice

	Vlogln(2, "[adb]", "wait-for-device")
	_, err := bot.Adb("wait-for-device")
	if err != nil {
		Vlogln(1, "adb err", err)
	}

	// press Home key
	bot.KeyHome()

	// start APP
	bot.StartApp(*APP)

	// wait
	time.Sleep(time.Millisecond * 10000)


	// create matching region between Point <100,635> and <9999,9999>
	//reg := bot.NewRectAbs(100, 635, 9999, 9999)

	// or All the screen (slow)
	reg := bot.NewRectAll()

	// create matching template
	tmpl, err := bot.NewTmpl(*TMPL, reg)
	if err != nil {
		Vlogln(2, "load template image err", err)
	} else {

		// try to find target
		// 10 times with 1000ms delay between each search
		x, y, val := bot.FindExistReg(tmpl, 10, 1000)
		if x == -1 && y == -1 {
			Vlogln(2, "template not found", x, y, val)
		} else {
			Vlogln(2, "template found at", x, y, val)
		}

	}

	infoname := time.Now().Format("20060102_150405")
	err = bot.SaveScreen(infoname + ".png")
	if err != nil {
		Vlogln(2, "SaveScreen err", err)
	} else {
		Vlogln(2, "SaveScreen as file ", infoname + ".png")
	}

	// force-stop APP
	bot.KillApp(*APP)

}

func Vlogln(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Println(v...)
	}
}

