FROM amazonlinux:2 AS build-clamav

RUN yum update -y
RUN amazon-linux-extras install epel -y
RUN yum install clamav clamd -y

COPY ./freshclam.conf /etc
RUN freshclam

FROM golang:1.17.7 AS build-env

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

COPY ./freshclam.conf /etc
COPY --from=build-clamav /usr/bin/clamscan /var/task/bin/clamscan
COPY --from=build-clamav /usr/lib64/libclam* /var/task/lib/
COPY --from=build-clamav /usr/lib64/libgnutls* /var/task/lib/
COPY --from=build-clamav /usr/lib64/libhogweed* /var/task/lib/
COPY --from=build-clamav /usr/lib64/libjson* /var/task/lib/
COPY --from=build-clamav /usr/lib64/libnettle* /var/task/lib/
COPY --from=build-clamav /usr/lib64/libpcre* /var/task/lib/
COPY --from=build-clamav /usr/lib64/libprelude* /var/task/lib/
COPY --from=build-clamav /usr/lib64/libtasn1* /var/task/lib/
COPY --from=build-clamav /var/lib/clamav /var/lib/clamav

COPY --from=build-env /go/bin/main /var/task/main
