// Main

package main

import (
	"sync"

	"github.com/joho/godotenv"
)

// Default max size (in bytes) for binary messages
const DEFAULT_MAX_BINARY_MSG_SIZE = 50 * 1024 * 1024

// Main
func main() {
	godotenv.Load() // Load env vars

	// Configure logs
	SetDebugLogEnabled(GetEnvBool("LOG_DEBUG", false))
	SetInfoLogEnabled(GetEnvBool("LOG_INFO", true))

	// External URL
	externalWebsocketUrl := FigureOutExternalServerWebsocketUrl()

	if externalWebsocketUrl != "" {
		LogInfo("External websocket URL: " + externalWebsocketUrl)
	} else {
		LogWarning("Could not load external websocket URL. It will be impossible to register the publishing streams.")
	}

	// Publish registry
	var publishRegistry *RedisPublishRegistry = nil

	if GetEnvBool("PUB_REG_REDIS_ENABLED", false) {
		pr, err := NewRedisPublishRegistry(RedisPublishRegistryConfig{
			Host:                          GetEnvString("PUB_REG_REDIS_HOST", "127.0.0.1"),
			Port:                          GetEnvInt("PUB_REG_REDIS_PORT", 6379),
			Password:                      GetEnvString("PUB_REG_REDIS_PASSWORD", ""),
			UseTls:                        GetEnvBool("PUB_REG_REDIS_USE_TLS", false),
			ExternalWebsocketUrl:          externalWebsocketUrl,
			PublishRefreshIntervalSeconds: GetEnvInt("PUB_REG_REFRESH_INTERVAL_SECONDS", 60),
		})

		if err != nil {
			LogError(err, "Could not initialize publish registry")
		}

		publishRegistry = pr
	}

	if publishRegistry != nil {
		LogInfo("Initialized publish registry")
	}

	// Auth
	authController := NewAuthController(AuthConfiguration{
		PullSecret: GetEnvString("PULL_SECRET", ""),
		PushSecret: GetEnvString("PUSH_SECRET", ""),
		AllowPush:  GetEnvBool("PUSH_ALLOWED", true),
	})

	// Sources controller
	sourcesController := NewSourcesController(SourcesControllerConfig{
		FragmentBufferMaxLength: GetEnvInt("FRAGMENT_BUFFER_MAX_LENGTH", 10),
	}, publishRegistry)

	// Relay controller
	relayController := NewRelayController(RelayControllerConfig{
		RelayFromUrl:            GetEnvString("RELAY_FROM_URL", ""),
		RelayFromEnabled:        GetEnvBool("RELAY_FROM_ENABLED", false),
		FragmentBufferMaxLength: GetEnvInt("FRAGMENT_BUFFER_MAX_LENGTH", 10),
		MaxBinaryMessageSize:    GetEnvInt64("MAX_BINARY_MESSAGE_SIZE", DEFAULT_MAX_BINARY_MSG_SIZE),
	}, authController, publishRegistry)

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
		MaxBinaryMessageSize: GetEnvInt64("MAX_BINARY_MESSAGE_SIZE", DEFAULT_MAX_BINARY_MSG_SIZE),
	}, authController, sourcesController, relayController)

	// Run server

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go server.Run(wg)

	// Wait for all threads to finish

	wg.Wait()
}
