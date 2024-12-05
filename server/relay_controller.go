// Relay controller

package main

import (
	"sync"

	"github.com/AgustinSRG/glog"
)

// Relay controller configuration
type RelayControllerConfig struct {
	// Websocket URL of another server to relay HLS streams from.
	RelayFromUrl string

	// True of relay from another server is enabled
	RelayFromEnabled bool

	// Max length of the fragment buffer
	FragmentBufferMaxLength int

	// Max binary message size
	MaxBinaryMessageSize int64

	// Inactivity period (seconds)
	InactivityPeriodSeconds int

	// True if it has a publish registry
	HasPublishRegistry bool
}

// Relay controller
type RelayController struct {
	// Configuration
	config RelayControllerConfig

	// Logger
	logger *glog.Logger

	// Mutex
	mu *sync.Mutex

	// Relays
	relays map[string]*HlsRelay

	// Next relay ID
	nextRelayId uint64

	// Authentication controller
	authController *AuthController

	// Publish registry
	publishRegistry PublishRegistry

	// Memory limiter for fragment buffers
	memoryLimiter *FragmentBufferMemoryLimiter
}

// Creates an instance RelayController
func NewRelayController(config RelayControllerConfig, authController *AuthController, publishRegistry PublishRegistry, memoryLimiter *FragmentBufferMemoryLimiter, logger *glog.Logger) *RelayController {
	return &RelayController{
		config:          config,
		logger:          logger,
		mu:              &sync.Mutex{},
		relays:          make(map[string]*HlsRelay),
		authController:  authController,
		publishRegistry: publishRegistry,
		memoryLimiter:   memoryLimiter,
	}
}

// Called after a relay is closed
func (rc *RelayController) OnRelayClosed(relay *HlsRelay) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	existingRelay := rc.relays[relay.streamId]

	if existingRelay != nil && existingRelay.id == relay.id {
		delete(rc.relays, relay.streamId)
	}
}

// Gets an existing relay
func (rc *RelayController) GetRelay(streamId string) *HlsRelay {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	return rc.relays[streamId]
}

// Gets an existing relay
func (rc *RelayController) GetRelayOrCreate(streamId string, relayUrl string, onlySource bool) *HlsRelay {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Does an existing relay can be provided?

	existingRelay := rc.relays[streamId]

	if existingRelay != nil && !existingRelay.IsClosed() {
		return existingRelay
	}

	// Create a new relay

	newRelayId := rc.nextRelayId
	rc.nextRelayId++

	newRelay := NewHlsRelay(rc, newRelayId, relayUrl, streamId, rc.config.FragmentBufferMaxLength, onlySource)

	rc.relays[streamId] = newRelay

	go newRelay.Run() // Run relay

	return newRelay
}

// Finds a source to relay the stream from
// The relay can be nil, meaning a relay method was not found
func (rc *RelayController) RelayStream(streamId string) *HlsRelay {
	existingRelay := rc.GetRelay(streamId)

	if existingRelay != nil && !existingRelay.IsClosed() {
		return existingRelay
	}

	// Find from publish registry

	var relayUrl string = ""
	var onlySource bool = false

	if rc.config.HasPublishRegistry {
		pubRegUrl, err := rc.publishRegistry.GetPublishingServer(streamId)

		if err != nil {
			rc.logger.Errorf("Could not find publishing server for stream: %v, %v", streamId, err)
		} else if pubRegUrl != "" {
			relayUrl = pubRegUrl
			onlySource = true

			if rc.logger.Config.DebugEnabled {
				rc.logger.Debugf("Found server for stream %v -> %v", streamId, pubRegUrl)
			}
		} else {
			if rc.logger.Config.DebugEnabled {
				rc.logger.Debugf("Could not find publishing server for stream: %v", streamId)
			}
		}
	} else {
		rc.logger.Debug("No publish registry is configured")
	}

	if relayUrl == "" && rc.config.RelayFromEnabled {
		relayUrl = rc.config.RelayFromUrl
	}

	if relayUrl == "" {
		return nil
	}

	relay := rc.GetRelayOrCreate(streamId, relayUrl, onlySource)

	// Wait for ready
	relay.WaitUntilReady()

	return relay
}
