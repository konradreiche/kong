build:
	go install cmd/*

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
