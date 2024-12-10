// Publisher

package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/AgustinSRG/glog"
	child_process_manager "github.com/AgustinSRG/go-child-process-manager"
	clientpublisher "github.com/AgustinSRG/hls-websocket-cdn/client-publisher"
)

const (
	HLS_LIVE_PLAYLIST_SIZE = 10
)

type TesterPublisher struct {
	mu *sync.Mutex

	playlist *HLS_PlayList

	logger *glog.Logger

	fragmentCount int

	fragments      map[int]*HLS_Fragment
	fragmentsReady map[int]bool
	fragmentsData  map[int]([]byte)

	clientPublisher *clientpublisher.HlsWebSocketPublisher
}

func (server *TesterPublisher) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	server.logger.Debugf("%v %v", req.Method, req.RequestURI)

	if req.Method != "PUT" {
		w.WriteHeader(200)
		return
	}

	uriParts := strings.Split(req.RequestURI, "/")

	if len(uriParts) != 3 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Bad request.")
		return
	}

	if uriParts[1] == "hls" {
		file := uriParts[2]

		if strings.HasSuffix(file, ".m3u8") {
			server.HandleRequestHLS_M3U8(w, req, file)
		} else if strings.HasSuffix(file, ".ts") {
			server.HandleRequestHLS_TS(w, req, file)
		} else {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Bad request: Invalid HLS file")
			return
		}
	} else {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Bad request: Invalid stream type")
		return
	}

}

func (server *TesterPublisher) HandleRequestHLS_M3U8(w http.ResponseWriter, req *http.Request, file string) {
	// Read the body

	bodyData, err := io.ReadAll(req.Body)

	if err != nil {
		server.logger.Errorf("Error reading body: %v", err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error.")
		return
	}

	// Decode playlist

	playList := DecodeHLSPlayList(string(bodyData))

	server.logger.Debugf("Decoded playlist, with %v fragments", len(playList.fragments))

	// Notice task

	server.OnPlaylistUpdate(playList)

	w.WriteHeader(200)
}

func (server *TesterPublisher) HandleRequestHLS_TS(w http.ResponseWriter, req *http.Request, file string) {
	fileParts := strings.Split(file, ".")

	if len(fileParts) != 2 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Bad request: Invalid TS file")
		return
	}

	fileIndex, err := strconv.Atoi(fileParts[0])

	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Bad request: Invalid TS file")
		return
	}

	// Read data

	fragmentData, err := io.ReadAll(req.Body)

	if err != nil {
		server.logger.Errorf("Error reading body: %v", err)
		w.WriteHeader(400)
		fmt.Fprintf(w, "Could not read request body.")
		return
	}

	// Notice the task that the preview image is ready

	server.OnFragmentReady(fileIndex, fragmentData)

	w.WriteHeader(200)
}

func (server *TesterPublisher) OnFragmentReady(fragmentIndex int, data []byte) {
	server.mu.Lock()
	defer server.mu.Unlock()

	if fragmentIndex < server.fragmentCount {
		return
	}

	server.fragmentsReady[fragmentIndex] = true
	server.fragmentsData[fragmentIndex] = data

	server.updateHLSInternal()
}

func (server *TesterPublisher) OnPlaylistUpdate(playlist *HLS_PlayList) {
	server.mu.Lock()
	defer server.mu.Unlock()

	for i := 0; i < len(playlist.fragments); i++ {
		if playlist.fragments[i].Index < server.fragmentCount {
			continue
		}

		server.fragments[playlist.fragments[i].Index] = &playlist.fragments[i]
	}

	server.updateHLSInternal()
}

func (server *TesterPublisher) updateHLSInternal() {
	// Compute the new fragment count
	newFragments := make([]HLS_Fragment, 0)
	oldFragmentCount := server.fragmentCount
	newFragmentCount := oldFragmentCount
	doneCounting := false

	for !doneCounting {
		nextFragment := server.fragments[newFragmentCount]
		if nextFragment != nil && server.fragmentsReady[newFragmentCount] {
			newFragments = append(newFragments, *nextFragment)
			delete(server.fragmentsReady, newFragmentCount)
			delete(server.fragments, newFragmentCount)
			newFragmentCount++
		} else {
			doneCounting = true
		}
	}

	if oldFragmentCount == newFragmentCount {
		return
	}

	server.fragmentCount = newFragmentCount

	// Update HLS Live playlist

	if server.playlist == nil {
		server.playlist = &HLS_PlayList{
			Version:        M3U8_DEFAULT_VERSION,
			TargetDuration: HLS_DEFAULT_SEGMENT_TIME,
			MediaSequence:  0,
			IsVOD:          false,
			IsEnded:        false,
			fragments:      make([]HLS_Fragment, 0),
		}
	}

	livePlaylist := server.playlist

	for i := 0; i < len(newFragments); i++ {
		// Send to the CDN
		data := server.fragmentsData[newFragments[i].Index]

		if data != nil {
			delete(server.fragmentsData, newFragments[i].Index)

			if len(data) > 0 {
				server.clientPublisher.SendFragment(float32(newFragments[i].Duration), data)
			}
		}

		// Add to the live playlist

		livePlaylist.fragments = append(livePlaylist.fragments, newFragments[i])

		if len(livePlaylist.fragments) > HLS_LIVE_PLAYLIST_SIZE {
			livePlaylist.fragments = livePlaylist.fragments[1:]
		}
	}

	if len(livePlaylist.fragments) > 0 {
		livePlaylist.MediaSequence = livePlaylist.fragments[0].Index
	} else {
		livePlaylist.MediaSequence = 0
	}
}

func publishStream(logger *glog.Logger, ffmpegPath string, videoPath string, url string, streamId string, secret string) {
	// Connect to the server

	clientPublisher := clientpublisher.NewHlsWebSocketPublisher(clientpublisher.HlsWebSocketPublisherConfiguration{
		ServerUrl:  url,
		StreamId:   streamId,
		AuthSecret: secret,
		OnReady: func() {
			logger.Info("Client publisher is ready")
		},
		OnError: func(url, msg string) {
			logger.Errorf("Could not connect to %v | %v", url, msg)
		},
	})

	defer clientPublisher.Close()

	// Create struct to store status

	testerPublisher := &TesterPublisher{
		mu:              &sync.Mutex{},
		playlist:        nil,
		logger:          logger.CreateChildLogger("[LoopbackServer] "),
		fragments:       make(map[int]*HLS_Fragment),
		fragmentsReady:  make(map[int]bool),
		fragmentsData:   make(map[int][]byte),
		fragmentCount:   0,
		clientPublisher: clientPublisher,
	}

	// Setup a loopback server to receive HLS from FFmpeg

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		_ = http.Serve(listener, testerPublisher)
	}()

	testerPublisher.logger.Infof("Listing on port %v", port)

	// Prepare FFmpeg command

	err = child_process_manager.InitializeChildProcessManager()
	if err != nil {
		panic(err)
	}

	defer child_process_manager.DisposeChildProcessManager()

	cmd := exec.Command(ffmpegPath)

	cmd.Args = make([]string, 1)

	cmd.Args[0] = ffmpegPath

	cmd.Args = append(cmd.Args, "-re", "-stream_loop", "-1", "-i", videoPath)

	cmd.Args = append(cmd.Args, "-vcodec", "libx264", "-acodec", "aac", "-preset", "veryfast", "-pix_fmt", "yuv420p")

	cmd.Args = append(cmd.Args, "-force_key_frames", "expr:gte(t,n_forced*"+fmt.Sprint(HLS_DEFAULT_SEGMENT_TIME)+")")

	cmd.Args = append(cmd.Args, "-hls_list_size", fmt.Sprint(HLS_LIVE_PLAYLIST_SIZE))
	cmd.Args = append(cmd.Args, "-hls_time", fmt.Sprint(HLS_DEFAULT_SEGMENT_TIME))

	cmd.Args = append(cmd.Args, "-method", "PUT")
	cmd.Args = append(cmd.Args, "-hls_segment_filename", "http://127.0.0.1:"+fmt.Sprint(port)+"/hls/%d.ts")
	cmd.Args = append(cmd.Args, "http://127.0.0.1:"+fmt.Sprint(port)+"/hls/index.m3u8")

	err = child_process_manager.ConfigureCommand(cmd)
	if err != nil {
		panic(err)
	}

	// Create a pipe to read StdErr
	pipe, err := cmd.StderrPipe()

	if err != nil {
		panic(err)
	}

	// Read from the pipe

	go func() {
		reader := bufio.NewReader(pipe)

		for {
			line, err := reader.ReadString('\r')

			if err != nil {
				return
			}

			line = strings.ReplaceAll(line, "\r", "")

			logger.Trace("[FFMPEG] " + line)
		}
	}()

	// Start process

	cmd.Start()

	// Add process as a child process
	err = child_process_manager.AddChildProcess(cmd.Process)
	if err != nil {
		cmd.Process.Kill() // We must kill the process if this fails
		panic(err)
	}

	// Wait for it to finish

	cmd.Wait()
}
