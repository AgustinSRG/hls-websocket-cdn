# HLS Websocket CDN - Authentication

The clients must authenticate in order to be able to send or receive HLS fragments.

In order to authenticate, the client must send an authentication token as a parameter of the action message (`PUSH` or `PULL`, check the [websocket protocol documentation](./websocket-protocol.md)).

The token is a **JSON Web Token (JWT)**, signed with a secret shared by the nodes, with the algorithm `HMAC_256`.

The JWT must have the following fields:

 - Subject (`sub`) must be `PUSH` or `PULL`, matching the action message type, followed by a colon (`:`) and the ID of the target stream. Example: `PULL:{ROOM}/{STREAM_ID}/{WIDTH}x{HEIGHT}-{FPS}~{BITRATE}`

The CDN nodes share 2 secrets:

 - `PULL_SECRET` - Secret to sign and validate tokens in order to receive HLS streams from the CDN.
 - `PUSH_SECRET` - Secret to sign and validate tokens in order to push HLS streams to the CDN.