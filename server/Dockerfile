## Dockerfile ###

FROM golang:alpine

WORKDIR /root

ADD ./ /root/server

WORKDIR /root/server

# Compile

RUN go get .

RUN go build .

# Prepare runner

FROM alpine as runner

# Add gcompat

RUN apk add gcompat

# Copy binaries

COPY --from=0 /root/server/server /bin/server

# Expose ports

EXPOSE 80
EXPOSE 443

# Entrypoint

ENTRYPOINT ["/bin/server"]
