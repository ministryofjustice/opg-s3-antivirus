FROM golang:1.26-alpine AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus

FROM public.ecr.aws/lambda/provided:al2

ENV PATH="${PATH}:/usr/sbin"

RUN yum update -y && yum install -y clamav-0.103.12-1.amzn2.0.1 clamd-0.103.12-1.amzn2.0.1 && yum clean all

RUN yum update -y && \
    yum upgrade -y \
    libcurl-8.3.0-1.amzn2.0.6 \
    libxml2-2.9.1-6.amzn2.5.13 \
    openssl-1:1.0.2k-24.amzn2.0.12 && \
    yum clean all


COPY ./clamd.conf /opt/etc/clamd.conf
COPY --from=build-env /go/bin/main /var/task/main

ENTRYPOINT [ "/var/task/main" ]
