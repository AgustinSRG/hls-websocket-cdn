// Websocket message

"use strict";

// CDN Websocket message
export interface CdnWebsocketMessage {
    // Message type (uppercase)
    type: string;

    // Parameters
    parameters?: Map<string, string>;
}

/**
 * Serializes websocket message
 * @param msg The message
 * @returns The serialized message
 */
export function serializeCdnWebsocketMessage(msg: CdnWebsocketMessage): string {
    let paramStr = "";

    if (msg.parameters && msg.parameters.size > 0) {
        const paramArray: string[] = [];

        msg.parameters.forEach((v, k) => {
            paramArray.push(encodeURIComponent(k) + "=" + encodeURIComponent(v));
        });

        paramStr = paramArray.join("&");
    }

    return msg.type + (paramStr ? (":" + paramStr) : "");
}

/**
 * Parses message coming from the CDN server
 * @param msg The string message
 * @returns The parsed message
 */
export function parseCdnWebsocketMessage(msg: string): CdnWebsocketMessage {
    const msgParts = msg.split(":");

    const msgType = (msgParts[0] || "").toUpperCase();

    if (msgParts.length === 1) {
        return {
            type: msgType,
        };
    }

    const msgParams = new Map<string, string>();

    msgParts.slice(1).join(":").split("&").map(p => {
        if (!p) {
            return null;
        }

        const parts = p.split("=");

        try {
            const key =  decodeURIComponent(parts[0] || "");
            const value = decodeURIComponent(parts.slice(1).join("="));

            msgParams.set(key, value);
        } catch (ex) {
            return null;
        }
    });

    return {
        type: msgType,
        parameters: msgParams,
    };
}
