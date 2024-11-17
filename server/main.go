// Main

package main

import (
	"sync"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load() // Load env vars

	// Configure logs
	SetDebugLogEnabled(GetEnvBool("LOG_DEBUG", false))
	SetInfoLogEnabled(GetEnvBool("LOG_INFO", true))

	// Setup server
	server := CreateHttpServer(HttpServerConfig{
		// HTTP
		HttpEnabled:  GetEnvBool("HTTP_ENABLED", true),
		InsecurePort: GetEnvInt("HTTP_PORT", 80),
		BindAddress:  GetEnvString("HTTP_BIND_ADDRESS", ""),
		// TLS
		TlsEnabled:            GetEnvBool("TLS_ENABLED", false),
		TlsPort:               GetEnvInt("TLS_PORT", 443),
		TlsBindAddress:        GetEnvString("TLS_BIND_ADDRESS", ""),
		TlsCertificateFile:    GetEnvString("TLS_CERTIFICATE", ""),
		TlsPrivateKeyFile:     GetEnvString("TLS_PRIVATE_KEY", ""),
		TlsCheckReloadSeconds: GetEnvInt("TLS_CHECK_RELOAD_SECONDS", 60),
		// Other config
		WebsocketPrefix:      GetEnvString("WEBSOCKET_PREFIX", "/"),
		MaxBinaryMessageSize: GetEnvInt64("MAX_BINARY_MESSAGE_SIZE", 50*1024*1024),
	})

	// Run server

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go server.Run(wg)

	// Wait for all threads to finish

	wg.Wait()
}
