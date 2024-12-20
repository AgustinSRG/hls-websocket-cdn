# HLS WebSocket CDN - Javascript client

[![npm version](https://badge.fury.io/js/%40asanrom%2Fhls-websocket-cdn.svg)](https://www.npmjs.com/package/@asanrom/hls-websocket-cdn)

This is a JavaScript client for **HLS WebSocket CDN**. You can use it in order to connect to the CDN servers and play the streams from a web browser.

## Installation

If you are using a npm managed project use:

```
npm install @asanrom/hls-websocket-cdn
```

If you are using it in the browser, without any bundler, download the minified file from the [Releases](https://github.com/AgustinSRG/hls-websocket-cdn/tags) section and import it to your html:

```html
<script type="text/javascript" src="/path/to/hls-websocket-cdn.js"></script>
```

The browser library exports all artifacts to the window global: `HlsWebSocketCdn`

## Usage

You need a video element and the connection details (normally given by the backend) to play the stream.

Here is an example:

```html
<script type="text/javascript" src="/path/to/hls-websocket-cdn.js"></script>
<video id="video"></video>
<script>
    const HlsWebSocket = HlsWebSocketCdn.HlsWebSocket;

    var video = document.getElementById('video');
    var cdnUrl = "wss://ws.example.com/";
    var cdnStreamId = "example-stream-id";
    var cdnAuth = "";
    var fallbackHlsUrl = "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8";

    // Check if MSE are supported
    if (HlsWebSocket.isSupported()) {
        // Create instance of HlsWebSocket
        var hls = new HlsWebSocket({
            cdnServerUrl: cdnUrl,
            streamId: cdnStreamId,
            authToken: cdnAuth,
        });

        // Call start() to connect to the server
        // and start pulling the stream
        hls.start();

        // Call attachMedia(videoElement) to attach the stream playback to a video element
        hls.attachMedia(video);
    } else if (video.canPlayType('application/vnd.apple.mpegurl')){
        // If MSE are not supported, but native HLS playback is supported
        // just set the video source to a fallback HLS URL
        // This is the case for IOS devices
        video.src = fallbackHlsUrl;
    }
</script>
```

## Documentation

 - [Typescript documentation](https://agustinsrg.github.io/hls-websocket-cdn/client-js/docs)

## Build instructions

To build the code, you need to install [Node.js](https://nodejs.org/en/). Install the latest stable version to avoid bugs.

Once installed, run the following command to install dependencies:

```sh
npm install
```

Run the following command to build the library:

```sh
npm run build
```
