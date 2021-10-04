build:
	go install cmd/*

lint:
	golangci-lint run

install:
	./scripts/install.sh

reload:
	./scripts/reload.sh

status:
	systemctl --user status kong.service

logs:
	journalctl --user -u kong.service -f
