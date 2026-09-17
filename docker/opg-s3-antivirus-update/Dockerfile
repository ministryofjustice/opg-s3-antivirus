FROM golang:1.27-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus-update

FROM alpine:3@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

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
