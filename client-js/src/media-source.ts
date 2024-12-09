// Media source controller

"use strict";

import { InitData, parseInitSegment } from "./mp4-tools";
import { type HlsWebSocketCdnClientOptions } from "./options";

interface MediaSegmentQueueEntry {
    /**
     * Duration of the segment in seconds
     */
    duration: number;

    /**
     * Init segment
     */
    initSegment: ArrayBuffer;

    /**
     * Data segment
     */
    data: ArrayBuffer;
}

const DEFAULT_QUEUE_MAX_LENGTH = 32;
const DEFAULT_MAX_DELAY_SECONDS = 30;

/**
 * Controller for the media source
 */
export class MediaSourceController {
    // Options
    private options: HlsWebSocketCdnClientOptions;

    // Max delay in seconds
    private maxDelay: number;

    // MediaSource instance
    private mediaSource: MediaSource;
    private mediaSourceReady: boolean;

    // Ended?
    private ended: boolean;
    private endedReady: boolean;

    // SourceBuffer instance
    private sourceBuffer: SourceBuffer | null;
    private sourceBufferDuration: number;
    private sourceBufferMaxDuration: number;

    // True if there is a pending update on the SourceBuffer
    private sourceBufferPendingUpdate: boolean;

    // Duration to add when the update is done
    private sourceBufferPendingUpdateDuration: number;

    // Segment queue
    private queue: MediaSegmentQueueEntry[];
    private queueMaxLength: number;

    // Detected codecs
    private audioCodec: string;
    private videoCodec: string;
    private codecReady: boolean;

    // Event listeners
    public onError: (err: Error) => void;

    constructor(options: HlsWebSocketCdnClientOptions) {
        this.options = options;

        this.maxDelay = options.maxDelay || DEFAULT_MAX_DELAY_SECONDS;

        this.mediaSource = new MediaSource();
        this.mediaSourceReady = false;

        this.ended = false;
        this.endedReady = false;

        this.sourceBuffer = null;
        this.sourceBufferDuration = 0;
        this.sourceBufferMaxDuration = options.maxBufferDuration || (this.maxDelay * 2);

        this.sourceBufferPendingUpdate = false;
        this.sourceBufferPendingUpdateDuration = 0;

        this.queue = [];
        this.queueMaxLength = options.maxSegmentQueueLength || DEFAULT_QUEUE_MAX_LENGTH;

        this.audioCodec = "";
        this.videoCodec = "";
        this.codecReady = false;

        this.mediaSource.addEventListener("sourceopen", this.onSourceOpen.bind(this));
    }

    /**
     * Called when MediaSource becomes ready
     */
    private onSourceOpen() {
        this.mediaSourceReady = true;
        this.onUpdated();
    }

    /**
     * Gets MIME type for SourceBuffer
     * @returns The MIME type
     */
    private getMimeType(): string {
        const codecs = [this.audioCodec, this.videoCodec].filter(a => !!a).join(",");
        return `video/mp4; codecs="${codecs}"`;
    }

    /**
     * Call on any update to continue
     */
    private onUpdated() {
        if (!this.sourceBuffer) {
            // No source buffer yet. Attempt to create it

            if (!this.mediaSourceReady || !this.codecReady) {
                return; // Not ready yet
            }

            this.sourceBuffer = this.mediaSource.addSourceBuffer(this.getMimeType());
            this.sourceBuffer.addEventListener("updateend", this.onUpdated.bind(this));
            this.sourceBufferDuration = 0;
        }

        if (this.queue.length === 0 && this.ended && !this.endedReady) {
            this.endedReady = true;
            this.mediaSource.endOfStream();
            return;
        }

        if (this.sourceBuffer.updating) {
            // Updating, wait until it is done
            return;
        }

        if (this.sourceBufferPendingUpdate) {
            // After update is done, update the duration
            this.sourceBufferPendingUpdate = false;
            this.sourceBufferDuration += this.sourceBufferPendingUpdateDuration;
        }

        // Check if the duration is too big
        
    }

    /**
     * Adds a segment to the media source
     * @param duration The duration of the segment
     * @param initSegment Initial MP4 segment with the metadata
     * @param data Data segment
     */
    public addSegment(duration: number, initSegment: ArrayBuffer, data: ArrayBuffer) {
        if (this.ended) {
            throw new Error("Called addSegment() after ended() was called")
        }

        if (this.queue.length >= this.queueMaxLength) {
            this.queue.shift(); // Discord one element
        }

        this.queue.push({
            duration: duration,
            initSegment: initSegment,
            data: data,
        });

        if (!this.codecReady) {
            // Try get the codecs from the initSegment

            let initData: InitData;

            try {
                initData = parseInitSegment(new Uint8Array(initSegment));
            } catch (ex) {
                if (this.onError) {
                    this.onError(ex);
                }
            }

            if (initData) {
                this.audioCodec = initData.audio ? initData.audio.codec : "";
                this.videoCodec = initData.video ? initData.video.codec : "";
                this.codecReady = true;
            }
        }

        this.onUpdated();
    }

    /**
     * Indicates there are no more segments
     */
    public end() {
        this.ended = true;
        this.onUpdated();
    }

    /**
     * Releases all resources
     */
    public destroy() {
        this.ended = true;
        this.queue = [];
        this.onUpdated();
    }
}
