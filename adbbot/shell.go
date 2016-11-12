package adbbot

import (
	"log"
	"os/exec"
	"sync"
	"strings"
)

var Verbosity = 3

func Cmd(cmd string) ([]byte, error) {
	Vlogln(4, "command: ", cmd)

	// splitting head => g++ parts => rest of the command
	parts := strings.Fields(cmd)
	head := parts[0]
	parts = parts[1:len(parts)]

	return exec.Command(head,parts...).Output()
}

func CmdArg(cmd ...string) ([]byte, error) {
	Vlogln(4, "command: ", cmd)

	return exec.Command(cmd[0], cmd[1:]...).Output()
}

func Cmds(x []string) {
	wg := new(sync.WaitGroup)
	wg.Add(len(x))

	for _, cmdstr := range x {
		go cmd_wg(cmdstr, wg)
	}

	wg.Wait()
}

func cmd_wg(cmd string, wg *sync.WaitGroup) {
	Vlogln(3, "command: ", cmd)

	parts := strings.Fields(cmd)
	head := parts[0]
	parts = parts[1:len(parts)]

	out, err := exec.Command(head, parts...).Output()
	if err != nil {
		Vlogf(3, "%s", err)
	}
	Vlogf(3, "%s", out)
	wg.Done()
}

func Vlogf(level int, format string, v ...interface{}) {
	if level <= Verbosity {
		log.Printf(format, v...)
//		fmt.Printf(format, v...)
	}
}
func Vlog(level int, v ...interface{}) {
	if level <= Verbosity {
		log.Print(v...)
//		fmt.Print(v...)
	}
}
func Vlogln(level int, v ...interface{}) {
	if level <= Verbosity {
		log.Println(v...)
//		fmt.Println(v...)
	}
}

