SCRIPT = ./scripts/install.sh
RELOAD = ./scripts/reload.sh
UNAME = $(shell uname)
ifeq ($(UNAME), Darwin)
	SCRIPT = ./scripts/install-darwin.sh
endif

build:
	go install cmd/*

lint:
	golangci-lint run

install:
	$(SCRIPT)

reload:
	$(RELOAD)

status:
	systemctl --user status kong.service

logs:
	journalctl --user -u kong.service
