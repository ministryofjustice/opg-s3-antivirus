FROM golang:1.27-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS build-env

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

USER clamsscan
ENTRYPOINT [ "/var/task/main" ]
