FROM golang:1.26.10-alpine@sha256:27f2d3b83cfc5803c8078f8e730af0f7c431bf067afaec8f60a36fb763187cba AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus-update

FROM alpine:3@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

RUN apk update && apk add --no-cache clamav

RUN apk update && \
    apk upgrade \
    busybox \
    libcrypto3 \
    libcurl \
    libssl3 \
    libxml2 \
    musl \
    musl-utils \
    openssl \
    pcre2 \
    zlib && \
    rm -rf /var/cache/apk/*

RUN mkdir -p /tmp/clamav && chown -R clamav:clamav /tmp/clamav

COPY ./freshclam.conf /etc
COPY --from=build-env /go/bin/main /var/task/main


USER clamav
ENTRYPOINT [ "/var/task/main" ]
