// Rate limit test

package main

import (
	"testing"

	"github.com/AgustinSRG/glog"
)

func TestRateLimiter(t *testing.T) {
	var rateLimiter *RateLimiter

	// If not enabled, connection count should not increase

	rateLimiter = NewRateLimiter(RateLimiterConfig{
		Enabled:        false,
		Whitelist:      "",
		MaxConnections: 0,
	}, glog.CreateRootLogger(glog.CreateLoggerConfigurationFromLevel(glog.TRACE), glog.StandardLogFunction))

	b := rateLimiter.StartConnection("10.0.0.1")

	if !b {
		t.Errorf("Rejected connection expected to be accepted")
	}

	if len(rateLimiter.connectionsCount) != 0 {
		t.Errorf("Connections count has been set")
	}

	rateLimiter.EndConnection("10.0.0.1")

	// If enabled, but MaxConnections = 0, connection count should not increase

	rateLimiter = NewRateLimiter(RateLimiterConfig{
		Enabled:        true,
		Whitelist:      "",
		MaxConnections: 0,
	}, glog.CreateRootLogger(glog.CreateLoggerConfigurationFromLevel(glog.TRACE), glog.StandardLogFunction))

	b = rateLimiter.StartConnection("10.0.0.1")

	if !b {
		t.Errorf("Rejected connection expected to be accepted")
	}

	if len(rateLimiter.connectionsCount) != 0 {
		t.Errorf("Connections count has been set")
	}

	rateLimiter.EndConnection("10.0.0.1")

	// If enabled, but IP is excepted, it should not increase

	rateLimiter = NewRateLimiter(RateLimiterConfig{
		Enabled:        true,
		Whitelist:      "*",
		MaxConnections: 1,
	}, glog.CreateRootLogger(glog.CreateLoggerConfigurationFromLevel(glog.TRACE), glog.StandardLogFunction))

	b = rateLimiter.StartConnection("10.0.0.1")

	if !b {
		t.Errorf("Rejected connection expected to be accepted")
	}

	if len(rateLimiter.connectionsCount) != 0 {
		t.Errorf("Connections count has been set")
	}

	rateLimiter.EndConnection("10.0.0.1")

	// If enabled, but IP is excepted, it should not increase

	rateLimiter = NewRateLimiter(RateLimiterConfig{
		Enabled:        true,
		Whitelist:      "*",
		MaxConnections: 1,
	}, glog.CreateRootLogger(glog.CreateLoggerConfigurationFromLevel(glog.TRACE), glog.StandardLogFunction))

	b = rateLimiter.StartConnection("10.0.0.1")

	if !b {
		t.Errorf("Rejected connection expected to be accepted")
	}

	if len(rateLimiter.connectionsCount) != 0 {
		t.Errorf("Connections count has been set")
	}

	rateLimiter.EndConnection("10.0.0.1")

	rateLimiter = NewRateLimiter(RateLimiterConfig{
		Enabled:        true,
		Whitelist:      "10.0.0.0/8",
		MaxConnections: 1,
	}, glog.CreateRootLogger(glog.CreateLoggerConfigurationFromLevel(glog.TRACE), glog.StandardLogFunction))

	b = rateLimiter.StartConnection("10.0.0.1")

	if !b {
		t.Errorf("Rejected connection expected to be accepted")
	}

	if len(rateLimiter.connectionsCount) != 0 {
		t.Errorf("Connections count has been set")
	}

	rateLimiter.EndConnection("10.0.0.1")

	// Should increase the count

	rateLimiter = NewRateLimiter(RateLimiterConfig{
		Enabled:        true,
		Whitelist:      "10.0.0.0/8",
		MaxConnections: 1,
	}, glog.CreateRootLogger(glog.CreateLoggerConfigurationFromLevel(glog.TRACE), glog.StandardLogFunction))

	b = rateLimiter.StartConnection("20.0.0.1")

	if !b {
		t.Errorf("Rejected connection expected to be accepted")
	}

	if len(rateLimiter.connectionsCount) != 1 || rateLimiter.connectionsCount["20.0.0.1"] != 1 {
		t.Errorf("Connection count does not match")
	}

	b = rateLimiter.StartConnection("20.0.0.1")

	if b {
		t.Errorf("Accepted connection expected to be rejected")
	}

	if len(rateLimiter.connectionsCount) != 1 || rateLimiter.connectionsCount["20.0.0.1"] != 1 {
		t.Errorf("Connection count does not match")
	}

	// End connection

	rateLimiter.EndConnection("20.0.0.1")

	if len(rateLimiter.connectionsCount) != 0 {
		t.Errorf("Connections count is not empty")
	}
}

func TestRequestCount(t *testing.T) {
	counter := RequestCount{
		count:     1,
		timestamp: 1,
	}

	counter.Update(2, 1)

	if counter.count != 0 {
		t.Errorf("count does not match")
	}

	if counter.timestamp != 2 {
		t.Errorf("timestamp does not match")
	}

	counter.count = 2

	counter.Update(2, 1)

	if counter.count != 2 {
		t.Errorf("count does not match")
	}

	if counter.timestamp != 2 {
		t.Errorf("timestamp does not match")
	}

	counter.Update(10, 1)

	if counter.count != 0 {
		t.Errorf("count does not match")
	}

	if counter.timestamp != 10 {
		t.Errorf("timestamp does not match")
	}
}
