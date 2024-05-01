#!/usr/bin/env bash

set -e

go mod download

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -tags lambda.norpc -o bootstrap ./opg-s3-antivirus
