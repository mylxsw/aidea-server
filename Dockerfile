# build stage
FROM golang:1.21 AS builder
ENV GOPROXY=https://goproxy.io,direct
WORKDIR /data
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /data/bin/aidea-server main.go

# final stage
FROM ubuntu:23.10

ENV TZ=Asia/Shanghai

RUN apt-get -y update && DEBIAN_FRONTEND="nointeractive" apt install -y tzdata ca-certificates --no-install-recommends && rm -r /var/lib/apt/lists/*
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

WORKDIR /data
COPY --from=builder /data/bin/aidea-server /usr/local/bin/
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/aidea-server", "--conf", "/etc/aidea.yaml"]