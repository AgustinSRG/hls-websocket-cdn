// Client

"use strict";

import { type CdnWebsocketMessage, parseCdnWebsocketMessage, serializeCdnWebsocketMessage } from "./message";
import { type HlsWebsocketCdnClientOptions } from "./options";

/**
 * Error state for the client
 */
export type HlsWebsocketCdnClientErrorState = "error-timeout" | "error-auth";

const HEARTBEAT_PERIOD = 30 * 1000;

const RECONNECT_DELAY = 1000;

/**
 * Callback to receive a playlist
 */
export type PlaylistRequestCallback = (err: HlsWebsocketCdnClientErrorState | null, playlist: string) => void;

/**
 * Fragment of the playlist
 */
export interface PlaylistFragment {
    /**
     * Index of the fragment
     */
    index: number;

    /**
     * Fragment duration (seconds)
     */
    duration: number;

    /**
     * Fragment data
     */
    data: ArrayBuffer;
}

/**
 * HLS websocket CDN client
 */
export class HlsWebsocketCdnClient {
    /**
     * Client options
     */
    private options: HlsWebsocketCdnClientOptions;

    /**
     * True if the client is closed
     */
    private closed: boolean;

    /**
     * Error state of the client
     */
    private error: HlsWebsocketCdnClientErrorState | null;

    /**
     * Websocket used for the connection to the server
     */
    private ws: WebSocket | null;

    /**
     * Reconnect timer (timeout)
     */
    private reconnectTimer: number | NodeJS.Timeout | null;

    /**
     * Heartbeat timer (Interval)
     */
    private heartbeatTimer: number | NodeJS.Timeout | null;

    /**
     * Timeout timer (timeout)
     */
    private timeoutTimer:  number | NodeJS.Timeout | null;

    /**
     * Timestamp of the last message received from the server
     */
    private lastReceivedMessage: number;

    /**
     * List of fragments kept on the playlist
     */
    private playlist: PlaylistFragment[];

    /**
     * List of stale fragments
     */
    private staleFragments: PlaylistFragment[];

    /**
     * True if playlist is ready
     */
    private playlistReady: boolean;

    /**
     * Max size of the playlist
     */
    private playlistMaxSize: number;

    /**
     * Index for the next fragment
     */
    private nextFragmentIndex: number;

    /**
     * Next fragment duration
     */
    private nextFragmentDuration: number;

    /**
     * Counter to generate unique identifiers for listeners
     */
    private nextListenerId: number;

    /**
     * Map of listeners fot the playlist
     */
    private playlistListeners: Map<number, PlaylistRequestCallback>;

    /**
     * Constructor. Creates instance of HlsWebsocketCdnClient
     * @param options The client options.
     */
    constructor(options: HlsWebsocketCdnClientOptions) {
        this.options = options;
        this.closed = false;
        this.error = null;
        this.ws = null;
        this.reconnectTimer = null;
        this.heartbeatTimer = null;
        this.timeoutTimer = null;
        this.lastReceivedMessage = 0;
        this.playlist = [];
        this.staleFragments = [];
        this.playlistMaxSize = options.internalPlaylistSize ? options.internalPlaylistSize : 10;
        this.nextFragmentIndex = 0;
        this.nextFragmentDuration = 0;
        this.nextListenerId = 0;
        this.playlistListeners = new Map();
    }

    /**
     * Clears socket
     */
    private clearSocket() {
        if (this.ws) {
            this.ws.onopen = null;
            this.ws.onmessage = null;
            this.ws.onclose = null;
            this.ws.onerror = null;
            this.ws.close();
            this.ws = null;
        }
    }

    /**
     * Connects to the server
     */
    private connect() {
        this.clearSocket();

        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer as any);
            this.reconnectTimer = null;
        }

        if (this.heartbeatTimer) {
            clearInterval(this.heartbeatTimer as any);
            this.heartbeatTimer = null;
        }

        this.ws = new WebSocket(this.options.cdnServerUrl);
        this.ws.binaryType = "arraybuffer";

        this.ws.onopen = () => {
            this.authenticate();
            this.lastReceivedMessage = Date.now();
            this.heartbeatTimer = setInterval(this.heartbeat.bind(this), HEARTBEAT_PERIOD);
        };

        this.ws.onmessage = (ev: MessageEvent) => {
            this.lastReceivedMessage = Date.now();

            if (ev.data instanceof ArrayBuffer) {
                // Binary

                this.receiveFragment(ev.data);
            } else {
                // Text

                const parsedMessage = parseCdnWebsocketMessage(ev.data + "");

                switch (parsedMessage.type) {
                    case "E":
                        this.close("error-auth");
                        break;
                    case "F":
                        this.receiveFragmentMetadata(parsedMessage);
                        break;
                    case "CLOSE":
                        if (this.playlistReady) {
                            // Stream ended
                            this.close();
                        }
                        break;
                }
            }
        };

        this.ws.onclose = () => {
            if (this.heartbeatTimer) {
                clearInterval(this.heartbeatTimer as any);
                this.heartbeatTimer = null;
            }

            this.ws = null;

            if (!this.closed) {
                this.reconnect();
            }
        };
    }

    /**
     * Reconnects after closed
     */
    private reconnect() {
        this.reconnectTimer = setTimeout(() => {
            this.reconnectTimer = null;
            this.connect();
        }, RECONNECT_DELAY);
    }

    /**
     * Sends action command with the authentication token
     */
    private authenticate() {
        if (!this.ws) {
            return;
        }

        this.ws.send(serializeCdnWebsocketMessage({
            type: "PULL",
            parameters: new Map([
                ["stream", this.options.streamId],
                ["auth", this.options.authToken],
            ]),
        }));
    }

    /**
     * Sends heartbeat message to the server
     * and checks for server inactivity
     */
    private heartbeat() {
        if (!this.ws) {
            return;
        }

        this.ws.send(serializeCdnWebsocketMessage({
            type: "H",
        }));

        if (Date.now() - this.lastReceivedMessage > (HEARTBEAT_PERIOD * 2)) {
            // Server inactivity
            this.ws.close();
        }
    }

    /**
     * Receives a fragment metadata message
     * @param msg The message
     */
    private receiveFragmentMetadata(msg: CdnWebsocketMessage) {
        const durationStr = msg.parameters ? msg.parameters.get("duration") : "";

        if (!durationStr) {
            return;
        }

        const duration = parseFloat(durationStr);

        if (isNaN(duration) || !isFinite(duration) || duration < 0) {
            return;
        }

        this.nextFragmentDuration = duration;
    }

    /**
     * Receives a fragment
     * @param data The fragment data
     */
    private receiveFragment(data: ArrayBuffer) {
        if (data.byteLength === 0) {
            return;
        }

        if (!this.nextFragmentDuration) {
            return;
        }

        const fragmentIndex = this.nextFragmentIndex;
        this.nextFragmentIndex++;

        this.playlist.push({
            index: fragmentIndex,
            duration: this.nextFragmentDuration,
            data: data,
        });

        this.nextFragmentDuration = 0;

        if (this.playlist.length > this.playlistMaxSize) {
            // Remove oldest fragment
            this.playlist.shift();
        }

        this.onPlaylistUpdated();
    }

    /**
     * Starts the client
     * (Call once)
     */
    public start() {
        if (this.timeoutTimer) {
            clearTimeout(this.timeoutTimer);
            this.timeoutTimer = null;
        }

        this.timeoutTimer = setTimeout(this.onTimeout.bind(this), this.options.timeout ? this.options.timeout : 30000);
        this.connect();
    }

    /**
     * Gets the playlist asynchronously
     * @param callback The callback
     * @returns The listener id
     */
    public getPlaylist(callback: PlaylistRequestCallback): number {
        if (this.error) {
            // Error
            callback(this.error, "");
            return -1;
        } else if (this.playlistReady) {
            // Playlist ready
            this.staleFragments = this.playlist.slice();
            callback(null, this.makePlaylist());
            return -1;
        }

        const listenerId = this.nextListenerId;
        this.nextListenerId++;

        this.playlistListeners.set(listenerId, callback);
    }

    /**
     * Removes playlist listener
     * @param id The listener ID
     */
    public removePlaylistListener(id: number) {
        if (id < 0) {
            return;
        }
        this.playlistListeners.delete(id);
    }

    /**
     * Generates the playlist
     * @returns The playlist content
     */
    private makePlaylist(): string {
        const lines = [
            "#EXTM3U",
            "#EXT-X-PLAYLIST-TYPE:EVENT",
            "#EXT-X-VERSION:3",
            "#EXT-X-TARGETDURATION:1",
            "#EXT-X-MEDIA-SEQUENCE:" + (this.playlist.length > 0 ? this.playlist[0].index : 0),
        ];

        for (const fragment of this.playlist) {
            lines.push("#EXTINF:" + fragment.duration.toFixed(6));
            lines.push("" + fragment.index + ".ts");
        }

        if (this.closed) {
            lines.push("#EXT-X-ENDLIST");
        }

        return lines.join("\n") + "\n";
    }

    /**
     * Gets fragment data
     * @param index The fragment index
     * @returns The fragment data
     */
    public getFragment(index: number): ArrayBuffer | null {
        for (const f of this.playlist) {
            if (f.index === index) {
                return f.data;
            }
        }

        for (const f of this.staleFragments) {
            if (f.index === index) {
                return f.data;
            }
        }

        return null;
    }

    /**
     * Called when the playlist gets an update
     */
    private onPlaylistUpdated() {
        // Cancel timeout, as the playlist has been finally updated
        if (this.timeoutTimer) {
            clearTimeout(this.timeoutTimer);
            this.timeoutTimer = null;
        }

        // Playlist is now ready
        this.playlistReady = true;

        // Pre-clear listeners

        const listenersToUpdate: PlaylistRequestCallback[] = [];

        this.playlistListeners.forEach(listener => {
            listenersToUpdate.push(listener);
        });

        this.playlistListeners.clear();

        // Send playlist to the listeners

        const playlist = this.makePlaylist();

        listenersToUpdate.forEach(listener => {
            listener(this.error, playlist);
        });

        if (listenersToUpdate.length > 0) {
            this.staleFragments = this.playlist.slice();
        }
    }

    /**
     * Called on timeout
     */
    private onTimeout() {
        this.timeoutTimer = null;
        this.close("error-timeout");
    }

    /**
     * Closes the client
     * @param error 
     */
    public close(error?: HlsWebsocketCdnClientErrorState) {
        if (error) {
            this.error = error;
        }

        this.clearSocket();

        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer as any);
            this.reconnectTimer = null;
        }

        if (this.heartbeatTimer) {
            clearInterval(this.heartbeatTimer as any);
            this.heartbeatTimer = null;
        }

        this.closed = true;

        this.onPlaylistUpdated();
    }
}