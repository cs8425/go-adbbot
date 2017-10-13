// go build -o httpsrv httpsrv.go packet.go
package main

import (
	"io"
	"io/ioutil"
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
	newclients <- c

/*	defer c.Close()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			Vln(2, "read:", err)
			break
		}
		Vf(3, "recv: %s", mt, message)
		op := string(message)
		lines := strings.Split(op, "\n")
		todo := lines[0]
	}*/
}

func mvs(data io.ReadCloser) {
	defer data.Close()

	body, err := ioutil.ReadAll(data)
	Vln(3, "[mvs]", string(body))
	if err != nil {
		return
	}

	var x0, y0, dt int64
	mvs := strings.Split(string(body), "\n")
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

var keymap map[string]string = map[string]string{
    "/home": "home",
    "/back": "back",
    "/task": "task",
    "/power": "power",
}
func keys(w http.ResponseWriter, r *http.Request) {
	key, ok := keymap[r.URL.Path]
	if ok {
		Vf(3, "got key: %v\n", r.URL.Path)
		t := OP {
			Type: 0,
			Op: key,
		}
		op <- t
		io.WriteString(w, r.URL.Path + ", ok")
	} else {
		switch r.URL.Path {
		case "/move":
			mvs(r.Body)

		case "/click":
			defer r.Body.Close()
			line, err := ioutil.ReadAll(r.Body)
			if err != nil {
				return
			}
			d := strings.Split(string(line), ",")
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
			io.WriteString(w, html)
		}
	}
}
/*
func keys(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, html)
}*/

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

/*	err = WriteTagStr(daemon, "ScreenSize")
	if err != nil {
		Vln(2, "error send ScreenSize req", err)
		return
	}
	scX, scY, err := readXY(daemon)
	if err != nil {
		Vln(2, "error decode ScreenSize resp", err)
		return
	}
	Vln(2, "[ScreenSize]", scX, scY)*/

	for {
		start := time.Now()
		err = WriteTagStr(daemon, "Screencap")
		if err != nil {
			Vln(2, "error send screencap req", err)
			return
		}
		buf, err = ReadVTagByte(daemon)
		if err != nil {
			Vln(2, "error poll screen", err)
			return
		}
		Vln(3, "poll screen ok", len(buf), time.Since(start))

		screen <- buf

		time.Sleep(2 * 1000 * time.Millisecond)
	}
}

func pollimg2(daemon net.Conn) {
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
	go pollimg2(poll)

	conn, err := net.Dial("tcp", *daemonAddr)
//	go pollimg(conn)
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
		$('#' + ele).bind('click', function(e){$.get('/' + ele)});
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

var scale = 1.0
function getXY(e) {
	var x,y;
	if(typeof e.touches != 'undefined'){
		x = e.touches[0].offsetX * scale;
		y = e.touches[0].offsetY * scale;
	}else{
		x = e.offsetX * scale;
		y = e.offsetY * scale;
	}
    return [x,y];
}


var isdrag = false;
var t = null;
var pos = {};
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
	$.post('/move', out, null, 'text');
	queue = [];
}

var img = document.querySelector('#screen')
$('#screen').bind('mousedown touchstart', function(e){
	isdrag = true;
	t = new Date();

	var xy = getXY(e)
	var x = xy[0]
	var y = xy[1]

	queue.push([x, y, 0]);

}).bind('mouseup touchend', function(e){
	isdrag = false;
	var dt = (new Date()) - t

	var xy = getXY(e)
	var x = xy[0]
	var y = xy[1]

	queue.push([x, y, dt])
	mousemove()

/*	if(dt > 120) {
		queue.push([x, y, dt])
//		if(!delaypost) delaypost = setTimeout(mousemove, 50);
		mousemove()
	} else {
		queue = []
		console.log('click', x, y)
		var out = x + ',' + y
		$.post('/click', out, null, 'text');
	}*/
}).bind('mousemove touchmove', function(e){
	if(!isdrag) return;
	e.preventDefault();
/*	var xy = getXY(e)
	var x = xy[0]
	var y = xy[1]
	queue.push([x, y]);

	if(!delaypost) delaypost = setTimeout(mousemove, 50);*/
}).bind('click', function(e){
	if(((new Date()) - t) > 120) return

	var xy = getXY(e)
	var x = xy[0]
	var y = xy[1]

/*	var x1 = e.offsetX ? (e.offsetX):(e.pageX-img.offsetLeft);
	var y1 = e.offsetY ? (e.offsetY):(e.pageY-img.offsetTop);
console.log('click2', x1, y1)*/
console.log('click', x, y, e)


//	var out = x + ',' + y
//	$.post('/click', out, null, 'text');
});

var dd,dd2
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
		var url = e.target.src
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
			ws.send('echo .... ');
		}
		ws.onclose = function(e) {
			console.log("CLOSE", e)
			ws = null;
			setTimeout(open, 1500)
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


/*			var fileReader = new FileReader()
			fileReader.onload = function() {
				var imgBuf = new Uint8Array(this.result)
				var cxtImg = ctx.getImageData(0, 0, W, H)
				var pix = cxtImg.data
				for(var i=0; i<pix.length; i++){
					pix[i] = imgBuf[i]
				}
				ctx.putImageData(cxtImg, 0, 0)
//				dd = pix
//				dd2 = imgBuf
			};
			fileReader.readAsArrayBuffer(e.data)*/
		}
		ws.onerror = function(e) {
			console.log("ERROR", e)
		}
	};

	open()
})

</script>
</html>`

