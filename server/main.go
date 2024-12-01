// Main

package main

import (
	"sync"

	"github.com/AgustinSRG/genv"
	"github.com/AgustinSRG/glog"
	"github.com/joho/godotenv"
)

// Default max size (in bytes) for binary messages
const DEFAULT_MAX_BINARY_MSG_SIZE = 50 * 1024 * 1024

// Default fragment buffer max length
const DEFAULT_FRAGMENT_BUFFER_MAX_LENGTH = 10

// Main
func main() {
	_ = godotenv.Load() // Load env vars

	// Configure logs
	logger := glog.CreateRootLogger(glog.LoggerConfiguration{
		ErrorEnabled:   genv.GetEnvBool("LOG_ERROR", true),
		WarningEnabled: genv.GetEnvBool("LOG_WARNING", true),
		InfoEnabled:    genv.GetEnvBool("LOG_INFO", true),
		DebugEnabled:   genv.GetEnvBool("LOG_DEBUG", false),
		TraceEnabled:   genv.GetEnvBool("LOG_TRACE", false),
	}, glog.StandardLogFunction)

	// External URL
	externalWebsocketUrl := FigureOutExternalServerWebsocketUrl(logger.CreateChildLogger("[FigureOutExternalServerWebsocketUrl] "))

	if externalWebsocketUrl != "" {
		logger.Info("External websocket URL: " + externalWebsocketUrl)
	} else {
		logger.Warning("Could not load external websocket URL. It will be impossible to register the publishing streams.")
	}

	// Publish registry
	var publishRegistry *RedisPublishRegistry = nil

	if genv.GetEnvBool("PUB_REG_REDIS_ENABLED", false) {
		pr, err := NewRedisPublishRegistry(RedisPublishRegistryConfig{
			Host:                          genv.GetEnvString("PUB_REG_REDIS_HOST", "127.0.0.1"),
			Port:                          genv.GetEnvInt("PUB_REG_REDIS_PORT", 6379),
			Password:                      genv.GetEnvString("PUB_REG_REDIS_PASSWORD", ""),
			UseTls:                        genv.GetEnvBool("PUB_REG_REDIS_USE_TLS", false),
			PublishRefreshIntervalSeconds: genv.GetEnvInt("PUB_REG_REFRESH_INTERVAL_SECONDS", 60),
		})

		if err != nil {
			logger.Errorf("Could not initialize publish registry: %v", err)
		}

		publishRegistry = pr
	}

	if publishRegistry != nil {
		logger.Info("Initialized publish registry")
	}

	// Auth
	authController := NewAuthController(AuthConfiguration{
		PullSecret: genv.GetEnvString("PULL_SECRET", ""),
		PushSecret: genv.GetEnvString("PUSH_SECRET", ""),
		AllowPush:  genv.GetEnvBool("PUSH_ALLOWED", true),
	}, logger.CreateChildLogger("[Auth] "))

	// Memory limiter
	memoryLimiter := NewFragmentBufferMemoryLimiter(FragmentBufferMemoryLimiterConfig{
		Enabled: genv.GetEnvBool("BUFFER_MEMORY_LIMITER_ENABLED", false),
		Limit:   genv.GetEnvInt64("BUFFER_MEMORY_LIMIT_MB", 256) * 1024 * 1024,
	})

	// Sources controller
	sourcesController := NewSourcesController(SourcesControllerConfig{
		FragmentBufferMaxLength: genv.GetEnvInt("FRAGMENT_BUFFER_MAX_LENGTH", DEFAULT_FRAGMENT_BUFFER_MAX_LENGTH),
		ExternalWebsocketUrl:    externalWebsocketUrl,
		HasPublishRegistry:      publishRegistry != nil,
	}, publishRegistry, memoryLimiter, logger.CreateChildLogger("[Sources] "))

	// Relay controller
	relayController := NewRelayController(RelayControllerConfig{
		RelayFromUrl:            genv.GetEnvString("RELAY_FROM_URL", ""),
		RelayFromEnabled:        genv.GetEnvBool("RELAY_FROM_ENABLED", false),
		FragmentBufferMaxLength: genv.GetEnvInt("FRAGMENT_BUFFER_MAX_LENGTH", DEFAULT_FRAGMENT_BUFFER_MAX_LENGTH),
		MaxBinaryMessageSize:    genv.GetEnvInt64("MAX_BINARY_MESSAGE_SIZE", DEFAULT_MAX_BINARY_MSG_SIZE),
		HasPublishRegistry:      publishRegistry != nil,
	}, authController, publishRegistry, memoryLimiter, logger.CreateChildLogger("[Relays] "))

	rateLimiter := NewRateLimiter(RateLimiterConfig{
		Enabled:                genv.GetEnvBool("RATE_LIMIT_ENABLED", false),
		Whitelist:              genv.GetEnvString("RATE_LIMIT_WHITELIST", ""),
		MaxConnections:         genv.GetEnvInt("RATE_LIMIT_CONNECTIONS", 0),
		MaxRequestsPerSecond:   genv.GetEnvInt("RATE_LIMIT_REQ_PER_SEC", 0),
		RequestBurst:           genv.GetEnvInt("RATE_LIMIT_REQ_BURST", 0),
		CleanupIntervalSeconds: genv.GetEnvInt64("RATE_LIMIT_REQ_CLEANUP", 10),
	}, logger.CreateChildLogger("[RateLimiter] "))

	// Setup server
	server := CreateHttpServer(HttpServerConfig{
		// HTTP
		HttpEnabled:  genv.GetEnvBool("HTTP_ENABLED", true),
		InsecurePort: genv.GetEnvInt("HTTP_PORT", 80),
		BindAddress:  genv.GetEnvString("HTTP_BIND_ADDRESS", ""),
		// TLS
		TlsEnabled:            genv.GetEnvBool("TLS_ENABLED", false),
		TlsPort:               genv.GetEnvInt("TLS_PORT", 443),
		TlsBindAddress:        genv.GetEnvString("TLS_BIND_ADDRESS", ""),
		TlsCertificateFile:    genv.GetEnvString("TLS_CERTIFICATE", ""),
		TlsPrivateKeyFile:     genv.GetEnvString("TLS_PRIVATE_KEY", ""),
		TlsCheckReloadSeconds: genv.GetEnvInt("TLS_CHECK_RELOAD_SECONDS", 60),
		// Other config
		WebsocketPrefix:      genv.GetEnvString("WEBSOCKET_PREFIX", "/"),
		MaxBinaryMessageSize: genv.GetEnvInt64("MAX_BINARY_MESSAGE_SIZE", DEFAULT_MAX_BINARY_MSG_SIZE),
		LogRequests:          genv.GetEnvBool("LOG_REQUESTS", true),
	}, logger.CreateChildLogger("[Server] "), authController, sourcesController, relayController, rateLimiter)

	// Run server

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go server.Run(wg)

	// Wait for all threads to finish

	wg.Wait()
}
