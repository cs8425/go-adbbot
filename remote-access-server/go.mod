module main

go 1.18

replace local/adbbot => ../adbbot

require (
	github.com/gorilla/websocket v1.5.0
	local/adbbot v0.0.0-00010101000000-000000000000
)

require github.com/golang/snappy v0.0.4 // indirect
