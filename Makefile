.PHONY: build test smoke install clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/diffplay .

test:
	go test -race ./...
	go vet ./...
	python3 scripts/test_install.py
	python3 scripts/test_quickinstall.py

smoke: build
	@scratch=$${TMPDIR:-$$HOME/.cache}; mkdir -p "$$scratch"; \
	state=$$(mktemp -d "$$scratch/diffplay-smoke.XXXXXX"); \
	trap 'rm -rf "$$state"' EXIT HUP INT TERM; \
	export HOME="$$state" XDG_CONFIG_HOME="$$state/config"; \
	for name in unified terminal list git hierarchy workspace activity selection; do \
		python3 -u "scripts/$${name}_smoke.py" "$(CURDIR)/bin/diffplay" || exit $$?; \
	done

install:
	./install.sh

clean:
	rm -f bin/diffplay
