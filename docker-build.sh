#!/usr/bin/env bash

VERSION=1.0.4
VERSION_DATE=202309131051

docker build -t mylxsw/aidea-server:$VERSION .
docker tag mylxsw/aidea-server:$VERSION mylxsw/aidea-server:$VERSION_DATE
docker tag mylxsw/aidea-server:$VERSION mylxsw/aidea-server:latest

docker push mylxsw/aidea-server:$VERSION
docker push mylxsw/aidea-server:$VERSION_DATE
docker push mylxsw/aidea-server:latest

