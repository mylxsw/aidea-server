#!/usr/bin/env bash

VERSION=1.0.9

docker buildx build --platform=linux/amd64,linux/arm64,linux/arm/v7 -t mylxsw/aidea-server:$VERSION . --push

