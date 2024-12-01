// Memory limiter

package main

import "sync"

// Configuration for memory limiter
type FragmentBufferMemoryLimiterConfig struct {
	// True if enabled
	Enabled bool

	// Limit in bytes
	Limit int64
}

// Fragment buffer memory limiter
type FragmentBufferMemoryLimiter struct {
	// Configuration
	config FragmentBufferMemoryLimiterConfig

	// Mutex for the struct
	mu *sync.Mutex

	// Memory usage
	usage int64
}

// Creates new instance of FragmentBufferMemoryLimiter
func NewFragmentBufferMemoryLimiter(config FragmentBufferMemoryLimiterConfig) *FragmentBufferMemoryLimiter {
	return &FragmentBufferMemoryLimiter{
		config: config,
		mu:     &sync.Mutex{},
		usage:  0,
	}
}

// Call on buffer release
func (ml *FragmentBufferMemoryLimiter) OnBufferRelease(buffer []*HlsFragment) {
	if !ml.config.Enabled || len(buffer) == 0 {
		return
	}

	total := int64(0)

	for _, f := range buffer {
		total += int64(len(f.Data))
	}

	ml.mu.Lock()

	ml.usage -= total

	ml.mu.Unlock()
}

// Checks the memory limit before adding a fragment
// Returns the new buffer (may change to make space) and
// a boolean to indicate if the new fragment can be added to the buffer
func (ml *FragmentBufferMemoryLimiter) CheckBeforeAddingFragment(buffer []*HlsFragment, fragment *HlsFragment) (newBuffer []*HlsFragment, canBeAdded bool) {
	fragmentLength := int64(len(fragment.Data))

	if !ml.config.Enabled || len(fragment.Data) == 0 {
		return buffer, true
	}

	fragmentsToRemove := 0

	ml.mu.Lock()

	for ml.usage+fragmentLength > ml.config.Limit && fragmentsToRemove < len(buffer) {
		ml.usage -= int64(len(buffer[fragmentsToRemove].Data))
		fragmentsToRemove++
	}

	fragmentCanBeAdded := ml.usage+fragmentLength <= ml.config.Limit

	if fragmentCanBeAdded {
		ml.usage += fragmentLength
	}

	ml.mu.Unlock()

	if fragmentsToRemove == 0 {
		return buffer, fragmentCanBeAdded
	} else if fragmentsToRemove >= len(buffer) {
		return make([]*HlsFragment, 0), fragmentCanBeAdded
	} else {
		return buffer[fragmentsToRemove:], fragmentCanBeAdded
	}
}
