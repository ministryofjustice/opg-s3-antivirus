FROM golang:1.26-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus-update

FROM alpine:3@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

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
