FROM golang:1.20.2 AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus

FROM alpine:3

RUN apk update && apk add --no-cache clamav

RUN apk update && \
    apk upgrade \
    libssl1.1 \
    libcrypto1.1 && \
    rm -rf /var/cache/apk/*

COPY ./clamd.conf /etc
COPY --from=build-env /go/bin/main /var/task/main

ENTRYPOINT [ "/var/task/main" ]
