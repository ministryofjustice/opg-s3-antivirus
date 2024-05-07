#!/usr/bin/env bash

set -e

printf "Install utilities for test\n\n"
dnf update -y
dnf install -y unzip tree

unzip lambda_layer.zip

tree ./

bin/clamdscan --version
