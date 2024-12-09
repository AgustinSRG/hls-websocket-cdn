// Options

"use strict";

// Options of the HLS websocket CDN client
export interface HlsWebSocketCdnClientOptions {
    /**
     * URL to connect to the CDN server
     */
    cdnServerUrl: string;

    /**
     * ID of the stream to pull from the CDN
     */
    streamId: string;

    /**
     * Token to authenticate to the CDN server
     */
    authToken: string;

    /**
     * Max delay in seconds
     * If player is playing with more delay than this, automatically seek.
     * Default: 30
     */
    maxDelay?: number;

    /**
     * Max duration of the SourceBuffer in seconds
     * If this duration is exceeded, oldest data will be removed from the buffer
     * By default, double the value of maxDelay
     */
    maxBufferDuration?: number;

    /**
     * Max length for the segment queue
     * The segments are appended to the queue, waiting for them to be processed
     * It must be limited to prevent a memory crash if the processing is too slow
     * Default: 32
     */
    maxSegmentQueueLength?: number;

    /**
     * Timeout (milliseconds) to start pulling the stream
     * Default: 30000
     */
    timeout?: number;

    /**
     * True to log debug messages
     */
    debug?: boolean;
}
