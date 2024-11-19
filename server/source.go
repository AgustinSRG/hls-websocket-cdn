// HLS source

package main

import "sync"

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
	// Mutex
	mu *sync.Mutex

	// Map of listeners
	listeners map[uint64]*HlsSourceListener

	// True if closed
	closed bool

	// Buffer of fragments
	fragmentBuffer []*HlsFragment

	// Max length of the fragment buffer
	fragmentBufferMaxLength int
}

// Creates new instance of HlsSource
func NewHlsSource(fragmentBufferMaxLength int) *HlsSource {
	return &HlsSource{
		mu:                      &sync.Mutex{},
		listeners:               make(map[uint64]*HlsSourceListener),
		closed:                  false,
		fragmentBuffer:          make([]*HlsFragment, 0),
		fragmentBufferMaxLength: fragmentBufferMaxLength,
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
		Channel: make(chan HlsEvent),
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

	closeEvent := HlsEvent{
		EventType: HLS_EVENT_TYPE_CLOSE,
	}

	for _, lis := range source.listeners {
		lis.Channel <- closeEvent
	}

	source.listeners = nil
	source.closed = true
}

// Adds fragment
func (source *HlsSource) AddFragment(frag *HlsFragment) {
	source.mu.Lock()
	defer source.mu.Unlock()

	if source.closed {
		return
	}

	// Append the fragment to the buffer

	if len(source.fragmentBuffer) >= source.fragmentBufferMaxLength && len(source.fragmentBuffer) > 0 {
		source.fragmentBuffer = append(source.fragmentBuffer[1:], frag)
	} else {
		source.fragmentBuffer = append(source.fragmentBuffer, frag)
	}

	// Send fragment to the listeners

	fragmentEvent := HlsEvent{
		EventType: HLS_EVENT_TYPE_FRAGMENT,
		Fragment:  frag,
	}

	for _, lis := range source.listeners {
		lis.Channel <- fragmentEvent
	}
}
