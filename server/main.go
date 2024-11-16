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

	// Setup server (TODO)

	// Run server

	wg := &sync.WaitGroup{}

	wg.Add(1)
	// TODO: Run server

	// Wait for all threads to finish

	wg.Wait()
}
