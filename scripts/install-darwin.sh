#!/usr/bin/env bash
go build -o bin/kong cmd/kong.go
cp bin/kong /usr/local/bin/kong
./scripts/reload-darwin.sh
