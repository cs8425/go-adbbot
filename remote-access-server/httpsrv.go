// go build -o httpsrv httpsrv.go packet.go
package main

import (
	"io"
//	"io/ioutil"
	"net"
	"net/http"
//	"syscall"

	"log"
	"runtime"
	"time"
	"flag"

//	"fmt"
	"strconv"
	"strings"

	"./websocket"
)

var localAddr = flag.String("l", ":5800", "")
var daemonAddr = flag.String("t", "127.0.0.1:6900", "")

var verbosity = flag.Int("v", 3, "verbosity")

type OP struct {
	Type      int	// 0 >> Key, 1 >> Click, 2 >> Swipe
	Op        string
	X0        int
	Y0        int
	X1        int
	Y1        int
	Dt        int
}

var upgrader = websocket.Upgrader{} // use default options

func ws(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		Vln(2, "upgrade:", err)
		return
	}
	defer c.Close()

	newclients <- c

	for {
		mt, msgb, err := c.ReadMessage()
		if err != nil {
			Vln(2, "read:", err)
			break
		}
		msg := string(msgb)
		Vln(4, "[recv]", mt, msg)
		lines := strings.Split(msg, "\n")
		if len(lines) < 2 {
			continue
		}
		todo := lines[0]

		switch todo {
		case "key": // home, back, task, power
			t := OP {
				Type: 0,
				Op: lines[1],
			}
			op <- t

		case "move":
			mvs(lines[1:])

		case "click":
			d := strings.Split(lines[1], ",")
			x, err := strconv.ParseInt(d[0], 10, 32)
			if err != nil {
				return
			}
			y, err := strconv.ParseInt(d[1], 10, 32)
			if err != nil {
				return
			}
			t := OP {
				Type: 1,
				X0: int(x),
				Y0: int(y),
			}
			Vln(3, "[click]", t)
			op <- t
		default:
			Vln(3, "[undef]", todo)
		}
	}
}

func mvs(mvs []string) {
	Vln(3, "[mvs]", mvs)

	var x0, y0, dt int64
	for idx, line := range mvs {
		d := strings.Split(line, ",")
		x1, err := strconv.ParseInt(d[0], 10, 32)
		if err != nil {
			return
		}
		y1, err := strconv.ParseInt(d[1], 10, 32)
		if err != nil {
			return
		}
		dt, err = strconv.ParseInt(d[2], 10, 32)
		if err != nil {
			return
		}
		t := OP {
			Type: 2,
			X0: int(x0),
			Y0: int(y0),
			X1: int(x1),
			Y1: int(y1),
			Dt: int(dt),
		}
		x0, y0 = x1, y1

		if idx > 0 {
			op <- t
		}
	}
}

func keys(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, html)
}

var newclients chan *websocket.Conn
var screen chan []byte
func broacast() {
	newclients = make(chan *websocket.Conn, 16)
	screen = make(chan []byte, 1)
	clients := make(map[*websocket.Conn]*websocket.Conn, 0)

	for {
		img := <- screen
		for i, c := range clients {
			err := c.WriteMessage(websocket.BinaryMessage, img)
			if err != nil {
				Vln(2, "write:", err)
				c.Close()
				delete(clients, i)
			}
		}
		for len(newclients) > 0 {
			client := <-newclients
			clients[client] = client
		}
	}
}

func pollimg(daemon net.Conn) {
	var err error
	var buf []byte

	WriteTagStr(daemon, "poll")
	conn := NewCompStream(daemon, 1)
//	conn := NewFlateStream(daemon, 1)
	for {
		start := time.Now()
		buf, err = ReadVTagByte(conn)
		if err != nil {
			Vln(2, "error poll screen", err)
			return
		}
		Vln(3, "poll screen ok", len(buf), time.Since(start))

		screen <- buf
	}
}

var op chan OP
func pushop(daemon net.Conn) {
	var err error
	op = make(chan OP, 4)

	for {
		todo := <- op

		switch todo.Type {
		case 0:
			err = WriteTagStr(daemon, "Key")
			if err != nil {
				Vln(2, "error send key req", err)
				return
			}

			err = WriteTagStr(daemon, todo.Op)
			if err != nil {
				Vln(2, "error send op req", err)
				return
			}
		case 1:
			err = WriteTagStr(daemon, "Click")
			if err != nil {
				Vln(2, "error send Click req", err)
				return
			}
			WriteVLen(daemon, int64(todo.X0))
			WriteVLen(daemon, int64(todo.Y0))
		case 2:
			err = WriteTagStr(daemon, "Swipe")
			if err != nil {
				Vln(2, "error send Click req", err)
				return
			}
			WriteVLen(daemon, int64(todo.X0))
			WriteVLen(daemon, int64(todo.Y0))
			WriteVLen(daemon, int64(todo.X1))
			WriteVLen(daemon, int64(todo.Y1))
			WriteVLen(daemon, int64(todo.Dt))
		}
	}
}
func main() {
	log.SetFlags(log.Ldate|log.Ltime)
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	Vf(1, "server Listen @ %v\n", *localAddr)

	poll, err := net.Dial("tcp", *daemonAddr)
	if err != nil {
		Vln(1, "error connct to", *daemonAddr)
		return
	}
	go pollimg(poll)

	conn, err := net.Dial("tcp", *daemonAddr)
	go pushop(conn)
	Vln(1, "connct", *daemonAddr, "ok!")

	go broacast()

	http.HandleFunc("/ws", ws)
	http.HandleFunc("/", keys)
	http.ListenAndServe(*localAddr, nil)
	
}

func readXY(p1 net.Conn) (x, y int, err error) {
	var x0, y0 int64
	x0, err = ReadVLen(p1)
	if err != nil {
		Vln(2, "[x]err", err)
		return
	}
	y0, err = ReadVLen(p1)
	if err != nil {
		Vln(2, "[y]err", err)
		return
	}
	return int(x0), int(y0), nil
}

func Vln(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Println(v...)
	}
}
func Vf(level int, format string, v ...interface{}) {
	if level <= *verbosity {
		log.Printf(format, v...)
	}
}

var html = `<!doctype html>
<head>
<title>adbbot</title>
<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=no">
<script src="//code.jquery.com/jquery-3.1.0.min.js"></script>
<style>
html, body {
	height: 100%;
	margin: 0px;
}
body > div {
	float: left;
	width: 50%;
	height: 10%;
}
#screen {
    width: auto;
    height: 85vh;
}
#btns {
	margin: 8px;
}
button {
	border: 1px solid gray;
	padding: 5px 10px;
	border-radius: 5px;
	background-color: #efefef;
}
</style>
</head>
<body>
	<div>
		<div id="btns">
			<button id="back">◁</button>
			<button id="home">◯</button>
			<button id="task">▢</button>
			<button id="power">⏻⏼</button>
		</div>
		<img id="screen" />
		<canvas id="screen_holder"></canvas>
	</div>
</body>
<script type="text/javascript">
var bindlist = ['home', 'back', 'task', 'power'];
for(idx in bindlist){
	var ele = bindlist[idx];
	(function(ele){
		$('#' + ele).bind('click', function(e){
//			$.get('/' + ele)
			send('key', ele)
		});
	})(ele);
}

var pand = function(num) {
    return (num < 10) ? '0' + num : num + '';
}

var now = function() {
    var t = new Date();
    var out = '[';
    out += t.getFullYear();
    out += '/' + pand(t.getMonth() + 1);
    out += '/' + pand(t.getDate());
    out += ' ' + pand(t.getHours());
    out += ':' + pand(t.getMinutes());
    out += ':' + pand(t.getSeconds()) + ']';
    return out;
}


var pos = {}
function getXY(e) {
	var x,y;
	if(typeof e.touches != 'undefined'){
//console.log('getXY()', e.touches, e.touches[0])
		if(e.touches.length == 0) return [pos.x, pos.y];
		var t = e.touches[0]
		var offsetX = t.pageX - img.offsetLeft
		var offsetY = t.pageY - img.offsetTop
		x = offsetX * scale;
		y = offsetY * scale;
	}else{
		x = e.offsetX * scale;
		y = e.offsetY * scale;
	}
    return [x,y];
}

function send(type, data) {
	if(!ws) return

	var out = type + '\n' + data
	ws.send(out)
}


var isdrag = false;
var t = null;
var queue = [];
var delaypost = null;
var mousemove = function(){
//	delaypost = setTimeout(mousemove, 50);
	if(queue.length == 0) return;
	var out = '';
	for(var i=0; i<queue.length; i++){
		var dx = queue[i][0];
		var dy = queue[i][1];
		var dt = queue[i][2];
		out += Math.round(dx) + ',' + Math.round(dy) + ',' + Math.round(dt) + '\n';
	}
//	console.log('move', out);
//	$.post('/move', out, null, 'text');
	send('move', out)
	queue = [];
}

var img = document.querySelector('#screen')
$('#screen').bind('mousedown touchstart', function(e){
	e.preventDefault()
	isdrag = true;
	t = new Date();

	var xy = getXY(e)
	var x = xy[0]
	var y = xy[1]
	pos.x = x
	pos.y = y

	queue.push([x, y, 0]);

}).bind('mouseup touchend', function(e){
	e.preventDefault()
	isdrag = false;
	var dt = (new Date()) - t

	var xy = getXY(e)
	var x = xy[0]
	var y = xy[1]

	queue.push([x, y, dt])
	mousemove()

	if(dt > 120) {
//		queue.push([x, y, dt])
//		mousemove()
//		if(!delaypost) delaypost = setTimeout(mousemove, 50);
	} else {
		queue = []
		console.log('click', x, y)
		var out = x + ',' + y
//		$.post('/click', out, null, 'text');
	}
}).bind('mousemove touchmove', function(e){
	if(!isdrag) return;
	e.preventDefault()
	var xy = getXY(e)
	var x = xy[0]
	var y = xy[1]
	pos.x = x
	pos.y = y
/*	queue.push([x, y]);

	if(!delaypost) delaypost = setTimeout(mousemove, 50);*/
}).bind('click', function(e){
	e.preventDefault()
//	if(((new Date()) - t) > 120) return

	var xy = getXY(e)
	var x = xy[0]
	var y = xy[1]

console.log('click1', x, y, e)


//	var out = x + ',' + y
//	$.post('/click', out, null, 'text');
});

var scale = 1.0
var ws;
$(document).ready(function(e) {

//	var ws;
	var img = document.querySelector('#screen')
	var urlCreator = window.URL || window.webkitURL
	var createObjectURL = urlCreator.createObjectURL
	var revokeObjectURL = urlCreator.revokeObjectURL

	var data;
	img.onload = function(e) {
		var img = e.target
		var url = img.src
		scale = img.naturalWidth / img.width
		revokeObjectURL(url)
//		console.log(now(), 'Freeing blob...', url)
		data = null
	};

	function open() {
		if (ws) {
			return false
		}
		ws = new WebSocket('ws://'+window.location.host+'/ws');
		ws.onopen = function(e) {
			console.log("OPEN", e)
		}
		ws.onclose = function(e) {
			console.log("CLOSE", e)
			ws = null;
			setTimeout(open, 2500)
		}
		ws.onmessage = function(e) {
			// console.log("RESPONSE", e)
			// display screen
			if (data) {
//				img.src = ''
				console.log(now(), "multiload!!", data)
				revokeObjectURL(data)
				data = null
			}
			data = createObjectURL( e.data )
			img.src = data
//			console.log(now(), 'New screen', data)

		}
		ws.onerror = function(e) {
			console.log("ERROR", e)
		}
	};

	open()
})

</script>
</html>`

