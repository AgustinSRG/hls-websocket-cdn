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
     * Internal playlist size
     * The max number of fragments to keep in memory
     */
    internalPlaylistSize?: number;

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
