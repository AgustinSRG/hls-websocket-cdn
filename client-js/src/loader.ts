// HLS loader

"use strict";

import { type HlsConfig, type Loader, type LoaderCallbacks, type LoaderConfiguration, type LoaderContext, type LoaderStats } from "hls.js";
import { HlsWebsocketCdnClient } from "./client";
import { type HlsWebsocketCdnClientOptions } from "./options";

function getEmptyLoadStats(): LoaderStats {
    return {
        aborted: false,
        loaded: 0,
        retry: 0,
        total: 0,
        chunkCount: 0,
        bwEstimate: 0,
        loading: { start: 0, first: 0, end: 0 },
        parsing: { start: 0, end: 0 },
        buffering: { start: 0, first: 0, end: 0 },
    };
}

export function getLoaderClass(client: HlsWebsocketCdnClient, options: HlsWebsocketCdnClientOptions) {
    return class HlsWebsocketCdnCustomLoader implements Loader<LoaderContext> {
        public config: HlsConfig;
        public client: HlsWebsocketCdnClient;
        public options: HlsWebsocketCdnClientOptions;

        public context: LoaderContext | null;
        public stats: LoaderStats;

        public listenerId: number;

        public callbacks: LoaderCallbacks<LoaderContext> | null;

        constructor(config: HlsConfig) {
            this.client = client;
            this.config = config;
            this.options = options;
            this.context = null;
            this.stats = getEmptyLoadStats();
            this.listenerId = -1;
            this.callbacks = null;
        }

        destroy(): void {
            if (this.listenerId >= 0) {
                this.client.removePlaylistListener(this.listenerId);
                this.listenerId = -1;
            }
        }

        abort(): void {
            if (this.listenerId >= 0) {
                this.client.removePlaylistListener(this.listenerId);
                this.listenerId = -1;
            }

            if (this.callbacks && this.callbacks.onAbort) {
                this.callbacks.onAbort(this.stats, this.context, "");
            }
        }

        load(context: LoaderContext, config: LoaderConfiguration, callbacks: LoaderCallbacks<LoaderContext>): void {
            this.context = context;

            const url = this.context.url;

            const file = url.split("/").pop();
            const fileExtension = file.split(".").pop();

            if (this.options.debug) {
                console.log(`[HLS-WS DEBUG] Load URL: ${url}`);
            }

            switch (fileExtension) {
                case "m3u8":
                    if (this.options.debug) {
                        console.log(`[HLS-WS DEBUG] Loading playlist`);
                    }
                    this.listenerId = this.client.getPlaylist((err, playlist) => {
                        if (err) {
                            if (this.options.debug) {
                                console.log(`[HLS-WS DEBUG] Error loading playlist: ${err}`);
                            }
                            switch (err) {
                                case "error-auth":
                                    callbacks.onError({
                                        code: 403,
                                        text: "Forbidden"
                                    }, context, '', this.stats);
                                    break;
                                default:
                                    callbacks.onError({
                                        code: 404,
                                        text: "Not Found"
                                    }, context, '', this.stats);
                            }
                        } else {
                            if (this.options.debug) {
                                console.log(`[HLS-WS DEBUG] Loaded playlist:\n${playlist}`);
                            }
                            callbacks.onSuccess({
                                url: url,
                                data: playlist,
                            }, this.stats, context, '');
                        }
                    });
                    break;
                case "ts":
                    {
                        const fragmentIndex = parseInt(file.split(".")[0], 10);
                        const fragment = this.client.getFragment(fragmentIndex);

                        if (fragment) {
                            if (this.options.debug) {
                                console.log(`[HLS-WS DEBUG] Loaded fragment ${fragmentIndex}`);
                            }
                            callbacks.onSuccess({
                                url: url,
                                data: fragment,
                            }, this.stats, context, '');
                        } else {
                            if (this.options.debug) {
                                console.log(`[HLS-WS DEBUG] Could not load fragment ${fragmentIndex}`);
                            }
                            callbacks.onError({
                                code: 404,
                                text: "Not Found"
                            }, context, '', this.stats);
                        }
                    }
                    break;
                default:
                    callbacks.onError({
                        code: 404,
                        text: "Not Found"
                    }, context, '', this.stats);
            }
        }

        // Unused methods

        getCacheAge(): number | null {
            return null;
        }

        getResponseHeader(name: string): string | null {
            return null;
        }
    };
}
