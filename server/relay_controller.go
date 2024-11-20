// Relay controller

package main

import "sync"

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
}

// Relay controller
type RelayController struct {
	// Configuration
	config RelayControllerConfig

	// Mutex
	mu *sync.Mutex

	// Relays
	relays map[string]*HlsRelay

	// Next relay ID
	nextRelayId uint64

	// Authentication controller
	authController *AuthController

	// Publish registry
	publishRegistry *RedisPublishRegistry
}

// Creates an instance RelayController
func NewRelayController(config RelayControllerConfig, authController *AuthController, publishRegistry *RedisPublishRegistry) *RelayController {
	return &RelayController{
		config:          config,
		mu:              &sync.Mutex{},
		relays:          make(map[string]*HlsRelay),
		authController:  authController,
		publishRegistry: publishRegistry,
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

	if rc.publishRegistry != nil {
		pubRegUrl, err := rc.publishRegistry.GetPublishingServer(streamId)

		if err != nil {
			LogError(err, "Could not find publishing server for stream: "+streamId)
		} else if pubRegUrl != "" {
			relayUrl = pubRegUrl
			onlySource = true
		}
	}

	if relayUrl == "" && rc.config.RelayFromEnabled {
		relayUrl = rc.config.RelayFromUrl
	}

	if relayUrl == "" {
		return nil
	}

	return rc.GetRelayOrCreate(streamId, relayUrl, onlySource)
}
