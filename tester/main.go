// Main

package main

import (
	"fmt"
	"strings"

	"github.com/AgustinSRG/glog"
	"github.com/joho/godotenv"
)

// Main
func main() {
	_ = godotenv.Load() // Load env vars

	ok, args := LoadArguments()

	if !ok {
		return
	}

	// Configure logs
	logger := glog.CreateRootLogger(glog.LoggerConfiguration{
		ErrorEnabled:   true,
		WarningEnabled: true,
		InfoEnabled:    true,
		DebugEnabled:   args.Debug,
		TraceEnabled:   args.Debug,
	}, glog.StandardLogFunction)

	switch strings.ToLower(args.Command) {
	case "pull", "spectate":
		runSpectator(args, logger)
	case "push", "publish":
		runPublisher(args, logger)
	case "?", "help", "-h", "-help", "--help":
		printHelp()
	default:
		fmt.Printf("Unrecognized command: %v\n\n", args.Command)
		printHelp()
	}
}

func runSpectator(args *TesterArguments, logger *glog.Logger) {
	// Check for bundle

	if !CheckFileExists(args.ClientJavaScriptBundle) {
		fmt.Println("Could not find the JavaScript bundle: " + args.ClientJavaScriptBundle)
		fmt.Println("Make sure to compile the JavaScript client before running the tester.")
		fmt.Println("You can change the bundle location with the --js-bundle option")
		return
	}

	// Check args

	if args.Url == "" {
		fmt.Println("Please, provide the server URL with the --url option")
		return
	}

	if args.StreamId == "" {
		fmt.Println("Please, provide the stream ID with the --id option")
		return
	}

	spectateStream(logger, args.ClientJavaScriptBundle, args.Url, args.StreamId, args.Secret)
}

func runPublisher(args *TesterArguments, logger *glog.Logger) {
	if args.InputVideo == "" {
		fmt.Println("Please, provide a video with the --video option")
		return
	}

	if args.Url == "" {
		fmt.Println("Please, provide the server URL with the --url option")
		return
	}

	if args.StreamId == "" {
		fmt.Println("Please, provide the stream ID with the --id option")
		return
	}

	publishStream(logger, args.FFmpegBinary, args.InputVideo, args.Url, args.StreamId, args.Secret)
}

func printHelp() {
	fmt.Println("Usage: tester <COMMAND> [OPTIONS]")
	fmt.Println("Commands:")
	fmt.Println("  publish   Publishes a video on loop as HLS to the CDN server")
	fmt.Println("  spectate  Server a browser client to spectate as HLS stream")
	fmt.Println("Options:")
	fmt.Println("  -s, --secret <secret>   Secret to sign authentication tokens")
	fmt.Println("  -u, --url <url>         URL of the CDN server")
	fmt.Println("  -i, --id <id>           Stream ID")
	fmt.Println("  -v, --video <path>      Path to the video file to publish")
	fmt.Println("  -d, --debug             Enables debug and trace messages")
	fmt.Println("  --ffmpeg <path>          Sets a custom location to the FFmpeg binary")
	fmt.Println("  --js-bundle <path>       Sets a custom location to the client JavaScript bundle")
}
