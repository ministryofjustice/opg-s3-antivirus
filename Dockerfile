FROM amazonlinux:2 AS build-clamav

RUN yum update -y
RUN amazon-linux-extras install epel -y
RUN yum install -y cpio yum-utils zip openssl
RUN yum install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm

WORKDIR /tmp

RUN yumdownloader -x \*i686 --archlist=x86_64 clamav clamav-lib clamav-update gnutls json-c libprelude libtasn1 nettle pcre2
RUN rpm2cpio clamav-0*.rpm | cpio -idmv
RUN rpm2cpio clamav-lib*.rpm | cpio -idmv
RUN rpm2cpio clamav-update*.rpm | cpio -idmv
RUN rpm2cpio gnutls*.rpm | cpio -idmv
RUN rpm2cpio json-c*.rpm | cpio -idmv
RUN rpm2cpio libprelude*.rpm | cpio -idmv
RUN rpm2cpio libtasn1*.rpm | cpio -idmv
RUN rpm2cpio nettle*.rpm | cpio -idmv
RUN rpm2cpio pcre*.rpm | cpio -idmv

FROM golang:1.17.6 AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main

FROM lambci/lambda:go1.x

ENV DOCKER_LAMBDA_WATCH=1
ENV DOCKER_LAMBDA_STAY_OPEN=1
ENV AWS_LAMBDA_FUNCTION_HANDLER=main
ENV LD_LIBRARY_PATH=/var/task/lib

COPY --from=build-clamav /tmp/usr/bin/clamscan /var/task/bin/clamscan
COPY --from=build-clamav /tmp/usr/bin/freshclam /var/task/bin/freshclam
COPY --from=build-clamav /tmp/usr/lib64 /var/task/lib

COPY ./freshclam.conf /etc
RUN mkdir -p /tmp/usr/clamav
RUN /var/task/bin/freshclam

COPY --from=build-env /go/bin/main /var/task/main
