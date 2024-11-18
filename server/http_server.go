// HTTP server

package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

const DEFAULT_HTTP_RESPONSE = "OK - HLS Websocket CDN"

// HTTP server configuration
type HttpServerConfig struct {
	// HTTP enabled?
	HttpEnabled bool

	// Server port
	InsecurePort int

	// Server bind address
	BindAddress string

	// TLS enabled?
	TlsEnabled bool

	// TLS port
	TlsPort int

	// Server bind address for TLS
	TlsBindAddress string

	// Certificate file
	TlsCertificateFile string

	// Key file
	TlsPrivateKeyFile string

	// Number of second to reload TLS config
	TlsCheckReloadSeconds int

	// Websocket prefix
	WebsocketPrefix string

	// Max binary message size
	MaxBinaryMessageSize int64
}

// HTTP websocket server
type HttpServer struct {
	// Server config
	config HttpServerConfig

	// Mutex
	mu *sync.Mutex

	// Next connection ID
	nextConnectionId uint64

	// Websocket connection upgrader
	upgrader *websocket.Upgrader

	// Auth controller
	authController *AuthController
}

// Creates HTTP server
func CreateHttpServer(config HttpServerConfig, authController *AuthController) *HttpServer {
	return &HttpServer{
		config:           config,
		upgrader:         &websocket.Upgrader{},
		mu:               &sync.Mutex{},
		nextConnectionId: 0,
		authController:   authController,
	}
}

// Gets an unique ID for a connection
func (server *HttpServer) GetConnectionId() uint64 {
	server.mu.Lock()
	defer server.mu.Unlock()

	id := server.nextConnectionId

	server.nextConnectionId++

	return id
}

// Serves HTTP request
func (server *HttpServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)

	if err != nil {
		LogError(err, "Error parsing request IP")
		w.WriteHeader(200)
		fmt.Fprint(w, DEFAULT_HTTP_RESPONSE)
		return
	}

	LogInfo("[HTTP] [FROM: " + ip + "] " + req.Method + " " + req.URL.Path)

	if strings.HasPrefix(req.URL.Path, server.config.WebsocketPrefix) {
		// Upgrade connection

		c, err := server.upgrader.Upgrade(w, req, nil)
		if err != nil {
			LogError(err, "Error upgrading connection")
			return
		}

		// Handle connection
		ch := CreateConnectionHandler(c, server)
		go ch.Run()
	} else {
		w.WriteHeader(200)
		fmt.Fprint(w, DEFAULT_HTTP_RESPONSE)
	}
}

// Runs insecure HTTP server
func (server *HttpServer) RunInsecure(wg *sync.WaitGroup) {
	defer wg.Done()

	port := server.config.InsecurePort
	bind_addr := server.config.BindAddress

	LogInfo("[HTTP] Listening on " + bind_addr + ":" + strconv.Itoa(port))
	errHTTP := http.ListenAndServe(bind_addr+":"+strconv.Itoa(port), server)

	if errHTTP != nil {
		LogError(errHTTP, "Error starting HTTP server")
	}
}

// Runs TLS HTTPS server
func (server *HttpServer) RunTls(wg *sync.WaitGroup) {
	defer wg.Done()

	port := server.config.TlsPort
	bind_addr := server.config.TlsBindAddress
	certFile := server.config.TlsCertificateFile
	keyFile := server.config.TlsPrivateKeyFile

	certificateLoader, err := NewSslCertificateLoader(certFile, keyFile, server.config.TlsCheckReloadSeconds)

	if err != nil {
		LogError(err, "Error starting HTTPS server")
	}

	go certificateLoader.RunReloadThread()

	tlsServer := http.Server{
		Addr:    bind_addr + ":" + strconv.Itoa(port),
		Handler: server,
		TLSConfig: &tls.Config{
			GetCertificate: certificateLoader.GetCertificateFunc(),
		},
	}

	LogInfo("[HTTPS] Listening on " + bind_addr + ":" + strconv.Itoa(port))

	errSSL := tlsServer.ListenAndServeTLS("", "")

	if errSSL != nil {
		LogError(errSSL, "Error starting HTTPS server")
	}
}

// Runs the server
// wg - Wait group
func (server *HttpServer) Run(wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
	}()

	wgInternal := &sync.WaitGroup{}

	if server.config.TlsEnabled {
		wgInternal.Add(1)
		go server.RunTls(wgInternal)
	}

	if server.config.HttpEnabled {
		wgInternal.Add(1)
		go server.RunInsecure(wgInternal)
	}

	// Wait for all threads to finish

	wgInternal.Wait()

}
