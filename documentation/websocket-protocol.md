# HLS Websocket CDN - Websocket Protocol

This document is a specification of the protocol that must be followed in order to interact with the hls-websocket-cdn nodes.

The protocol works on top of the [Websocket Protocol](https://datatracker.ietf.org/doc/html/rfc6455). The clients, or other nodes, will connect to the node via websocket, at the root path. Examples:

```
ws://example-host/
wss://example-host/
```

The default port for TLS Websocket (`wss`) is `443`. The default port for insecure websocket (`ws`) is `80`. The ports can be changed by the node configuration. The client needs to know them in order to connect to the nodes.

## Message format

The Websocket protocol allow for 2 types of messages: Text messages and binary messages.

The text messages must follow the following structure: The message type (which can only contain letters or numbers), followed by a colon character (`:`) and, optionally, followed by a [query string](https://en.wikipedia.org/wiki/Query_string) with the message parameters. Example:

```
MSG_TYPE:param=value&param2=value
```

Note: The message type is **case insensitive**.

The binary messages are used in order to send video fragments. They must be preceded by a text message containing their metadata.

## Message types

The following are a complete list of all the message types that are available for the protocol.

### Heartbeat message

The heartbeat message type is `H`, with no parameters:

```
H
```

The heartbeat messages are used in order to ensure both, client and server, are alive. In order to do that, both parties will send heartbeat messages periodically to each other.

The parties must send a heartbeat messages if they did not send any messages for more than **30 seconds**.

If no messages are received for more that **1 minute**, the connection may be considered dead, and will be closed.

### Error message

The error message type is `E`, with the following parameters:

 - `code` - Error code (string)
 - `message` - Error message (string)

```
E:code=ERROR_CODE&message=ExampleErrorMessage
```

### Fragment message

The fragment message type is `F`, with the following parameters:

 - `duration` - Fragment duration in seconds (floating point number)

```
F:duration=1.000000
```

After a fragment message, it is expected to be received a **binary message** with the fragment itself. The fragments must be MPEG-2 video files (`.ts`).

### Pull message

The pull message type is `PULL`, with the following parameters:

 - `stream` - Identifier of the stream. Can be any string, with a max length of 255 characters. Usually has the following structure `{ROOM}/{STREAM_ID}/{WIDTH}x{HEIGHT}-{FPS}~{BITRATE}`
 - `auth` - Authentication token. See the [authentication token specification](./authentication.md).
 - `only_source` - Optional. Set it to `true` in order to ensure the node does not relay the stream pull to other node.

```
PULL:stream=stream-id&auth=auth-token
```

### Push message

The push message type is `PUSH`, with the following parameters:

 - `stream` - Identifier of the stream. Can be any string, with a max length of 255 characters. Usually has the following structure `{ROOM}/{STREAM_ID}/{WIDTH}x{HEIGHT}-{FPS}~{BITRATE}`
 - `auth` - Authentication token. See the [authentication token specification](./authentication.md).

```
PUSH:stream=stream-id&auth=auth-token
```

## OK message

The OK message type is `OK`, with no parameters:

```
OK
```

This message is sent in order to indicate the `PUSH` or `PULL` message were accepted, and the fragment exchange may start.

## Close message

The close message type is `CLOSE`, with no parameters:

```
CLOSE
```

This message is send if the HLS stream ended, and there are no more fragments.

## Protocol

The following sections specify the message order for the protocol, either for pushing or pulling HLS streams.

### Push protocol

Protocol used when a client (publisher, usually an automated encoding process) wants to push an HLS stream.

Protocol timeline:

 1. The client connects to the server.
 2. The client sends a [Push message](#push-message), containing the ID of the stream to publish, and an authentication token.
 3. The server will send an [OK message](#ok-message) after validating the authentication token and setting it all up.
 4. The client will send [Fragment messages](#fragment-message) for each video fragment of the stream.
 5. When the stream ends, and the client sends its last fragment, the client must send a [Close message](#close-message). After sending this last message, the connection must be closed.


Error cases:

 - If the client does not send a [Push message](#push-message) as its first message, or 30 seconds pass without a first message, the server will send an [Error message](#error-message) and close the connection.
 - If the authentication token or the stream ID set in the [Push message](#push-message) are not valid, the server will send an [Error message](#error-message) and close the connection.
 - If the client does not follow the fragment exchange protocol and send binary messages without their corresponding [Fragment message](#fragment-message), or any other protocol violation, the server will send an [Error message](#error-message) and close the connection.

### Pull protocol

Protocol used when a client (spectator) wants to receive an HLS stream.

Protocol timeline:

 1. The client connects to the server.
 2. The client sends a [Pull message](#pull-message), containing the ID of the stream to receive, and an authentication token.
 3. The server will send an [OK message](#ok-message) after validating the authentication token and setting it all up.
 4. The server will send [Fragment messages](#fragment-message) for each video fragment of the stream.
 5. When the stream ends, and the server sends its last fragment, the server must send a [Close message](#close-message). After sending this last message, the connection must be closed.

Error cases:

 - If the client does not send a [Pull message](#pull-message) as its first message, or 30 seconds pass without a first message, the server will send an [Error message](#error-message) and close the connection.
 - If the authentication token or the stream ID set in the [Pull message](#pull-message) are not valid, the server will send an [Error message](#error-message) and close the connection.
 - For any other protocol violation, the server will send an [Error message](#error-message) and close the connection.
