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

## Health check

You can check for the server health by sending an `HTTP GET` request to any other path that is not the websocket path. The server will return a `200 OK` response with the body `OK - HLS Websocket CDN`.

## Configuration

You can configure the server using environment variables.

### Log configuration

| Variable    | Description                                                                       |
| ----------- | --------------------------------------------------------------------------------- |
| `LOG_INFO`  | Can be `YES` or `NO`. Set it to `YES` in order to enable logging `INFO` messages  |
| `LOG_DEBUG` | Can be `YES` or `NO`. Set it to `YES` in order to enable logging `DEBUG` messages |

### Server configuration (HTTP)

| Variable            | Description                                                                                    |
| ------------------- | ---------------------------------------------------------------------------------------------- |
| `HTTP_ENABLED`      | Can be `YES` or `NO`. Set it to `YES` in order to enable the HTTP server (enabled by default). |
| `HTTP_PORT`         | The port number for the HTTP server (80 by default)                                            |
| `HTTP_BIND_ADDRESS` | The bind address for the HTTP server (Leave empty to listen on all network interfaces)         |

### Server configuration (HTTPS, HTTP over TLS)

| Variable                   | Description                                                                                     |
| -------------------------- | ----------------------------------------------------------------------------------------------- |
| `TLS_ENABLED`              | Can be `YES` or `NO`. Set it to `YES` in order to enable the HTTPS server (enabled by default). |
| `TLS_PORT`                 | The port number for the HTTPS server (443 by default)                                           |
| `TLS_BIND_ADDRESS`         | The bind address for the HTTPS server (Leave empty to listen on all network interfaces)         |
| `TLS_CERTIFICATE`          | Path to the X.509 certificate for TLS                                                           |
| `TLS_PRIVATE_KEY`          | Path to the private key for TLS                                                                 |
| `TLS_CHECK_RELOAD_SECONDS` | Number of seconds to check for changes in the certificate or key (for auto renewal)             |

### Websocket protocol configuration

| Variable                  | Description                                                                          |
| ------------------------- | ------------------------------------------------------------------------------------ |
| `WEBSOCKET_PREFIX`        | Path clients must use to connect to the server. By default: `/`.                     |
| `MAX_BINARY_MESSAGE_SIZE` | When handling binary messages, what is the limit for them, in bytes. Default: 50 MB. |
