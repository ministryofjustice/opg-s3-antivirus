#!/usr/bin/env bash

set -e

printf "Install utilities for test\n\n"
dnf update -y
dnf install -y unzip

unzip lambda_layer.zip -d /usr

# export PATH=/app/bin:$PATH

# tree /usr

clamd --version
clamdscan --version
