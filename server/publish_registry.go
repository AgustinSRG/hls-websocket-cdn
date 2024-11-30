// Publish registry

package main

import "time"

type PublishRegistry interface {
	// Gets the URL of the publishing server given the stream ID
	GetPublishingServer(streamId string) (string, error)

	// Gets the interval to announce to the registry
	GetAnnounceInterval() time.Duration

	// Announces to the publish database that this server [url]
	// has a stream with [streamId] being published
	// This method must be called periodically, as the value is temporal
	AnnouncePublishedStream(streamId string, url string) error
}
