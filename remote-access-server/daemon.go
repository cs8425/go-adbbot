// go build -o daemon daemon.go
package main

import (
	"flag"
	"log"
	"net"
	"runtime"
	"time"

	"../adbbot"
)

var (
	verbosity = flag.Int("v", 3, "verbosity")
	ADB       = flag.String("adb", "adb", "adb exec path")
	DEV       = flag.String("dev", "", "select device")

	OnDevice = flag.Bool("od", false, "run on device")
	compress = flag.Bool("comp", false, "compress connection")

	bindAddr = flag.String("l", ":6900", "")

	reflash = flag.Int("r", 1000, "update screen minimum time (ms)")

	scale = flag.Float64("scale", 1.0, "screen resize after capture")
)

func main() {

	log.SetFlags(log.Ldate | log.Ltime)
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	adbbot.Verbosity = *verbosity
	bot := adbbot.NewLocalBot(*DEV, *ADB)

	// run on android by adb user(shell)
	bot.IsOnDevice = *OnDevice

	bot.SetScale(*scale)

	Vln(2, "[adb]", "wait-for-device")
	_, err := bot.Adb("wait-for-device")
	if err != nil {
		Vln(1, "[adb] err", err)
		return
	}

	m := adbbot.NewMonkey(bot, 1080)
	defer m.Close()
	bot.Input = m

	ln, err := net.Listen("tcp", *bindAddr)
	if err != nil {
		Vln(1, "[Daemon]Error listening:", err)
		return
	}
	Vln(1, "daemon start at", *bindAddr)

	// go screencap(bot)

	daemon, err := adbbot.NewDaemon(ln, bot, *compress)
	if err != nil {
		Vln(1, "[Daemon]Start Error:", err)
		return
	}
	defer daemon.Close()
	daemon.Listen()
}

func screencap(bot adbbot.Bot) {
	limit := time.Duration(*reflash) * time.Millisecond

	for {
		start := time.Now()
		err := bot.TriggerScreencap()
		if err != nil {
			return
		}
		Vln(4, "[screen][trigger]", time.Since(start))

		if time.Since(start) < limit {
			time.Sleep(limit - time.Since(start))
		}
	}
}

func Vln(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Println(v...)
	}
}
