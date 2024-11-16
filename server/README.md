# HLS Websocket CDN - Server

This the main backend component for **HLS Websocket CDN**, implemented in golang.

## Compilation

In order to install dependencies, type:

```sh
go get .
```

To compile the code type:

```sh
go build .
```

The build command will create a binary in the current directory, called `server`, or `server.exe` if you are using Windows.

## Docker image

You can find the docker image for this project available in Docker Hub: https://hub.docker.com/r/asanrom/hls-websocket-cdn

To pull it type:

```sh
docker pull asanrom/hls-websocket-cdn
```

Example compose file:

```yaml
version: "3.7"

services:
  cdn_server:
    image: asanrom/hls-websocket-cdn
    ports:
      - "80:80"
      #- '443:443'
    environment:
      # Configure it using env vars:
      - LOG_INFO=YES
      - LOG_DEBUG=NO
```

## Configuration

You can configure the server using environment variables.

### Log configuration

| Variable    | Description                                                                       |
| ----------- | --------------------------------------------------------------------------------- |
| `LOG_INFO`  | Can be `YES` or `NO`. Set it to `YES` in order to enable logging `INFO` messages  |
| `LOG_DEBUG` | Can be `YES` or `NO`. Set it to `YES` in order to enable logging `DEBUG` messages |
