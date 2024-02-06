# syntax=docker/dockerfile:1

##
## Build
##
FROM golang:1.20-alpine AS BUILD

WORKDIR /build

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./

RUN go build -o kube-image-deployer

##
## Deploy
##
FROM alpine

WORKDIR /app

COPY --from=BUILD /build/kube-image-deployer kube-image-deployer

ENTRYPOINT ["/app/kube-image-deployer"]
