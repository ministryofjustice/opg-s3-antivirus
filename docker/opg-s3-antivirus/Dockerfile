FROM golang:1.26-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus

FROM public.ecr.aws/lambda/provided:al2023@sha256:26136a72871f0d0f9948a98a4568010b3aa210cd7bcb7dd6b51b606fe743b79a

ENV PATH="${PATH}:/usr/sbin"

RUN dnf update -y && \
    dnf install -y clamav-0.103.12-1.amzn2023.0.1 clamd-0.103.12-1.amzn2023.0.1

RUN dnf upgrade -y --releasever=latest && \
    dnf clean all


COPY ./clamd.conf /opt/etc/clamd.conf
COPY --from=build-env /go/bin/main /var/task/main

ENTRYPOINT [ "/var/task/main" ]
