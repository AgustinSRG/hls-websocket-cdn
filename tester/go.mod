module github.com/AgustinSRG/hls-websocket-cdn/tester

go 1.23.0

toolchain go1.24.1

replace github.com/AgustinSRG/hls-websocket-cdn/client-publisher => ../client-publisher

require (
	github.com/AgustinSRG/genv v1.0.0
	github.com/AgustinSRG/glog v1.0.1
	github.com/AgustinSRG/go-child-process-manager v1.0.1
	github.com/AgustinSRG/hls-websocket-cdn/client-publisher v0.0.0-00010101000000-000000000000
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/joho/godotenv v1.5.1
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/sys v0.31.0 // indirect
)
