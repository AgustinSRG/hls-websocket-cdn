module github.com/AgustinSRG/hls-websocket-cdn/tester

go 1.22.0

replace github.com/AgustinSRG/hls-websocket-cdn/client-publisher => ../client-publisher

require (
	github.com/AgustinSRG/genv v1.0.0
	github.com/AgustinSRG/glog v1.0.1
	github.com/AgustinSRG/go-child-process-manager v1.0.1
	github.com/AgustinSRG/hls-websocket-cdn/client-publisher v0.0.0-00010101000000-000000000000
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/joho/godotenv v1.5.1
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/sys v0.0.0-20220825204002-c680a09ffe64 // indirect
)
