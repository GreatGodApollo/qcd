#!/bin/bash
echo "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o qcd-linux main.go
if [ $? -eq 0 ]; then
    echo "Build successful: qcd-linux"
else
    echo "Build failed"
    exit 1
fi
