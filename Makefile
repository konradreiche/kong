build:
	go install cmd/kong.go && rm -f cmd/cmd

test:
	go test ./...

lint: install-tools
	golangci-lint run

install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.49.0

install:
	./scripts/install.sh

reload:
	./scripts/reload.sh

stop:
	./scripts/stop.sh

status:
	systemctl --user status kong.service

logs:
	journalctl --user -u kong.service -f
