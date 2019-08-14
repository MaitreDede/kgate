#!/usr/bin/env /bin/bash

set -e

GOOS=windows GOARCH=386 go build -o kgate.exe ./main.go