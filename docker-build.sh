#!/usr/bin/env bash

VERSION=1.0.13

docker buildx build --platform=linux/amd64,linux/arm64 -t mylxsw/aidea-server:$VERSION . --push

