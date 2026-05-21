FROM golang:1.26-alpine AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus

FROM public.ecr.aws/lambda/provided:al2023

ENV PATH="${PATH}:/usr/sbin"

RUN dnf update -y && \
    dnf install -y clamav-0.103.12-1.amzn2023.0.1 clamd-0.103.12-1.amzn2023.0.1

RUN dnf upgrade -y --releasever=latest && \
    dnf clean all


COPY ./clamd.conf /opt/etc/clamd.conf
COPY --from=build-env /go/bin/main /var/task/main

ENTRYPOINT [ "/var/task/main" ]
