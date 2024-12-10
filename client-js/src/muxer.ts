// TLS to MP4 transmuxer

"use strict";

import * as MuxJS from "mux.js";
import { type HlsWebSocketCdnClientOptions } from "./options";

/**
 * Fragment queue entry
 */
interface FragmentQueueEntry {
    duration: number;

    /**
     * Fragment data
     */
    data: ArrayBuffer;
}

/**
 * Transmuxer segment
 */
type TransmuxerSegment = { initSegment: Uint8Array | ArrayBuffer, data: Uint8Array | ArrayBuffer };

/**
 * Transmuxer internal interface
 */
interface Transmuxer {
    // Adds listener for data
    on(ev: "data", listener: (segment: TransmuxerSegment) => void): void;
    // Pushes data to be transmuxed
    push(data: Uint8Array): void;
    // Flushes the data
    flush(): void;
}

const DEFAULT_QUEUE_MAX_LENGTH = 32;

const LOG_PREFIX = "[HlsWebSocket] [Mp4Muxer] [DEBUG] ";

/**
 * Muxer to transform TS to MP4 to MediaSource compatibility
 */
export class Mp4Muxer {
    // Options
    private options: HlsWebSocketCdnClientOptions;

    // Queue
    private queue: FragmentQueueEntry[];
    private queueMaxLength: number;

    // Transmuxer
    private transmuxer: Transmuxer;

    // Is the transmuxer busy?
    private busy: boolean;

    // Ended?
    private ended: boolean;
    private endedReady: boolean;

    // Init segment
    private initSegment: Uint8Array;

    // Next segment duration
    private nextSegmentDuration: number;

    // Events

    /**
     * Event function called when a segment is ready
     */
    public onSegment: (duration: number, initSegment: Uint8Array, data: Uint8Array) => void;

    /**
     * Event function called after the muxer is done
     */
    public onEnded: () => void;

    /**
     * Constructor of Mp4Muxer
     * @param options The options
     */
    constructor(options: HlsWebSocketCdnClientOptions) {
        this.options = options;

        this.onSegment = null;
        this.onEnded = null;

        this.queue = [];
        this.queueMaxLength = options.maxFragmentQueueLength || DEFAULT_QUEUE_MAX_LENGTH;

        this.transmuxer = new MuxJS.mp4.Transmuxer();

        this.transmuxer.on("data", (segment: TransmuxerSegment) => {
            this.busy = false;

            const initSegment = segment.initSegment ? new Uint8Array(segment.initSegment) : this.initSegment;

            this.initSegment = initSegment;

            const data = segment.data ? new Uint8Array(segment.data) : new Uint8Array([]);

            if (this.options.debug) {
                console.log(LOG_PREFIX + "Segment transmuxed to MP4 (" + this.nextSegmentDuration + " seconds) (initSegment " + initSegment.length + " bytes, data " + data.length + " bytes)");
            }

            if (this.onSegment) {
                this.onSegment(this.nextSegmentDuration, initSegment, data);
                this.nextSegmentDuration = 0;
            }

            this.onUpdated();
        });

        this.busy = false;

        this.ended = false;
        this.endedReady = false;

        this.initSegment = new Uint8Array([]);

        this.nextSegmentDuration = 0;
    }

    /**
     * Called on update
     */
    private onUpdated() {
        if (this.busy) {
            // Busy, wait to be done
            return;
        }

        if (this.queue.length === 0) {
            // No more fragments, yet...

            if (this.ended && !this.endedReady) {
                this.endedReady = true;

                if (this.options.debug) {
                    console.log(LOG_PREFIX + "Reached the end of the stream");
                }

                if (this.onEnded) {
                    this.onEnded(); // Notify the ending
                }
            }

            return;
        }

        const fragmentToProcess = this.queue.shift();

        this.busy = true;

        if (this.options.debug) {
            console.log(LOG_PREFIX + "Pushing data to the transmuxer: " + fragmentToProcess.data.byteLength + " bytes (" + fragmentToProcess.duration + " seconds).");
        }

        this.nextSegmentDuration = fragmentToProcess.duration;

        this.transmuxer.push(new Uint8Array(fragmentToProcess.data));
        this.transmuxer.flush();
    }

    /**
     * Adds a fragment to the queue
     * @param data The fragment data
     */
    public addFragment(duration: number, data: ArrayBuffer) {
        if (this.ended) {
            throw new Error("Called addFragment() after end() was called");
        }

        if (this.queue.length >= this.queueMaxLength) {
            if (this.options.debug) {
                console.log(LOG_PREFIX + "Queue cannot keep up. Discarding one element.");
            }
            this.queue.shift(); // Discard one element
        }

        this.queue.push({
            duration: duration,
            data: data,
        });

        if (this.options.debug) {
            console.log(LOG_PREFIX + "Fragment added to the queue. Duration: " + duration + " seconds. Size: " + data.byteLength + " bytes.");
        }

        this.onUpdated();
    }

    /**
     * Indicate there are no more fragments to add
     */
    public end() {
        this.ended = true;
        this.onUpdated();
    }

    public destroy() {
        this.onEnded = null;
        this.onSegment = null;
        this.ended = true;
        this.queue = [];
        this.onUpdated();
    }
}