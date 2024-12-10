// Media source controller

"use strict";

import { type InitData, parseInitSegment } from "./mp4-tools";
import { type HlsWebSocketCdnClientOptions } from "./options";

/**
 * Segment queue entry
 */
interface MediaSegmentQueueEntry {
    /**
     * Duration in seconds
     */
    duration: number;

    /**
     * Init segment
     */
    initSegment: Uint8Array;

    /**
     * Data segment
     */
    data: Uint8Array;
}

const DEFAULT_QUEUE_MAX_LENGTH = 32;
const DEFAULT_DELAY_SECONDS = 30;

const LOG_PREFIX = "[HlsWebSocket] [MediaSourceController] [DEBUG] ";

/**
 * Controller for the media source
 */
export class MediaSourceController {
    // Options
    private options: HlsWebSocketCdnClientOptions;

    // Delay in seconds
    private delay: number;

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

    // Segment queue
    private queue: MediaSegmentQueueEntry[];
    private queueMaxLength: number;

    // Counts number of appended segments
    private segmentAppendedCount: number;

    // Detected codecs
    private audioCodec: string;
    private videoCodec: string;
    private codecReady: boolean;

    // Media element
    private mediaElement: HTMLMediaElement | null;
    private mediaObjectUrl: string;
    private onMediaElementUpdateHandler: () => void;

    // Event listeners

    /**
     * Event function called on error
     */
    public onError: (err: Error) => void;

    /**
     * Event function called after the MediaSource reaches its end
     */
    public onEnded: () => void;

    /**
     * Constructor of MediaSourceController
     * @param options The options
     */
    constructor(options: HlsWebSocketCdnClientOptions) {
        this.options = options;

        this.onError = null;
        this.onEnded = null;

        this.delay = options.delay || DEFAULT_DELAY_SECONDS;
        this.maxDelay = Math.max(this.delay, options.maxDelay || this.delay);

        this.mediaSource = new MediaSource();
        this.mediaSourceReady = false;

        this.ended = false;
        this.endedReady = false;

        this.sourceBuffer = null;
        this.sourceBufferDuration = 0;
        this.sourceBufferMaxDuration = options.maxBufferDuration || (this.maxDelay * 2);

        this.sourceBufferPendingUpdate = false;

        this.queue = [];
        this.queueMaxLength = options.maxSegmentQueueLength || DEFAULT_QUEUE_MAX_LENGTH;

        this.segmentAppendedCount = 0;

        this.audioCodec = "";
        this.videoCodec = "";
        this.codecReady = false;

        this.mediaElement = null;
        this.mediaObjectUrl = "";
        this.onMediaElementUpdateHandler = () => {
            this.onMediaElementUpdate();
        };

        this.mediaSource.addEventListener("sourceopen", this.onSourceOpen.bind(this));
    }

    /**
     * Called when MediaSource becomes ready
     */
    private onSourceOpen() {
        this.mediaSourceReady = true;
        this.mediaSource.duration = 0;
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

            const mimeType = this.getMimeType();

            this.sourceBuffer = this.mediaSource.addSourceBuffer(mimeType);
            this.sourceBuffer.addEventListener("updateend", this.onUpdated.bind(this));
            this.sourceBufferDuration = 0;

            if (this.options.debug) {
                console.log(LOG_PREFIX + "SourceBuffer created for MIME type: " + mimeType);
            }
        }

        if (this.queue.length === 0 && this.ended && !this.endedReady) {
            this.endedReady = true;
            this.mediaSource.endOfStream();

            if (this.onEnded) {
                this.onEnded();
            }

            if (this.options.debug) {
                console.log(LOG_PREFIX + "MediaSource reached the end of the stream");
            }
            return;
        }

        if (this.sourceBuffer.updating) {
            // Updating, wait until it is done
            if (this.options.debug) {
                console.log(LOG_PREFIX + "SourceBuffer is updating.");
            }
            return;
        }

        if (this.sourceBufferPendingUpdate) {
            // After update is done, update the duration
            this.sourceBufferPendingUpdate = false;

            let bufferDuration = 0;

            let rangeStart = 0;
            let rangeEnd = 0;

            if (this.sourceBuffer.buffered) {
                rangeStart = this.sourceBuffer.buffered.length > 0 ? this.sourceBuffer.buffered.start(0) : 0;

                for (let i = 0; i < this.sourceBuffer.buffered.length; i++) {
                    const start = this.sourceBuffer.buffered.start(i);
                    const end = this.sourceBuffer.buffered.end(i);

                    rangeEnd = Math.max(rangeEnd, end);

                    bufferDuration += (end - start);
                }
            }

            this.sourceBufferDuration = bufferDuration;

            if (this.options.debug) {
                console.log(LOG_PREFIX + "Source buffer time updated: " + this.sourceBufferDuration + " seconds. Buffered range = [" + rangeStart + ", " + rangeEnd + "]");
            }

            // Check if the duration is too big
            if (this.sourceBufferDuration > 0 && this.sourceBufferDuration > this.sourceBufferMaxDuration) {
                const timeToRemove = this.sourceBufferDuration - this.sourceBufferMaxDuration;

                const timeToRemoveStart = this.sourceBuffer.buffered.length > 0 ? this.sourceBuffer.buffered.start(0) : 0;
                const timeToRemoveEnd = timeToRemoveStart + timeToRemove;

                this.sourceBuffer.remove(timeToRemoveStart, timeToRemoveEnd);

                if (this.options.debug) {
                    console.log(LOG_PREFIX + "Removed from SourceBuffer (" + timeToRemoveStart + ", " + timeToRemoveEnd + ")");
                }

                return;
            }
        }


        if (this.queue.length === 0) {
            if (this.options.debug) {
                console.log(LOG_PREFIX + "Queue is empty.");
            }
            return;
        }

        const itemToAppend = this.queue.shift();

        this.sourceBufferPendingUpdate = true;

        let data: Uint8Array;

        if (this.segmentAppendedCount === 0) {
            // First segment. Append the initSegment + data
            data = new Uint8Array(itemToAppend.initSegment.byteLength + itemToAppend.data.byteLength);
            data.set(itemToAppend.initSegment, 0);
            data.set(itemToAppend.data, itemToAppend.initSegment.byteLength);
        } else {
            data = itemToAppend.data;
        }

        this.mediaSource.duration += itemToAppend.duration;
        this.sourceBuffer.appendBuffer(data);

        if (this.options.debug) {
            console.log(LOG_PREFIX + "Appended segment to SourceBuffer. Index: " + this.segmentAppendedCount + ". Duration: " + itemToAppend.duration + " seconds. Size: " + data.byteLength + " bytes.");
        }

        this.segmentAppendedCount++;
    }

    /**
     * Adds a segment to the media source
     * @param duration Duration in seconds
     * @param initSegment Initial MP4 segment with the metadata
     * @param data Data segment
     */
    public addSegment(duration: number, initSegment: Uint8Array, data: Uint8Array) {
        if (this.ended) {
            throw new Error("Called addSegment() after end() was called");
        }

        if (this.queue.length >= this.queueMaxLength) {
            if (this.options.debug) {
                console.log(LOG_PREFIX + "Queue cannot keep up. Discarding one element.");
            }
            this.queue.shift(); // Discard one element
        }

        this.queue.push({
            duration: duration,
            initSegment: initSegment,
            data: data,
        });

        if (this.options.debug) {
            console.log(LOG_PREFIX + "Added segment to queue. Duration: " + duration + " seconds. Size: " + initSegment.byteLength + " bytes initSegment, " + data.byteLength + " bytes data, " + (initSegment.byteLength + data.byteLength) + " bytes total.");
        }

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

            if (this.options.debug) {
                console.log(LOG_PREFIX + "Parsed initSegment. Metadata: \n" + JSON.stringify({
                    video: initData.video,
                    audio: initData.audio,
                }));
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
     * Sets delay options
     * @param delay Desired delay
     * @param maxDelay Max delay
     */
    public setDelayOptions(delay: number, maxDelay: number) {
        this.delay = delay;
        this.maxDelay = maxDelay;
        if (!this.options.maxBufferDuration) {
            this.sourceBufferMaxDuration = this.maxDelay * 2;
        }
        this.onMediaElementUpdate();
    }

    /**
     * Called when the media element updates
     */
    private onMediaElementUpdate() {
        const mediaElement = this.mediaElement;

        if (!mediaElement) {
            return;
        }

        const duration = mediaElement.duration;
        const currentTime = mediaElement.currentTime;

        if (isNaN(duration) || !isFinite(duration) || duration <= 0 || isNaN(currentTime) || !isFinite(currentTime)) {
            // Not yet loaded
            return;
        }

        if (mediaElement.paused) {
            // If paused, no need to check
            return;
        }

        const currentDelay = duration - currentTime;

        if (currentDelay > this.maxDelay) {
            // Seek
            const newCurrentTime = duration - this.delay;
            mediaElement.currentTime = newCurrentTime;

            if (this.options.debug) {
                console.log(LOG_PREFIX + "Max delay reached (" + currentDelay + "). Seeking " + currentTime + " -> " + newCurrentTime);
            }
        }
    }

    /**
     * Attaches to a media element
     * @param mediaElement The media element
     */
    public attachMedia(mediaElement: HTMLMediaElement) {
        this.detachMedia();

        this.mediaElement = mediaElement;
        this.mediaObjectUrl = URL.createObjectURL(this.mediaSource);
        this.mediaElement.src = this.mediaObjectUrl;
        this.mediaElement.addEventListener("durationchange", this.onMediaElementUpdateHandler);
        this.mediaElement.addEventListener("timeupdate", this.onMediaElementUpdateHandler);
    }

    /**
     * Detaches from the current media element
     */
    public detachMedia() {
        if (this.mediaObjectUrl) {
            URL.revokeObjectURL(this.mediaObjectUrl);
            this.mediaObjectUrl = "";
        }

        if (this.mediaElement) {
            this.mediaElement.removeAttribute("src");
            this.mediaElement.removeEventListener("durationchange", this.onMediaElementUpdateHandler);
            this.mediaElement.removeEventListener("timeupdate", this.onMediaElementUpdateHandler);
            this.mediaElement = null;
        }
    }

    /**
     * Releases all resources
     */
    public destroy() {
        this.onError = null;
        this.onEnded = null;
        this.detachMedia();
        this.ended = true;
        this.queue = [];
        this.onUpdated();
    }
}
