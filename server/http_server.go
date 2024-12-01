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

	"github.com/AgustinSRG/glog"
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

	// True to log requests
	LogRequests bool
}

// HTTP websocket server
type HttpServer struct {
	// Server config
	config HttpServerConfig

	// Logger
	logger *glog.Logger

	// Mutex
	mu *sync.Mutex

	// Next connection ID
	nextConnectionId uint64

	// Websocket connection upgrader
	upgrader *websocket.Upgrader

	// Auth controller
	authController *AuthController

	// Sources controller
	sourceController *SourcesController

	// Relay controller
	relayController *RelayController
}

// Creates HTTP server
func CreateHttpServer(config HttpServerConfig, logger *glog.Logger, authController *AuthController, sourceController *SourcesController, relayController *RelayController) *HttpServer {
	return &HttpServer{
		config: config,
		logger: logger,
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		mu:               &sync.Mutex{},
		nextConnectionId: 0,
		authController:   authController,
		sourceController: sourceController,
		relayController:  relayController,
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
		server.logger.Errorf("Error parsing request IP: %v", err)
		w.WriteHeader(200)
		fmt.Fprint(w, DEFAULT_HTTP_RESPONSE)
		return
	}

	if server.config.LogRequests {
		server.logger.Infof("[HTTP] [FROM: %v] %v %v", ip, req.Method, req.URL.Path)
	}

	if strings.HasPrefix(req.URL.Path, server.config.WebsocketPrefix) {
		// Upgrade connection

		c, err := server.upgrader.Upgrade(w, req, nil)
		if err != nil {
			server.logger.Errorf("Error upgrading connection: %v", err)
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

	server.logger.Infof("[HTTP] Listening on %v:%v", bind_addr, port)
	errHTTP := http.ListenAndServe(bind_addr+":"+strconv.Itoa(port), server)

	if errHTTP != nil {
		server.logger.Errorf("Error starting HTTP server: %v", errHTTP)
	}
}

// Runs TLS HTTPS server
func (server *HttpServer) RunTls(wg *sync.WaitGroup) {
	defer wg.Done()

	port := server.config.TlsPort
	bind_addr := server.config.TlsBindAddress
	certFile := server.config.TlsCertificateFile
	keyFile := server.config.TlsPrivateKeyFile

	certificateLoader, err := NewSslCertificateLoader(server.logger.CreateChildLogger("[CertificateLoader] "), certFile, keyFile, server.config.TlsCheckReloadSeconds)

	if err != nil {
		server.logger.Errorf("Error starting HTTPS server: %v", err)
		return
	}

	go certificateLoader.RunReloadThread()

	tlsServer := http.Server{
		Addr:    bind_addr + ":" + strconv.Itoa(port),
		Handler: server,
		TLSConfig: &tls.Config{
			GetCertificate: certificateLoader.GetCertificateFunc(),
		},
	}

	server.logger.Infof("[HTTPS] Listening on %v:%v", bind_addr, port)

	errSSL := tlsServer.ListenAndServeTLS("", "")

	if errSSL != nil {
		server.logger.Errorf("Error starting HTTPS server: %v", errSSL)
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
