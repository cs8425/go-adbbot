// go build -o httpsrv httpsrv.go
package main

import (
	"io"
	"net"
	"net/http"
	"sync/atomic"

	"flag"
	"log"
	"runtime"
	"time"

	// "fmt"
	"strconv"
	"strings"

	"image"

	"local/adbbot"

	"github.com/gorilla/websocket"
)

var (
	localAddr  = flag.String("l", ":5800", "")
	daemonAddr = flag.String("t", "127.0.0.1:6900", "")

	wsComp   = flag.Bool("wscomp", false, "ws compression")
	compress = flag.Bool("comp", false, "compress connection")

	reflash = flag.Int("r", 1000, "update screen minimum time (ms)")

	verbosity = flag.Int("v", 3, "verbosity")
)

type OP struct {
	Type int // 0 >> Key, 1 >> touch
	Op   string
	X0   int
	Y0   int
	Ev   int
}

var upgrader = websocket.Upgrader{EnableCompression: false} // use default options

func ws(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		Vln(2, "upgrade:", err)
		return
	}
	defer c.Close()

	newclients <- c

	atomic.AddInt32(&clientCount, int32(1))
	defer atomic.AddInt32(&clientCount, int32(-1))

	select {
	case pollNotify <- struct{}{}:
	default:
	}

	for {
		mt, msgb, err := c.ReadMessage()
		if err != nil {
			Vln(2, "read:", err)
			break
		}
		msg := string(msgb)
		Vln(5, "[recv]", mt, msg)
		lines := strings.Split(msg, "\n")
		if len(lines) < 2 {
			continue
		}
		todo := lines[0]
		Vln(4, "[lines]", lines)

		switch todo {
		case "key": // home, back, task, power
			ev, err := strconv.ParseInt(lines[2], 10, 32)
			if err != nil {
				continue
			}
			t := OP{
				Type: 0,
				Op:   lines[1],
				Ev:   int(ev),
			}
			Vln(3, "[key]", t)
			op <- t

		case "move":
			mvs(lines[1:])

		default:
			Vln(3, "[undef]", todo)
		}
	}
}

func mvs(mvs []string) {
	Vln(4, "[mvs]", mvs)

	var x, y, ev int64
	var err error
	for _, line := range mvs {
		d := strings.Split(line, ",")
		x, err = strconv.ParseInt(d[0], 10, 32)
		if err != nil {
			return
		}
		y, err = strconv.ParseInt(d[1], 10, 32)
		if err != nil {
			return
		}
		ev, err = strconv.ParseInt(d[2], 10, 32)
		if err != nil {
			return
		}

		t := OP{
			Type: 1,
			X0:   int(x),
			Y0:   int(y),
			Ev:   int(ev),
		}
		Vln(5, "[mv]", t)
		op <- t
	}
}

func keys(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, html)
}

type Wsclient struct {
	*websocket.Conn
	data chan []byte
}

func (c *Wsclient) Send(buf []byte) {
	select {
	case <-c.data:
	default:
	}
	c.data <- buf
}
func (c *Wsclient) worker() {
	for {
		buf := <-c.data
		err := c.WriteMessage(websocket.BinaryMessage, buf)
		if err != nil {
			c.Close()
			return
		}
	}
}

var newclients chan *websocket.Conn
var screen chan []byte

func broacast() {
	newclients = make(chan *websocket.Conn, 16)
	screen = make(chan []byte, 1)
	clients := make(map[*Wsclient]*Wsclient, 0)

	for {
		img := <-screen
		for _, c := range clients {
			c.Send(img)
		}
		for len(newclients) > 0 {
			client := <-newclients
			c := &Wsclient{client, make(chan []byte, 1)}
			go c.worker()
			clients[c] = c
			Vln(3, "[new client]", client.RemoteAddr())
		}
	}
}

var clientCount int32 = 0
var pollNotify = make(chan struct{}, 1)

func pollimg(bot adbbot.Bot) {
	var err error
	var buf []byte

	limit := time.Duration(*reflash) * time.Millisecond

	for {
		if atomic.LoadInt32(&clientCount) == 0 { // block untill have any client
			<-pollNotify
		}

		start := time.Now()
		err = bot.TriggerScreencap()
		if err != nil {
			Vln(2, "[screen][trigger]err", err)
			return
		}
		Vln(4, "[screen][trigger]", time.Since(start))

		buf, err = bot.PullScreenByte()
		if err != nil {
			Vln(2, "[screen][pull]err", err)
			return
		}
		Vln(4, "[screen][pull]", len(buf), time.Since(start))

		select {
		case <-screen:
		default:
		}
		screen <- buf

		if time.Since(start) < limit {
			time.Sleep(limit - time.Since(start))
		}
	}
}

var op chan OP

func pushop(bot adbbot.Bot) {
	op = make(chan OP, 4)

	var evmap = map[int]adbbot.KeyAction{
		-1: adbbot.KEY_UP,
		0:  adbbot.KEY_MV,
		1:  adbbot.KEY_DOWN,
	}

	var keymap = map[string]string{
		"home":  "KEYCODE_HOME",
		"back":  "KEYCODE_BACK",
		"task":  "KEYCODE_APP_SWITCH",
		"power": "KEYCODE_POWER",
	}

	for {
		todo := <-op

		switch todo.Type {
		case 0:
			keycode, ok := keymap[todo.Op]
			if !ok {
				continue
			}

			ty, ok := evmap[todo.Ev]
			if !ok {
				continue
			}
			bot.Key(keycode, ty)

		case 1:
			ty, ok := evmap[todo.Ev]
			if !ok {
				continue
			}
			bot.Touch(image.Pt(todo.X0, todo.Y0), ty)
		}
	}
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	// adbbot.Verbosity = *verbosity

	upgrader.EnableCompression = *wsComp
	Vf(1, "ws EnableCompression = %v\n", *wsComp)
	Vf(1, "server Listen @ %v\n", *localAddr)

	conn, err := net.Dial("tcp", *daemonAddr)
	if err != nil {
		Vln(1, "error connct to", *daemonAddr)
		return
	}

	bot, err := adbbot.NewRemoteBot(conn, *compress)
	if err != nil {
		Vln(1, "connct to", *daemonAddr, "err:", err)
		return
	}
	go pushop(bot)
	go pollimg(bot)
	Vln(1, "connct", *daemonAddr, "ok!")

	go broacast()

	http.HandleFunc("/ws", ws)
	http.HandleFunc("/", keys)
	http.ListenAndServe(*localAddr, nil)

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
	margin: .1em;
}
body > div {
	height: 10vh;
}
#screen {
	max-width: calc(100vw - 0.4em);
	max-height: 85vh;
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
	<img id="screen" />
	<div id="btns">
		<button id="back">◁</button>
		<button id="home">◯</button>
		<button id="task">▢</button>
		<button id="power">⏻⏼</button>
	</div>
</body>
<script type="text/javascript">
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

var bindlist = ['home', 'back', 'task', 'power'];
for(idx in bindlist){
	var ele = bindlist[idx];
	(function(ele){
		$('#' + ele).bind('mousedown touchstart', function(e){
			e.preventDefault()
			send('key', ele + '\n1')
		}).bind('mouseup touchend', function(e){
			e.preventDefault()
			send('key', ele + '\n-1')
		})
	})(ele);
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
	delaypost = setTimeout(mousemove, 50);
	if(queue.length == 0) return;
	var out = '';
	for(var i=0; i<queue.length; i++){
		var dx = queue[i][0];
		var dy = queue[i][1];
		var dt = queue[i][2];
		out += Math.round(dx) + ',' + Math.round(dy) + ',' + Math.round(dt) + '\n';
	}
//	console.log('move', out);
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

	queue.push([x, y, 1])

	if(!delaypost) delaypost = setTimeout(mousemove, 50)

}).bind('mouseup touchend', function(e){
	e.preventDefault()
	isdrag = false;
	var dt = (new Date()) - t

	var xy = getXY(e)
	var x = xy[0]
	var y = xy[1]
	pos.x = x
	pos.y = y

	queue.push([x, y, -1])

	if(!delaypost) delaypost = setTimeout(mousemove, 50)

}).bind('mousemove touchmove', function(e){
	if(!isdrag) return;
	e.preventDefault()
	var xy = getXY(e)
	var x = xy[0]
	var y = xy[1]
	pos.x = x
	pos.y = y
	queue.push([x, y, 0])

	if(!delaypost) delaypost = setTimeout(mousemove, 50)

})
$(document).bind('mouseup touchend', function(e){
	if(!isdrag) return;
	e.preventDefault()
	isdrag = false;

	queue.push([pos.x, pos.y, -1])

	if(!delaypost) delaypost = setTimeout(mousemove, 50)
})

var scale = 1.0
var ws;
$(document).ready(function(e) {

	var img = document.querySelector('#screen')
	var urlCreator = window.URL || window.webkitURL
	var createObjectURL = urlCreator.createObjectURL
	var revokeObjectURL = urlCreator.revokeObjectURL

	var lastFrame = null
	var updateFrame = function(){
		img.src = createObjectURL( lastFrame )
		lastFrame = null
	}

	img.onload = function(e) {
		var img = e.target
		var url = img.src
		scale = img.naturalWidth / img.width
		revokeObjectURL(url)
//		console.log(now(), 'Freeing blob...', url)
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

			if(!lastFrame) {
				requestAnimationFrame(updateFrame)
			}
			lastFrame = e.data

//			console.log(now(), 'New screen', lastFrame)
		}
		ws.onerror = function(e) {
			console.log("ERROR", e)
		}
	};

	open()
})

</script>
</html>`
