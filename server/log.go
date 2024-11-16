// Logs
// Any utils to log events must be placed here

package main

import (
	"log"
)

var (
	log_debug_enabled = false
	log_info_enabled  = false
)

func SetDebugLogEnabled(enabled bool) {
	log_debug_enabled = enabled
}

func SetInfoLogEnabled(enabled bool) {
	log_info_enabled = enabled
}

func LogLine(line string) {
	log.Println(line)
}

func LogWarning(line string) {
	LogLine("[WARNING] " + line)
}

func LogInfo(line string) {
	if log_info_enabled {
		LogLine("[INFO] " + line)
	}
}

func LogError(err error, msg string) {
	if err != nil {
		LogLine("[ERROR] " + msg + " | " + err.Error())
	} else {
		LogLine("[ERROR] " + msg)
	}
}

func LogDebug(line string) {
	if log_debug_enabled {
		LogLine("[DEBUG] " + line)
	}
}
