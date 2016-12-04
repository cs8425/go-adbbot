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

var OUT = flag.String("o", "", "output")


func main() {

	log.SetFlags(log.Ldate|log.Ltime)
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	adbbot.Verbosity = *verbosity
	bot := adbbot.NewBot(*DEV, *ADB)

	_, err := bot.Adb("wait-for-device")
	if err != nil {
		Vlogln(1, "adb err", err)
	}

	t := time.Now()
	Vlogln(3, "[now]", t.Format("20060102_150405"), *ADB, *DEV)
	capfile := *OUT
	if capfile == "" {
		capfile = t.Format("20060102_150405") + ".png"
	} else {
		capfile = capfile + ".png"
	}
	Vlogln(3, "saveInfo", capfile)
	err = bot.SaveScreen(capfile)
	if err != nil {
		Vlogln(3, "SaveScreen err", err)
	}
/*	img, _ := bot.Screencap()
//	img = img.(*image.NRGBA).SubImage(bot.NewRect(0, 180, 448, 95))
	img = img.(*image.NRGBA).SubImage(bot.NewRect(0, 0, 448, 275))
	err := adbbot.SaveImage(img, capfile)
	if err != nil {
		Vlogln(3, "SaveScreen err", err)
		return false
	}*/

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

