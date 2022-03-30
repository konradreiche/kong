build:
	go install cmd/kong.go && rm -f cmd/cmd

test:
	golangci-lint run
	go test ./...

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

install-linter:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.45.2
