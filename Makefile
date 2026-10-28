.PHONY: build test install clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/diffplay .

test:
	go test -race ./...
	go vet ./...
	python3 scripts/test_install.py
	python3 scripts/test_quickinstall.py

install:
	./install.sh

clean:
	rm -f bin/diffplay
