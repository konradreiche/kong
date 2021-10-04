#!/usr/bin/env bash 
if [ "$(uname)" == "Darwin" ]; then
	go build -o bin/kong cmd/kong.go
	cp bin/kong /usr/local/bin/kong
else
	go install cmd/*
fi
./scripts/reload.sh
