// HLS sources controller

package main

import "sync"

// Configuration for the sources controller
type SourcesControllerConfig struct {
	// Max length of the fragment buffer
	FragmentBufferMaxLength int

	// External websocket URL
	ExternalWebsocketUrl string

	// True if it has a publish registry
	HasPublishRegistry bool
}

// Sources controller
type SourcesController struct {
	// Mutex
	mu *sync.Mutex

	// Publish registry
	publishRegistry PublishRegistry

	// Configuration
	config SourcesControllerConfig

	// Sources
	sources map[string]*HlsSource
}

// Creates new instance of SourcesController
func NewSourcesController(config SourcesControllerConfig, publishRegistry PublishRegistry) *SourcesController {
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
	source := NewHlsSource(sc, streamId, sc.config.FragmentBufferMaxLength)

	sc.mu.Lock()

	existingSource := sc.sources[streamId]

	sc.sources[streamId] = source

	sc.mu.Unlock()

	// Close existing source

	if existingSource != nil {
		existingSource.Close()
	}

	// Announce

	source.Announce()

	return source
}

// Removes a source
// Must be called only after the source of closed, by the publisher
func (sc *SourcesController) RemoveSource(streamId string, source *HlsSource) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	existingSource := sc.sources[streamId]

	if existingSource != source {
		return
	}

	delete(sc.sources, streamId)
}
