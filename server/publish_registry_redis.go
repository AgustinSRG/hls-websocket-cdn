// Publish registry with Redis database

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Publish registry config
type RedisPublishRegistryConfig struct {
	// Host
	Host string

	// Port
	Port int

	// Password
	Password string

	// True to connect with TLS
	UseTls bool

	// Number of seconds for the publish registry to be refreshed
	PublishRefreshIntervalSeconds int
}

// Creates new instance of RedisPublishRegistry
func NewRedisPublishRegistry(config RedisPublishRegistryConfig) (*RedisPublishRegistry, error) {
	var redisClient *redis.Client

	if config.UseTls {
		redisClient = redis.NewClient(&redis.Options{
			Addr:      config.Host + ":" + fmt.Sprint(config.Port),
			Password:  config.Password,
			TLSConfig: &tls.Config{},
		})
	} else {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     config.Host + ":" + fmt.Sprint(config.Port),
			Password: config.Password,
		})
	}

	return &RedisPublishRegistry{
		config:      config,
		redisClient: redisClient,
	}, nil
}

// Publish registry
type RedisPublishRegistry struct {
	// Configuration
	config RedisPublishRegistryConfig

	// Redis client
	redisClient *redis.Client
}

// Gets the interval to announce to the registry
func (pr *RedisPublishRegistry) GetAnnounceInterval() time.Duration {
	return time.Duration(pr.config.PublishRefreshIntervalSeconds) * time.Second
}

// Gets the URL of the publishing server given the stream ID
func (pr *RedisPublishRegistry) GetPublishingServer(streamId string) (string, error) {
	res := pr.redisClient.Get(context.Background(), streamId)

	if res.Err() != nil {
		return "", res.Err()
	}

	return res.Val(), nil
}

// Announces to the publish database that this server [url]
// has a stream with [streamId] being published
// This method must be called periodically, as the value is temporal
func (pr *RedisPublishRegistry) AnnouncePublishedStream(streamId string, url string) error {
	status := pr.redisClient.Set(context.Background(), streamId, url, time.Duration(pr.config.PublishRefreshIntervalSeconds)*2*time.Second)
	return status.Err()
}
