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
dnf download --downloaddir=/tmp/build --arch x86_64  --resolve clamav-"${VERSION}" clamd-"${VERSION}" systemd-libs

printf "Convert clamav and clamd RPM to CPIO stream (-vmid verbose, preserve-modification-time, extract, make-directories)\n\n"
rpm2cpio clamav*.rpm | cpio -vimd
rpm2cpio clamd*.rpm | cpio -vimd
rpm2cpio clamav-lib-*.rpm | cpio -vimd

# printf "!!!>>>>>Download other package dependencies\n\n"
rpm2cpio systemd-libs-*.rpm | cpio -vimd
# rpm2cpio libargon2-*.rpm | cpio -vimd
# rpm2cpio gzip-*.rpm     | cpio -vimd
# rpm2cpio pam-*.rpm     | cpio -vimd
# rpm2cpio xkeyboard-config-*.rpm | cpio -vimd
# rpm2cpio shadow-utils-*.rpm | cpio -vimd
# rpm2cpio libpwquality-*.rpm | cpio -vimd
# rpm2cpio diffutils-*.rpm | cpio -vimd
# rpm2cpio libfdisk-*.rpm | cpio -vimd
# rpm2cpio qrencode-libs-*.rpm | cpio -vimd
# rpm2cpio systemd-*.rpm | cpio -vimd
# rpm2cpio cracklib-*.rpm | cpio -vimd
# rpm2cpio util-linux-*.rpm | cpio -vimd
# rpm2cpio dbus-*.rpm | cpio -vimd
# rpm2cpio util-linux-core-*.rpm | cpio -vimd
# rpm2cpio dbus-broker-*.rpm | cpio -vimd
# rpm2cpio dbus-common-*.rpm | cpio -vimd
# rpm2cpio systemd-networkd-*.rpm | cpio -vimd
# rpm2cpio kmod-libs-*.rpm | cpio -vimd
# rpm2cpio libdb-*.rpm | cpio -vimd
# rpm2cpio libtool-ltdl-*.rpm | cpio -vimd
# rpm2cpio systemd-pam-*.rpm | cpio -vimd
# rpm2cpio systemd-resolved-*.rpm | cpio -vimd
# rpm2cpio libxkbcommon-*.rpm | cpio -vimd
# rpm2cpio cryptsetup-libs-*.rpm | cpio -vimd
# rpm2cpio device-mapper-*.rpm | cpio -vimd
# rpm2cpio libseccomp-*.rpm | cpio -vimd
# rpm2cpio libsemanage-*.rpm | cpio -vimd
# rpm2cpio libutempter-*.rpm | cpio -vimd
# rpm2cpio libeconf-*.rpm | cpio -vimd

# list what we're moving
# tree /tmp/build/usr/
# tree /opt/app

printf "move binaries and lib for zipping\n\n"
mkdir -p bin lib etc lib64

cp /tmp/build/usr/bin/clamdscan bin/.
cp /tmp/build/usr/sbin/clamd bin/.
cp -R /tmp/build/usr/lib/* lib/.
cp -R /tmp/build/usr/lib64/* lib64/.
cp /app/clamd.conf etc/.

zip -r9 /app/lambda_layer.zip bin
zip -r9 /app/lambda_layer.zip lib
zip -r9 /app/lambda_layer.zip lib64
zip -r9 /app/lambda_layer.zip etc
