package main

import (
	"flag"
	"log"
	"runtime"
	"time"

	ab "./adbbot"
)

var verbosity = flag.Int("v", 3, "verbosity")
var ADB = flag.String("adb", "adb", "adb exec path")
var DEV = flag.String("dev", "", "select device")

var TMPL = flag.String("t", "tmpl.png", "template image")
var IN = flag.String("i", "", "input image")

var sizeX = flag.Int("sizeX", 720, "sizeX")
var sizeY = flag.Int("sizeY", 1280, "sizeY")

func main() {

	log.SetFlags(log.Ldate|log.Ltime)
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	ab.Verbosity = *verbosity

	t := time.Now()
	Vln(3, "[now]", t.Format("20060102_150405"), *ADB, *DEV)


	// create matching template
	tmpl, err := ab.NewTmpl(*TMPL, ab.RectAll)
	if err != nil {
		Vln(2, "load template image err", err)
		return
	}

	if *IN != "" {
		var x, y int
		var val float64

		input, err := ab.OpenImage(*IN)
		if err != nil {
			Vln(2, "load input image err", err)
		}

		x, y, val = ab.FindP(input, tmpl.Image)
		x, y, val = ab.FindP2(input, tmpl.Image)
		x, y, val = ab.FindP(input, tmpl.Image)
		x, y, val = ab.FindP2(input, tmpl.Image)
		if x == -1 && y == -1 {
			Vln(2, "template not found", x, y, val)
		} else {
			Vln(2, "template found at", x, y, val)
		}

		x, y, val = ab.Find(input, tmpl.Image)
		if x == -1 && y == -1 {
			Vln(2, "template not found", x, y, val)
		} else {
			Vln(2, "template found at", x, y, val)
		}

		return
	}

	bot := ab.NewLocalBot(*DEV, *ADB)
	if *sizeX > 0 && *sizeY > 0 {
		bot.ScriptScreen(0,0, *sizeX, *sizeY)
	}

	// try to find target once
	x, y, val := ab.FindExistReg(bot, tmpl, 2, 0)
	if x == -1 && y == -1 {
		Vln(2, "template not found", x, y, val)
	} else {
		Vln(2, "template found at", x, y, val)
	}

}

func Vf(level int, format string, v ...interface{}) {
	if level <= *verbosity {
		log.Printf(format, v...)
	}
}
func Vln(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Println(v...)
	}
}

