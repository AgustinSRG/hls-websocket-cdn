// HLS sources controller

package main

import "sync"

// Configuration for the sources controller
type SourcesControllerConfig struct {
	// Max length of the fragment buffer
	FragmentBufferMaxLength int
}

// Sources controller
type SourcesController struct {
	// Mutex
	mu *sync.Mutex

	// Publish registry
	publishRegistry *RedisPublishRegistry

	// Configuration
	config SourcesControllerConfig

	// Sources
	sources map[string]*HlsSource
}

// Creates new instance of SourcesController
func NewSourcesController(config SourcesControllerConfig, publishRegistry *RedisPublishRegistry) *SourcesController {
	return &SourcesController{
		mu:              &sync.Mutex{},
		config:          config,
		sources:         make(map[string]*HlsSource),
		publishRegistry: publishRegistry,
	}
}

// Gets a source
// May return nil if there is no source for the specified streamId
func (sc *SourcesController) GetSource(streamId string) *HlsSource {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	return sc.sources[streamId]
}

// Creates a source
// May return nil if the streamId is already in use
func (sc *SourcesController) CreateSource(streamId string) *HlsSource {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	existingSource := sc.sources[streamId]

	if existingSource != nil {
		return nil
	}

	source := NewHlsSource(sc, streamId, sc.config.FragmentBufferMaxLength)

	sc.sources[streamId] = source

	return source
}

// Removes a source
// Must be called only after the source of closed, by the publisher
func (sc *SourcesController) RemoveSource(streamId string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	delete(sc.sources, streamId)
}
