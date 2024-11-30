// Publish registry test tools

package main

import (
	"sync"
	"time"
)

// Mock publish registry, for testing
type MockPublishRegistry struct {
	// Mutex for the struct
	mu *sync.Mutex

	// Internal registry
	registry map[string]string
}

func NewMockPublishRegistry() *MockPublishRegistry {
	return &MockPublishRegistry{
		mu:       &sync.Mutex{},
		registry: make(map[string]string),
	}
}

func (pr *MockPublishRegistry) GetPublishingServer(streamId string) (string, error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	server := pr.registry[streamId]

	return server, nil
}

func (pr *MockPublishRegistry) GetAnnounceInterval() time.Duration {
	return 10 * time.Second
}

func (pr *MockPublishRegistry) AnnouncePublishedStream(streamId string, url string) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.registry[streamId] = url

	return nil
}
