// Arguments

package main

import (
	"fmt"
	"os"

	"github.com/AgustinSRG/genv"
)

// Tester arguments
type TesterArguments struct {
	// Command
	Command string

	// Auth secret
	Secret string

	// Input video path
	InputVideo string

	// CDN server URL
	Url string

	// Stream id
	StreamId string

	// Path to the FFmpeg binary
	FFmpegBinary string

	// Path to the client JS bundle
	ClientJavaScriptBundle string

	// Debug mode
	Debug bool
}

func LoadArguments() (bool, *TesterArguments) {
	result := TesterArguments{
		FFmpegBinary:           genv.GetEnvString("FFMPEG_PATH", "/usr/bin/ffmpeg"),
		ClientJavaScriptBundle: genv.GetEnvString("CLIENT_JS_BUNDLE_PATH", "../client-js/dist.webpack/hls-websocket-cdn.js"),
	}
	args := os.Args

	if len(args) == 0 {
		fmt.Printf("Usage: tester <COMMAND> [OPTIONS]\n")
		fmt.Printf("Type tester --help for help\n")
		return false, nil
	}

	if len(args) < 2 {
		fmt.Printf("Usage: %v <COMMAND> [OPTIONS]\n", args[0])
		fmt.Printf("Type %v --help for help\n", args[0])
		return false, nil
	}

	result.Command = args[1]

	for i := 2; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "-s", "--secret":
			if i >= len(args)-1 {
				fmt.Printf("The option %v requires an argument.\n", arg)
				fmt.Printf("Type %v --help for help\n", args[0])
				return false, nil
			}
			result.Secret = args[i+1]
			i++
		case "-d", "--debug":
			result.Debug = true
		case "-u", "--url":
			if i >= len(args)-1 {
				fmt.Printf("The option %v requires an argument.\n", arg)
				fmt.Printf("Type %v --help for help\n", args[0])
				return false, nil
			}
			result.Url = args[i+1]
			i++
		case "-v", "--video":
			if i >= len(args)-1 {
				fmt.Printf("The option %v requires an argument.\n", arg)
				fmt.Printf("Type %v --help for help\n", args[0])
				return false, nil
			}
			result.InputVideo = args[i+1]
			i++
		case "-i", "--id":
			if i >= len(args)-1 {
				fmt.Printf("The option %v requires an argument.\n", arg)
				fmt.Printf("Type %v --help for help\n", args[0])
				return false, nil
			}
			result.StreamId = args[i+1]
			i++
		case "--ffmpeg":
			if i >= len(args)-1 {
				fmt.Printf("The option %v requires an argument.\n", arg)
				fmt.Printf("Type %v --help for help\n", args[0])
				return false, nil
			}
			result.FFmpegBinary = args[i+1]
			i++
		case "--js-bundle":
			if i >= len(args)-1 {
				fmt.Printf("The option %v requires an argument.\n", arg)
				fmt.Printf("Type %v --help for help\n", args[0])
				return false, nil
			}
			result.ClientJavaScriptBundle = args[i+1]
			i++
		default:
			fmt.Printf("Unrecognized option: %v\n", arg)
			fmt.Printf("Type %v --help for help\n", args[0])
			return false, nil
		}
	}

	return true, &result
}
