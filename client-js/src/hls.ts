// HLS WebSocket controller

"use strict";

import { HlsWebSocketCdnClient, type HlsWebSocketCdnClientErrorState } from "./client";
import { MediaSourceController } from "./media-source";
import { Mp4Muxer } from "./muxer";
import { type HlsWebSocketCdnClientOptions } from "./options";

/**
 * Map of possible events of HlsWebSocket
 */
export type HlsWebSocketEventMap = {
    /**
     * Error event
     * @param error The error 
     */
    "error": (error: Error) => void;

    /**
     * Emitted when the connection to the server is closed 
     */
    "close": () => void;

    /**
     * Emitted when there are no more fragments, and the media source has ended
     */
    "ended": () => void;
};

/**
 * Controller to pull HLS from a CDN server
 * and sent it to a media element
 */
export class HlsWebSocket {
    /**
     * Checks if MSE is supported (required for this library to work)
     * If not supported, you may use regular HLS instead
     * @returns True if supported
     */
    public static isSupported(): boolean {
        return !!window.MediaSource && !!window.SourceBuffer && MediaSource.isTypeSupported("video/mp4; codecs=\"avc1.42E01E,mp4a.40.2\"");
    }

    /**
     * Options
     */
    private options: HlsWebSocketCdnClientOptions;

    /**
     * Client
     */
    private client: HlsWebSocketCdnClient;

    /**
     * Muxer
     */
    private muxer: Mp4Muxer;

    /**
     * Media source controller
     */
    private mediaSourceController: MediaSourceController;

    /**
     * Event listeners
     */
    private eventListeners: { [eventName: string]: ((...args: any[]) => void)[] };

    /**
     * Constructor for HlsWebSocket
     * @param options The options
     */
    constructor(options: HlsWebSocketCdnClientOptions) {
        this.options = options;

        this.client = new HlsWebSocketCdnClient(options);
        this.muxer = new Mp4Muxer(options);
        this.mediaSourceController = new MediaSourceController(options);

        this.eventListeners = Object.create(null);

        this.client.onFragment = (duration: number, data: ArrayBuffer) => {
            this.muxer.addFragment(duration, data);
        };

        this.client.onClose = (err: HlsWebSocketCdnClientErrorState | null) => {
            if (err) {
                this.trigger("error", new Error("Client error: " + err));
            }

            this.trigger("close");

            // Client closed, notify the muxer
            this.muxer.end();
        };

        this.muxer.onSegment = (duration: number, initSegment: Uint8Array, data: Uint8Array) => {
            this.mediaSourceController.addSegment(duration, initSegment, data);
        };

        this.muxer.onEnded = () => {
            this.mediaSourceController.end();
        };

        this.mediaSourceController.onError = (err) => {
            this.trigger("error", new Error("Media source error: " + err));
        };

        this.mediaSourceController.onEnded = () => {
            this.trigger("ended");
        };
    }

    /**
     * Starts the client
     */
    public start() {
        this.client.start();
    }

    /**
     * Triggers an event
     * @param eventName The event name
     * @param args The arguments
     */
    public trigger<K extends keyof HlsWebSocketEventMap>(eventName: K, ...args: Parameters<HlsWebSocketEventMap[K]>) {
        if (this.eventListeners[eventName]) {
            for (const handler of this.eventListeners[eventName]) {
                try {
                    handler(...args);
                } catch (ex) {
                    console.error(ex);
                }
            }
        }
    }

    /**
     * Adds event listener
     * @param eventName The event name
     * @param handler The event handler
     */
    public addEventListener<K extends keyof HlsWebSocketEventMap>(eventName: K, handler: HlsWebSocketEventMap[K]) {
        if (!this.eventListeners[eventName]) {
            this.eventListeners[eventName] = [];
        }
        this.eventListeners[eventName].push(handler);
    }

    /**
     * Removes event listener
     * @param eventName The event name
     * @param handler The event handler (the used in addEventListener)
     */
    public removeEventListener<K extends keyof HlsWebSocketEventMap>(eventName: K, handler: HlsWebSocketEventMap[K]) {
        if (!this.eventListeners[eventName]) {
            return;
        }
        const i = this.eventListeners[eventName].indexOf(handler);
        if (i >= 0) {
            this.eventListeners[eventName].splice(i, 1);
            if (this.eventListeners[eventName].length === 0) {
                delete this.eventListeners[eventName];
            }
        }
    }

    /**
     * Attaches to media element for playback
     * @param mediaElement The media element
     */
    public attachMedia(mediaElement: HTMLMediaElement) {
        this.mediaSourceController.attachMedia(mediaElement);
    }

    /**
     * Detaches from the current attached media element
     */
    public detachMedia() {
        this.mediaSourceController.detachMedia();
    }

    /**
     * Sets delay options
     * @param delay The desired delay (seconds)
     * @param maxDelay The max delay (seek if exceeded) (seconds)
     */
    public setDelayOptions(delay: number, maxDelay?: number) {
        maxDelay = Math.max(delay, maxDelay || (delay + 1));

        this.options.delay = delay;
        this.options.maxDelay = maxDelay;

        this.mediaSourceController.setDelayOptions(delay, maxDelay);
    }

    /**
     * Releases all resources
     */
    public destroy() {
        this.eventListeners = Object.create(null);
        this.client.destroy();
        this.muxer.destroy();
        this.mediaSourceController.destroy();
    }
}