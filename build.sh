#!/usr/bin/env bash

if [ -f ./detect-drain ]; then
  rm ./detect-drain
fi

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o detect-drain main.go
