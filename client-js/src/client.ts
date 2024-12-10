// Client

"use strict";

import { type CdnWebSocketMessage, parseCdnWebSocketMessage, serializeCdnWebSocketMessage } from "./message";
import { type HlsWebSocketCdnClientOptions } from "./options";

/**
 * Error state for the client
 */
export type HlsWebSocketCdnClientErrorState = "error-timeout" | "error-auth";

const HEARTBEAT_PERIOD = 30 * 1000;
const HEARTBEAT_DEADLINE = HEARTBEAT_PERIOD * 2;

const RECONNECT_DELAY = 1000;

const LOG_PREFIX = "[HlsWebSocket] [HlsWebSocketCdnClient] [DEBUG] ";

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
     * True if ready
     */
    private ready: boolean;

    /**
     * Next fragment duration
     */
    private nextFragmentDuration: number;

    /**
     * Event function to call on fragment received
     */
    public onFragment: (duration: number, data: ArrayBuffer) => void;

    /**
     * Event function to call on close
     */
    public onClose: (err: HlsWebSocketCdnClientErrorState | null) => void;

    /**
     * Constructor. Creates instance of HlsWebSocketCdnClient
     * @param options The client options.
     */
    constructor(options: HlsWebSocketCdnClientOptions) {
        this.options = options;
        this.onFragment = null;
        this.onClose = null;

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
            if (this.options.debug) {
                console.log(LOG_PREFIX + "Connected");
            }
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
                    case "OK":
                        if (this.options.debug) {
                            console.log(LOG_PREFIX + "OK received. Ready to receive fragments.");
                        }
                        break;
                    case "E":
                        if (this.options.debug) {
                            console.log(LOG_PREFIX + "Error from server: " + parsedMessage.parameters.get("code") + " - " + parsedMessage.parameters.get("message"));
                        }
                        this.close("error-auth");
                        break;
                    case "F":
                        this.receiveFragmentMetadata(parsedMessage);
                        break;
                    case "CLOSE":
                        if (this.ready) {
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

            if (this.options.debug) {
                console.log(LOG_PREFIX + "Disconnected");
            }

            this.ws = null;

            if (!this.closed) {
                this.reconnect();
            }
        };

        this.ws.onerror = err => {
            if (this.options.debug) {
                console.log(LOG_PREFIX + "Error", err);
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
        if (this.options.debug) {
            console.log(LOG_PREFIX + "Scheduled reconnection (" + RECONNECT_DELAY + "ms)");
        }
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
                ["max_initial_fragments", (this.options.maxInitialFragments || "") + ""],
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

        if (Date.now() - this.lastReceivedMessage > HEARTBEAT_DEADLINE) {
            // Server inactivity
            if (this.options.debug) {
                console.log("[HlsWebSocket] [Mp4Muxer] [Client] Server inactivity");
            }
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

        if (this.timeoutTimer) {
            clearTimeout(this.timeoutTimer);
            this.timeoutTimer = null;
        }

        this.ready = true;

        if (this.options.debug) {
            console.log(LOG_PREFIX + "Fragment received (Duration=" + this.nextFragmentDuration + "s, Size=" + data.byteLength + " bytes)");
        }

        if (this.onFragment) {
            try {
                this.onFragment(this.nextFragmentDuration, data);
            } catch (ex) {
                if (this.options.debug) {
                    console.error(ex);
                }
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
        if (this.options.debug) {
            console.log(LOG_PREFIX + "Timed out");
        }
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

        if (this.onClose) {
            this.onClose(this.error);
        }
    }

    /**
     * Releases resources and closes the connection
     */
    public destroy() {
        this.onClose = null;
        this.onFragment = null;

        this.close();
    }
}