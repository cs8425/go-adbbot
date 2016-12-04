package main

import (
	"flag"
	"log"
	"runtime"
	"time"

	"./adbbot"
)

var verbosity = flag.Int("v", 3, "verbosity")
var ADB = flag.String("adb", "adb", "adb exec path")
var DEV = flag.String("dev", "", "select device")

var TMPL = flag.String("t", "tmpl.png", "template image")
var IN = flag.String("i", "", "input image")

var sizeX = flag.Int("sizeX", 448, "sizeX")
var sizeY = flag.Int("sizeY", 816, "sizeY")

func main() {

	log.SetFlags(log.Ldate|log.Ltime)
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	adbbot.Verbosity = *verbosity
	bot := adbbot.NewBot(*DEV, *ADB)
	if *sizeX > 0 && *sizeY > 0{
		bot.ScriptScreen(0,0, *sizeX, *sizeY)
	}


	t := time.Now()
	Vlogln(3, "[now]", t.Format("20060102_150405"), *ADB, *DEV)

	// create matching template
	tmpl, err := bot.NewTmpl(*TMPL, bot.NewRectAll())
	if err != nil {
		Vlogln(2, "load template image err", err)
	} else {

		if *IN == "" {

			// try to find target once
			x, y, val := bot.FindExistReg(tmpl, 2, 0)
			if x == -1 && y == -1 {
				Vlogln(2, "template not found", x, y, val)
			} else {
				Vlogln(2, "template found at", x, y, val)
			}

		} else {

			input, err := adbbot.OpenImage(*IN)
			if err != nil {
				Vlogln(2, "load input image err", err)
			}
			x, y, val := adbbot.FindP(input, tmpl.Image)
			if x == -1 && y == -1 {
				Vlogln(2, "template not found", x, y, val)
			} else {
				Vlogln(2, "template found at", x, y, val)
			}
		}

	}

}

func Vlogf(level int, format string, v ...interface{}) {
	if level <= *verbosity {
		log.Printf(format, v...)
//		fmt.Printf(format, v...)
	}
}
func Vlog(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Print(v...)
//		fmt.Print(v...)
	}
}
func Vlogln(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Println(v...)
//		fmt.Println(v...)
	}
}

