FROM golang:1.26.10-alpine@sha256:27f2d3b83cfc5803c8078f8e730af0f7c431bf067afaec8f60a36fb763187cba AS build-env

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/main ./cmd/opg-s3-antivirus

FROM public.ecr.aws/lambda/provided:al2023@sha256:11c68dffb9aedf1a2198505278c1779f921edf1a821d2aa6434ad9e7da260caf

ENV PATH="${PATH}:/usr/sbin"

RUN dnf update -y && \
    dnf install -y clamav-0.103.12-1.amzn2023.0.1 clamd-0.103.12-1.amzn2023.0.1

RUN dnf upgrade -y --releasever=latest && \
    dnf clean all

RUN rm /usr/local/bin/aws-lambda-rie

COPY ./clamd.conf /opt/etc/clamd.conf
COPY --from=build-env /go/bin/main /var/task/main

USER clamsscan
ENTRYPOINT [ "/var/task/main" ]
