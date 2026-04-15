FROM golang:1.25-alpine AS build-env

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

RUN dnf upgrade -y \
    libcurl-minimal-8.17.0-1.amzn2023.0.1 \
    openssl-1:3.2.2-1.amzn2023.0.5 && \
    dnf update -y libxml2 --releasever 2023.10.20260302 && \
    dnf update -y openssl-3.5.5-1.amzn2023.0.4 openssl-libs-1:3.5.5-1.amzn2023.0.4 openssl-fips-provider-latest-1:3.5.5-1.amzn2023.0.4 libnghttp2-1.59.0-3.amzn2023.0.2 --releasever 2023.11.20260413 && \
    dnf clean all


COPY ./clamd.conf /opt/etc/clamd.conf
COPY --from=build-env /go/bin/main /var/task/main

ENTRYPOINT [ "/var/task/main" ]
