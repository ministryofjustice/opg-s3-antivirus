#!/usr/bin/env bash

set -e

printf "Install utilities for test\n\n"
dnf update -y
dnf install -y unzip tree

unzip lambda_layer.zip -d /opt

export PATH=/opt/bin:$PATH
export LD_LIBRARY_PATH=/opt/lib:$LD_LIBRARY_PATH

ls -al /opt

cat /opt/etc/clamd.conf

clamd --config-file /opt/etc/clamd.conf --version
clamdscan --config-file /opt/etc/clamd.conf --version
