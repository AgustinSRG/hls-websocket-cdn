// HLS source

package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/AgustinSRG/glog"
)

// HLS fragment
type HlsFragment struct {
	// Duration of the fragment in seconds
	Duration float32

	// Data
	Data []byte
}

// Event types
const HLS_EVENT_TYPE_CLOSE = 0
const HLS_EVENT_TYPE_FRAGMENT = 1

// HLS event
type HlsEvent struct {
	// Event type
	EventType int

	// Fragment reference
	Fragment *HlsFragment
}

// HLS source listener
type HlsSourceListener struct {
	Channel chan HlsEvent
}

// HLS source
type HlsSource struct {
	// Unique ID for the source
	id uint64

	// Mutex
	mu *sync.Mutex

	// Stream ID
	streamId string

	// Controller
	controller *SourcesController

	// Logger
	logger *glog.Logger

	// Map of listeners
	listeners map[uint64]*HlsSourceListener

	// True if closed
	closed bool

	// Buffer of fragments
	fragmentBuffer []*HlsFragment

	// Max length of the fragment buffer
	fragmentBufferMaxLength int

	// Channel to interrupt the announcing thread
	announceInterruptChannel chan bool
}

// Creates new instance of HlsSource
func NewHlsSource(id uint64, controller *SourcesController, streamId string, fragmentBufferMaxLength int) *HlsSource {
	logger := controller.logger.CreateChildLogger("[#" + fmt.Sprint(id) + "] ")

	logger.Infof("New source created for %v", streamId)

	return &HlsSource{
		id:                       id,
		mu:                       &sync.Mutex{},
		streamId:                 streamId,
		controller:               controller,
		logger:                   logger,
		listeners:                make(map[uint64]*HlsSourceListener),
		closed:                   false,
		fragmentBuffer:           make([]*HlsFragment, 0),
		fragmentBufferMaxLength:  fragmentBufferMaxLength,
		announceInterruptChannel: make(chan bool, 1),
	}
}

// Periodically announces the source
func (source *HlsSource) PeriodicallyAnnounce() {
	if !source.controller.config.HasPublishRegistry {
		return
	}

	announceInterval := source.controller.publishRegistry.GetAnnounceInterval()

	for {
		select {
		case <-time.After(announceInterval):
			source.Announce()
		case <-source.announceInterruptChannel:
			return
		}
	}
}

// Announces source to the publish registry
func (source *HlsSource) Announce() {
	if !source.controller.config.HasPublishRegistry {
		return
	}

	err := source.controller.publishRegistry.AnnouncePublishedStream(source.streamId, source.controller.config.ExternalWebsocketUrl)

	if err != nil {
		source.logger.Errorf("Error publishing stream source: %v", err)
	} else {
		source.logger.Debug("Source announced to the publish registry")
	}
}

// Adds a listener
// id - Connection ID
// Returns
// - success: True if the listener was added. If the source is closed, it will be false
// - channel: The channel to receive the events
// - initialFragments: List of fragments to be sent as initial (they were in the buffer)
func (source *HlsSource) AddListener(id uint64) (success bool, channel chan HlsEvent, initialFragments []*HlsFragment) {
	lis := &HlsSourceListener{
		Channel: make(chan HlsEvent, source.fragmentBufferMaxLength),
	}

	source.mu.Lock()
	defer source.mu.Unlock()

	if source.closed {
		return false, nil, nil
	}

	source.listeners[id] = lis

	initialFragmentsBuffer := make([]*HlsFragment, len(source.fragmentBuffer))
	copy(initialFragmentsBuffer, source.fragmentBuffer)

	return true, lis.Channel, initialFragmentsBuffer
}

// Removes a listener
// id - Connection ID
func (source *HlsSource) RemoveListener(id uint64) {
	source.mu.Lock()
	defer source.mu.Unlock()

	delete(source.listeners, id)
}

// Closes the source
func (source *HlsSource) Close() {
	source.mu.Lock()
	defer source.mu.Unlock()

	if source.closed {
		return
	}

	if source.logger.Config.DebugEnabled {
		source.logger.Debug("Source closed")
	}

	closeEvent := HlsEvent{
		EventType: HLS_EVENT_TYPE_CLOSE,
	}

	for _, lis := range source.listeners {
		lis.Channel <- closeEvent
	}

	source.listeners = nil
	source.closed = true

	source.announceInterruptChannel <- true
}

// Adds fragment
func (source *HlsSource) AddFragment(frag *HlsFragment) {
	source.mu.Lock()
	defer source.mu.Unlock()

	if source.closed {
		return
	}

	if source.logger.Config.DebugEnabled {
		source.logger.Debugf("Fragment added. Duration: %v, Size: %v", frag.Duration, len(frag.Data))
	}

	// Append the fragment to the buffer

	newFragmentBuffer, canAdd := source.controller.memoryLimiter.CheckBeforeAddingFragment(source.fragmentBuffer, frag)

	if canAdd {
		if len(source.fragmentBuffer) >= source.fragmentBufferMaxLength && len(source.fragmentBuffer) > 0 {
			source.fragmentBuffer = append(source.fragmentBuffer[1:], frag)
		} else {
			source.fragmentBuffer = append(source.fragmentBuffer, frag)
		}
	} else {
		source.fragmentBuffer = newFragmentBuffer
	}

	// Send fragment to the listeners

	fragmentEvent := HlsEvent{
		EventType: HLS_EVENT_TYPE_FRAGMENT,
		Fragment:  frag,
	}

	for _, lis := range source.listeners {
		select {
		case lis.Channel <- fragmentEvent:
		default:
		}
	}
}
