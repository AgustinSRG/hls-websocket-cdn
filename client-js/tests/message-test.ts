// Test

"use strict";

import { CdnWebSocketMessage, serializeCdnWebSocketMessage, parseCdnWebSocketMessage } from "../src/message";

import assert from "assert";

function isEmptyMap(m: Map<string, string> | undefined): boolean {
    if (!m) {
        return true;
    }

    return m.size === 0;
}

function testMessage(msg: CdnWebSocketMessage) {
    const msgStr = serializeCdnWebSocketMessage(msg);
    const parsedMessage = parseCdnWebSocketMessage(msgStr);

    assert.equal(parsedMessage.type, msg.type);

    if ((isEmptyMap(parsedMessage.parameters) || isEmptyMap(msg.parameters)) && (!isEmptyMap(parsedMessage.parameters) || !isEmptyMap(msg.parameters))) {
        throw new Error("Parsed message parameters does not match with expected parameters. \n" + msgStr + "\n" + serializeCdnWebSocketMessage(parsedMessage))
    }

    if (isEmptyMap(parsedMessage.parameters) && isEmptyMap(msg.parameters)) {
        return true;
    }

    for (let [key, value] of msg.parameters?.entries() || []) {
        assert.equal(parsedMessage.parameters?.get(key), value);
    }

    for (let [key, value] of parsedMessage.parameters?.entries() || []) {
        assert.equal(msg.parameters?.get(key), value);
    }
}

describe("Message serialization / deserialization test", () => {
    it('Heartbeat message', async () => {
        testMessage({
            type: "H"
        });
    });

    it('Error message', async () => {
        testMessage({
            type: "E",
            parameters: new Map([
                ["code", "ERROR_CODE"],
                ["message", "Example error message"],
            ])
        });
    });

    it('PUSH message', async () => {
        testMessage({
            type: "PUSH",
            parameters: new Map([
                ["stream", "stream_id"],
                ["auth", "example-auth-token"],
            ])
        });
    });

    it('PULL message', async () => {
        testMessage({
            type: "PULL",
            parameters: new Map([
                ["stream", "stream_id"],
                ["auth", "example-auth-token"],
                ["only_source", "false"],
                ["max_initial_fragments", "10"]
            ])
        });
    });

    it('OK message', async () => {
        testMessage({
            type: "OK"
        });
    });

    it('Fragment message', async () => {
        testMessage({
            type: "F",
            parameters: new Map([
                ["duration", "1.5"],
            ])
        });
    });

    it('CLOSE message', async () => {
        testMessage({
            type: "CLOSE"
        });
    });
});
