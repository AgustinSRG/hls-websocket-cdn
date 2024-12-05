// Spectator

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/AgustinSRG/glog"
)

type SpectatorServer struct {
	logger   *glog.Logger
	jsBundle string
	url      string
	streamId string
	secret   string
}

func (server *SpectatorServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	server.logger.Debugf("%v %v", req.Method, req.RequestURI)

	if req.Method != "GET" {
		w.WriteHeader(404)
		fmt.Fprint(w, "Not found")
		return
	}

	switch req.RequestURI {
	case "/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		fmt.Fprint(w, server.GetSpectatorTesterPage())
	case "/hls-websocket-cdn.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.WriteHeader(200)
		fmt.Fprint(w, server.jsBundle)
	default:
		w.WriteHeader(404)
		fmt.Fprint(w, "Not found")
	}
}

func (server *SpectatorServer) GetSpectatorTesterPageStyle() string {
	css := `
	/* Style */

	*,
	*::before,
	*::after {
    	box-sizing: border-box;
	}

	body {
		padding: 0.5rem;
	}
	`
	return css
}

func (server *SpectatorServer) GetSpectatorTesterPageScript() string {
	return `
	    window.playStream = function () {
			const videoElement = document.getElementById("video");
			const HlsWebSocket = HlsWebSocketCdn.HlsWebSocket;

			if (HlsWebSocket.isSupported()) {
				var hls = new HlsWebSocket({
            		cdnServerUrl: CDN_URL,
            		streamId: STREAM_ID,
					authToken: AUTH_TOKEN,
					debug: true,
        		}, {debug: true, liveMaxLatencyDuration: 6, liveSyncDuration: 5});

				hls.start();

				hls.attachMedia(videoElement);
			
				videoElement.play();
			} else {
				console.log("[ERROR] MSE is not supported by this browser");
			}
		}

		playStream();
	`
}

func (server *SpectatorServer) GetSpectatorTesterPage() string {
	html := "<!DOCTYPE html>"
	html += "<html lang=\"en\">"

	// HEAD

	html += "<head>"

	html += "<meta charset=\"UTF-8\">"
	html += "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">"
	html += "<title>HLS WebSocket CDN test spectator</title>"

	html += "<style>" + server.GetSpectatorTesterPageStyle() + "</style>"

	html += "<script type=\"text/javascript\" src=\"/hls-websocket-cdn.js\"></script>"

	// Parameters

	html += "<script>"

	urlJson, err := json.Marshal(server.url)

	if err != nil {
		panic(err)
	}

	html += "window.CDN_URL = " + string(urlJson) + ";"

	idJson, err := json.Marshal(server.streamId)

	if err != nil {
		panic(err)
	}

	html += "window.STREAM_ID = " + string(idJson) + ";"

	authToken, err := signAuthToken(server.secret, "PULL", server.streamId)

	if err != nil {
		panic(err)
	}

	authTokenJson, err := json.Marshal(authToken)

	if err != nil {
		panic(err)
	}

	html += "window.AUTH_TOKEN = " + string(authTokenJson) + ";"

	html += "</script>"

	html += "</head>"

	// BODY

	html += "<body>"

	html += "<h1>HLS WebSocket CDN test spectator</h1>"

	html += "<p>The stream will be played in the video element below. Refresh the page after the stream ends to play a new one.</p>"

	html += "<video controls id=\"video\"></video>"

	html += "<script>" + server.GetSpectatorTesterPageScript() + "</script>"

	html += "</body>"

	return html
}

func spectateStream(logger *glog.Logger, jsBundlePath string, url string, streamId string, secret string) {
	// Load bundle

	bundle, err := os.ReadFile(jsBundlePath)

	if err != nil {
		logger.Errorf("Error reading JavaScript bundle: %v", bundle)
		return
	}

	// Create server

	server := &SpectatorServer{
		logger:   logger,
		jsBundle: string(bundle),
		url:      url,
		streamId: streamId,
		secret:   secret,
	}

	// Create listener

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	logger.Infof("Spectator ready. To use, open your browser at: http://127.0.0.1:%v/", port)

	// Serve

	http.Serve(listener, server)
}
