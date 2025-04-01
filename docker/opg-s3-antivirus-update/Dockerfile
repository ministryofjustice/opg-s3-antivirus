FROM golang:1.24.1 AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus-update

FROM alpine:3

RUN apk update && apk add --no-cache clamav

RUN apk update && \
    apk upgrade \
    busybox \
    libcrypto3 \
    libcurl \
    libssl3 \
    libxml2 \
    openssl && \
    rm -rf /var/cache/apk/*

RUN mkdir -p /tmp/clamav && chown -R clamav:clamav /tmp/clamav

COPY ./freshclam.conf /etc
COPY --from=build-env /go/bin/main /var/task/main

ENTRYPOINT [ "/var/task/main" ]
