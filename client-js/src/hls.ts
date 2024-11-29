// HLS class

"use strict";

import Hls, { type HlsConfig } from "hls.js";
import { type HlsWebSocketCdnClientOptions } from "./options";
import { getLoaderClass } from "./loader";
import { HlsWebSocketCdnClient } from "./client";

/**
 * Extension of HLS.js to pull streams from HLS WebSocket CDN
 */
export class HlsWebSocket extends Hls {
    /**
     * Checks if MSE is supported (required for this library to work)
     * If not supported, you may use regular HLS instead
     * @returns True if supported
     */
    public static isSupported(): boolean {
        return Hls.isSupported();
    }

    // Client
    private wsCdnClient: HlsWebSocketCdnClient;

    /**
     * Constructor. Creates instance of HlsWebSocket
     * @param options HLS WebSocket CDN options
     * @param hlsConfig Base HLS configuration for HLS.js
     */
    constructor(options: HlsWebSocketCdnClientOptions, hlsConfig?: Partial<HlsConfig>) {
        const client = new HlsWebSocketCdnClient(options);

        super({
            ...(hlsConfig || {}),
            loader: getLoaderClass(client, options),
        });

        this.wsCdnClient = client;
    }

    /**
     * Starts pulling the stream
     */
    public start() {
        this.wsCdnClient.start(); // Start the websocket client

        super.loadSource(document.location.protocol + "//hls.internal/index.m3u8"); // Load dummy URL
    }

    /**
     * Closes any connections and frees any resources
     */
    public override destroy() {
        super.destroy();
        this.wsCdnClient.close();
    }
}
