# HLS Websocket CDN - Tester

This is a simple program to manually test the CDN, allowing to publish a video in a loop, and also to serve a client to play the stream from the browser.

## Compilation

To install dependencies, type:

```sh
go get .
```

To build the program, type:

```sh
go build .
```

This will generate a binary with the name `tester`, or `tester.exe` if you are in Windows.

## Usage

Turn the binary, with the following command usage:

```
Usage: tester <COMMAND> [OPTIONS]
Commands:
  publish   Publishes a video on loop as HLS to the CDN server
  spectate  Server a browser client to spectate as HLS stream 
Options:
  -s, --secret <secret>   Secret to sign authentication tokens
  -u, --url <url>         URL of the CDN server
  -i, --id <id>           Stream ID
  -v, --video <path>      Path to the video file to publish   
  -d, --debug             Enables debug and trace messages    
  --ffmpeg <path>          Sets a custom location to the FFmpeg binary
  --js-bundle <path>       Sets a custom location to the client JavaScript bundle
```

Example command to publish a video file in a loop:

```sh
./tester publish -u ws://127.0.0.1/ -i test -s secret -v /path/to/video.mp4 --debug
```

Example command to serve a client to spectate:

```sh
./tester spectate -u ws://127.0.0.1/ -i test -s secret --debug
```

# Run a test network with Docker Compose

You can run a test network, with a Redis publish registry + 4 nodes using the `docker-compose.yml` file provided in this folder.

The nodes can be accessed from the following URLs:

 - Server 1: `ws://127.0.0.1:8081/`
 - Server 2: `ws://127.0.0.1:8082/`
 - Relay 1 (Connected to Server 1): `ws://127.0.0.1:8083/`
 - Relay 2 (Connected to Server 2): `ws://127.0.0.1:8084/`

The secret for all of the servers, both for pulling and pushing is `demosecret`. You can change it by setting the environment variable `SHARED_SECRET`.

To start the test network, run:

```
docker compose up -d
```

To stop the network, run:

```
docker compose down
```
