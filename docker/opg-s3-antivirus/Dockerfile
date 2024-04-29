FROM golang:1.22.2 AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus

FROM public.ecr.aws/lambda/provided:al2023-x86_64

ENV PATH="${PATH}:/usr/sbin"

RUN dnf update && dnf --best install -y clamav clamd && dnf clean all

RUN dnf update && \
    dnf upgrade \
    libcurl \
    libxml2 \
    openssl && \
    dnf clean all

COPY ./clamd.conf /etc
COPY --from=build-env /go/bin/main /var/task/main

ENTRYPOINT [ "/var/task/main" ]
