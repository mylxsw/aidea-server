#!/usr/bin/env bash

VERSION=2.0.0

docker buildx build --platform=linux/amd64 -t mylxsw/aidea-server:$VERSION . --push

