# HLS WebSocket CDN - Server

This the main backend component for **HLS WebSocket CDN**, implemented in golang.

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

| Variable       | Description                                                                                                        |
| -------------- | ------------------------------------------------------------------------------------------------------------------ |
| `LOG_ERROR`    | Can be `YES` or `NO`. Default: `YES`. Set it to `YES` in order to enable logging `ERROR` messages                  |
| `LOG_WARNING`  | Can be `YES` or `NO`. Default: `YES`. Set it to `YES` in order to enable logging `WARNING` messages                |
| `LOG_INFO`     | Can be `YES` or `NO`. Default: `YES`. Set it to `YES` in order to enable logging `INFO` messages                   |
| `LOG_REQUESTS` | Can be `YES` or `NO`. Default: `YES`. Set it to `YES` in order to enable logging `INFO` messages for HTTP requests |
| `LOG_DEBUG`    | Can be `YES` or `NO`. Default: `NO`. Set it to `YES` in order to enable logging `DEBUG` messages                   |
| `LOG_TRACE`    | Can be `YES` or `NO`. Default: `NO`. Set it to `YES` in order to enable logging `TRACE` messages                   |

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

| Variable                  | Description                                                                                                                                          |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `EXTERNAL_WEBSOCKET_URL`  | External websocket URL of the server, for other servers to connect with it. If empty, it will be automatically detected from the network interfaces. |
| `WEBSOCKET_PREFIX`        | Path clients must use to connect to the server. By default: `/`.                                                                                     |
| `MAX_BINARY_MESSAGE_SIZE` | When handling binary messages, what is the limit for them, in bytes. Default: 50 MB.                                                                 |

### Publish registry (Redis)

| Variable                           | Description                                                                          |
| ---------------------------------- | ------------------------------------------------------------------------------------ |
| `PUB_REG_REDIS_ENABLED`            | Can be `YES` or `NO`. Set it to `YES` in order to enable the redis publish registry. |
| `PUB_REG_REDIS_HOST`               | Redis host. Default: `127.0.0.1`                                                     |
| `PUB_REG_REDIS_PORT`               | Redis port. Default: `6379`                                                          |
| `PUB_REG_REDIS_PASSWORD`           | Password to authenticate to the Redis server.                                        |
| `PUB_REG_REDIS_USE_TLS`            | Can be `YES` or `NO`. Set it to `YES` in order to use TLS to connect to Redis.       |
| `PUB_REG_REFRESH_INTERVAL_SECONDS` | Number of seconds to refresh publish registry entries. Default `60` seconds.         |

### Relay

| Variable             | Description                                                                           |
| -------------------- | ------------------------------------------------------------------------------------- |
| `RELAY_FROM_ENABLED` | Can be `YES` or `NO`. Set it to `YES` to enable relaying streams from another server. |
| `RELAY_FROM_URL`     | Websocket URL of another server to relay HLS streams from.                            |

### Authentication

| Variable       | Description                                                                    |
| -------------- | ------------------------------------------------------------------------------ |
| `PULL_SECRET`  | Secret to sign and validate the authentication tokens for pulling the streams. |
| `PUSH_SECRET`  | Secret to sign and validate the authentication tokens for pushing the streams. |
| `PUSH_ALLOWED` | Can be `YES` or `NO`. Set it to `YES` to allow pushing streams to the server.  |

## Other options

| Variable                     | Description                                                                           |
| ---------------------------- | ------------------------------------------------------------------------------------- |
| `FRAGMENT_BUFFER_MAX_LENGTH` | Max number of fragments to keep in the buffer for new pull connections. Default: `10` |

## Health check

You can check for the server health by sending an `HTTP GET` request to any other path that is not the websocket path. The server will return a `200 OK` response with the body `OK - HLS Websocket CDN`.
