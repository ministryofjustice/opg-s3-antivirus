#!/usr/bin/env bash

set -e

VERSION=${VERSION:-0.103.9-1.amzn2023.0.2.x86_64}

printf "Install utilities for build task\n\n"
dnf update -y
dnf install -y cpio dnf-plugins-core zip tree

printf "Make directory for build\n\n"
mkdir -p /tmp/build
pushd /tmp/build

printf "Download the clamav and clamd package\n\n"
dnf download --downloaddir=/tmp/build clamav-${VERSION} clamd-${VERSION}

printf "Convert clamav and clamd RPM to CPIO stream (-vmid verbose, preserve-modification-time, extract, make-directories)\n\n"
rpm2cpio clamav*.rpm | cpio -vimd
rpm2cpio clamd*.rpm | cpio -vimd

# list what we're moving
tree /tmp/build/usr/
tree /opt/app

printf "move binaries and lib for zipping\n\n"
mkdir -p bin lib etc

cp /tmp/build/usr/bin/clamdscan bin/.
cp /tmp/build/usr/sbin/clamd bin/.
cp -R /tmp/build/usr/lib/* lib/.
cp /app/clamd.conf etc/.

zip -r9 /app/lambda_layer.zip bin
zip -r9 /app/lambda_layer.zip lib
zip -r9 /app/lambda_layer.zip etc
