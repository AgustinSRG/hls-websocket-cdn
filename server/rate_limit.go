// Rate limit

package main

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/AgustinSRG/glog"
)

// Rate limit configuration
type RateLimiterConfig struct {
	// True if enabled
	Enabled bool

	// Whitelist
	Whitelist string

	// Max number of connections
	MaxConnections int

	// Max requests per second
	MaxRequestsPerSecond int

	// Request burst
	RequestBurst int

	// Interval to perform cleanup on the request count map (seconds)
	CleanupIntervalSeconds int64
}

// Struct to store the request count of an IP
type RequestCount struct {
	// Request count
	count int

	// Timestamp of the last time the count was checked
	// (Unix seconds)
	timestamp int64
}

func (rc *RequestCount) Update(now int64, maxRequestsPerSecond int) {
	secondsPassed := now - rc.timestamp

	if secondsPassed <= 0 {
		return
	}

	allowedRequests := int64(maxRequestsPerSecond) * secondsPassed

	if allowedRequests > int64(rc.count) {
		rc.count = 0
	} else {
		rc.count -= int(allowedRequests)
	}

	rc.timestamp = now
}

// Rate limiter
type RateLimiter struct {
	// Configuration
	config RateLimiterConfig

	// Mutex for the struct
	mu *sync.Mutex

	// Logger
	logger *glog.Logger

	// List of IP ranges
	whitelistArray []*net.IPNet

	// Whitelist all (*)
	whitelistAll bool

	// Map (IP -> Connections count)
	connectionsCount map[string]int

	// Request limit
	requestLimit int

	// Map (IP -> Request count)
	requestCount map[string]*RequestCount

	// Last cleanup timestamp (Unix seconds)
	lastCleanup int64
}

// Creates new instance of RateLimiter
func NewRateLimiter(config RateLimiterConfig, logger *glog.Logger) *RateLimiter {
	whitelistArray := make([]*net.IPNet, 0)
	whitelistAll := false

	if config.Whitelist != "" {
		if config.Whitelist == "*" {
			whitelistAll = true
		} else {
			parts := strings.Split(config.Whitelist, ",")

			for _, part := range parts {
				_, rang, err := net.ParseCIDR(part)

				if err != nil {
					logger.Warningf("Config Error: Invalid IP range provided: %v", part)
					continue
				}

				whitelistArray = append(whitelistArray, rang)
			}
		}
	}

	requestLimit := config.RequestBurst

	if requestLimit < config.MaxRequestsPerSecond {
		requestLimit = config.MaxRequestsPerSecond
	}

	return &RateLimiter{
		config:           config,
		mu:               &sync.Mutex{},
		logger:           logger,
		whitelistArray:   whitelistArray,
		whitelistAll:     whitelistAll,
		connectionsCount: make(map[string]int),
		requestLimit:     requestLimit,
		requestCount:     make(map[string]*RequestCount),
		lastCleanup:      time.Now().Unix(),
	}
}

// Checks if IP is excepted from the rate limit
func (rl *RateLimiter) isIPExempted(ipStr string) bool {
	ip := net.ParseIP(ipStr)

	if ip == nil || rl.whitelistAll {
		return true
	}

	for _, rang := range rl.whitelistArray {
		if rang.Contains(ip) {
			return true
		}
	}

	return false
}

// Call when a connection is started
// If this method returns false, the connection should be rejected
func (rl *RateLimiter) StartConnection(ipStr string) bool {
	if !rl.config.Enabled {
		return true
	}

	if rl.config.MaxConnections <= 0 {
		return true
	}

	if rl.isIPExempted(ipStr) {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	oldCount := rl.connectionsCount[ipStr]

	if oldCount >= rl.config.MaxConnections {
		return false
	}

	rl.connectionsCount[ipStr] = oldCount + 1

	return true
}

// Call when a connection ends
func (rl *RateLimiter) EndConnection(ipStr string) {
	if !rl.config.Enabled {
		return
	}

	if rl.config.MaxConnections <= 0 {
		return
	}

	if rl.isIPExempted(ipStr) {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	oldCount := rl.connectionsCount[ipStr]

	if oldCount <= 1 {
		delete(rl.connectionsCount, ipStr)
	} else {
		rl.connectionsCount[ipStr] = oldCount - 1
	}
}

// Counts requests
// Returns false if the request should be rejected
func (rl *RateLimiter) CountRequest(ipStr string) bool {
	if !rl.config.Enabled {
		return true
	}

	if rl.config.MaxRequestsPerSecond <= 0 {
		return true
	}

	if rl.isIPExempted(ipStr) {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now().Unix()

	if now-rl.lastCleanup >= rl.config.CleanupIntervalSeconds {
		// Perform cleanup

		rl.logger.Debug("Performing cleanup of request counters")

		delCount := 0

		for ip, rc := range rl.requestCount {
			rc.Update(now, rl.config.MaxRequestsPerSecond)

			if rc.count <= 0 {
				delete(rl.requestCount, ip)
				delCount++
			}
		}

		rl.logger.Debugf("Cleanup finished. Total counters removed: %v", delCount)
	}

	// Get request counter

	rc := rl.requestCount[ipStr]

	if rc == nil {
		// First request
		rl.requestCount[ipStr] = &RequestCount{
			count:     1,
			timestamp: now,
		}
		return true
	}

	// Update counter

	rc.Update(now, rl.config.MaxRequestsPerSecond)

	// Check limit

	if rc.count >= rl.requestLimit {
		return false
	}

	// Add to the counter

	rc.count++

	return true
}
