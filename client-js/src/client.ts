// Client

"use strict";

import { type CdnWebSocketMessage, parseCdnWebSocketMessage, serializeCdnWebSocketMessage } from "./message";
import { type HlsWebSocketCdnClientOptions } from "./options";

/**
 * Error state for the client
 */
export type HlsWebSocketCdnClientErrorState = "error-timeout" | "error-auth";

const HEARTBEAT_PERIOD = 30 * 1000;

const RECONNECT_DELAY = 1000;

/**
 * HLS websocket CDN client
 */
export class HlsWebSocketCdnClient {
    /**
     * Client options
     */
    private options: HlsWebSocketCdnClientOptions;

    /**
     * True if the client is closed
     */
    private closed: boolean;

    /**
     * Error state of the client
     */
    private error: HlsWebSocketCdnClientErrorState | null;

    /**
     * WebSocket used for the connection to the server
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
     * True if playlist is ready
     */
    private playlistReady: boolean;

    /**
     * Next fragment duration
     */
    private nextFragmentDuration: number;

    /**
     * Function to call on fragment received
     */
    private onFragment: (duration: number, data: ArrayBuffer) => void;

    /**
     * Function to call on close
     */
    private onClose: (err: HlsWebSocketCdnClientErrorState | null) => void;

    /**
     * Constructor. Creates instance of HlsWebSocketCdnClient
     * @param options The client options.
     * @param onFragment Function to call when fragments are received
     * @param onClose Function to call on close
     */
    constructor(options: HlsWebSocketCdnClientOptions, onFragment: (duration: number, data: ArrayBuffer) => void, onClose: (err: HlsWebSocketCdnClientErrorState | null) => void) {
        this.options = options;
        this.onFragment = onFragment;
        this.onClose = onClose;

        this.closed = false;
        this.error = null;
        this.ws = null;
        this.reconnectTimer = null;
        this.heartbeatTimer = null;
        this.timeoutTimer = null;
        this.lastReceivedMessage = 0;
        this.nextFragmentDuration = 0;
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

                const parsedMessage = parseCdnWebSocketMessage(ev.data + "");

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

        this.ws.send(serializeCdnWebSocketMessage({
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

        this.ws.send(serializeCdnWebSocketMessage({
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
    private receiveFragmentMetadata(msg: CdnWebSocketMessage) {
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

        try {
            this.onFragment(this.nextFragmentDuration, data);
        } catch (ex) {
            if (this.options.debug) {
                console.error(ex);
            }
        }
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
    public close(error?: HlsWebSocketCdnClientErrorState) {
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

        this.onClose(this.error);
    }
}