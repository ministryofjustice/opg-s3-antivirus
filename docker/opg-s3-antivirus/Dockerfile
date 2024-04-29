FROM golang:1.22.2 AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus

FROM public.ecr.aws/lambda/provided:al2

ENV PATH="${PATH}:/usr/sbin"

RUN yum update -y && yum install -y clamav clamd && yum clean all

RUN yum update -y && \
    yum upgrade -y \
    libcurl \
    libxml2 \
    openssl && \
    yum clean all


COPY ./clamd.conf /etc
COPY --from=build-env /go/bin/main /var/task/main

ENTRYPOINT [ "/var/task/main" ]
