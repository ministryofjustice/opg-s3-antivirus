#!/usr/bin/env bash

set -e

VERSION=${VERSION:-0.103.9-1.amzn2023.0.2.x86_64}

echo "!!!>>>>>Install utilities for build task"
dnf update -y
dnf install -y cpio dnf-plugins-core zip tree

echo "!!!>>>>>Make directory for build"
mkdir -p /tmp/build
pushd /tmp/build

# Download the clamav and clamd package
echo "!!!>>>>>Download the clamav and clamd package"
dnf download --downloaddir=/tmp/build clamav-${VERSION} clamd-${VERSION}

# Convert clamav and clamd RPM to CPIO stream (-vmid verbose, preserve-modification-time, extract, make-directories)
echo "!!!>>>>>Convert clamav and clamd RPM to CPIO stream (-vmid verbose, preserve-modification-time, extract, make-directories)"
rpm2cpio clamav*.rpm | cpio -vimd
rpm2cpio clamd*.rpm | cpio -vimd

# list what we're moving
tree /tmp/build/usr/
tree /opt/app

# move binaries and lib for zipping
echo "!!!>>>>>move binaries and lib for zipping"
mkdir -p bin lib etc

cp /tmp/build/usr/bin/clamdscan bin/.
cp /tmp/build/usr/sbin/clamd bin/.
cp -R /tmp/build/usr/lib/* lib/.
cp /app/clamd.conf etc/.

zip -r9 /app/lambda_layer.zip bin
zip -r9 /app/lambda_layer.zip lib
zip -r9 /app/lambda_layer.zip etc
