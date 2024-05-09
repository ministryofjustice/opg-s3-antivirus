#!/usr/bin/env bash

set -e

printf "Install utilities for test\n\n"
dnf update -y
dnf install -y unzip tree

unzip lambda_layer.zip -d /usr

cat /usr/etc/clamd.conf

clamd --config-file /usr/etc/clamd.conf --version
clamdscan --config-file /usr/etc/clamd.conf --version
