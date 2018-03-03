package adbbot

import (
	"image"
	"net"
)

type OP struct {
	Type      int	// 0 >> Key, 1 >> touch
	Op        string
	X0        int
	Y0        int
	Ev        int
}

type RemoteBot struct {
	Last_screencap  image.Image

	Screen          *image.Rectangle
	TargetScreen    *image.Rectangle

	width           int
	height          int

	conn            net.Conn
	Compress        bool
	FindOnDaemon    bool // TODO
	op chan OP

	Input
}

func NewRemoteBot(url string) (*RemoteBot) {

	conn, err := net.Dial("tcp", url)
	if err != nil {
		Vln(1, "error connct to", url)
		return nil
	}

	b := RemoteBot {
		conn: conn,
		op: make(chan OP, 4),
	}

	go b.pushworker()

	return &b
}

func (b *RemoteBot) pushworker() {
	var err error

	for {
		todo := <- op

		switch todo.Type {
		case 0:
			err = WriteTagStr(b.conn, "Key")
			if err != nil {
				Vln(2, "[send][Key]err", err, todo)
				return
			}
			WriteTagStr(b.conn, todo.Op)
			WriteVLen(b.conn, int64(todo.Ev))

		case 1:
			err = WriteTagStr(b.conn, "Touch")
			if err != nil {
				Vln(2, "[send][Touch]err", err, todo)
				return
			}
			WriteVLen(b.conn, int64(todo.X0))
			WriteVLen(b.conn, int64(todo.Y0))
			WriteVLen(b.conn, int64(todo.Ev))
		}
	}
}


