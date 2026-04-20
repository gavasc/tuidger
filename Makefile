VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY    = tuidger
PREFIX   ?= /usr/local
LDFLAGS   = -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build install uninstall clean package-deb package-arch dist

build:
	go build $(LDFLAGS) -o $(BINARY) .

install: build
	install -Dm755 $(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/$(BINARY)

clean:
	rm -f $(BINARY)
	rm -rf dist/

dist:
	mkdir -p dist/

package-deb: dist build
	VERSION=$(VERSION) ARCH=amd64 nfpm package --packager deb --target dist --config packaging/nfpm.yaml

package-arch: dist build
	VERSION=$(VERSION) ARCH=amd64 nfpm package --packager archlinux --target dist --config packaging/nfpm.yaml
