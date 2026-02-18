BINARY   := auto-shutdown
MODULE   := auto-shutdown
PREFIX   := /usr/local
BINDIR   := $(PREFIX)/bin
CONFDIR  := /etc
UNITDIR  := /etc/systemd/system

GO       ?= go
GOFLAGS  ?=
LDFLAGS  ?= -s -w

# ── default ──────────────────────────────────────────────────────────

.PHONY: all
all: build

# ── build ────────────────────────────────────────────────────────────

.PHONY: build
build:
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) .

# ── code quality ─────────────────────────────────────────────────────

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: lint
lint: fmt vet

.PHONY: test
test:
	$(GO) test -v ./...

# ── install / uninstall (run with sudo) ──────────────────────────────

.PHONY: install
install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	@if [ ! -f $(DESTDIR)$(CONFDIR)/$(BINARY).conf ]; then \
		install -m 644 $(BINARY).conf $(DESTDIR)$(CONFDIR)/$(BINARY).conf; \
		echo "Installed default config to $(CONFDIR)/$(BINARY).conf"; \
	else \
		echo "$(CONFDIR)/$(BINARY).conf already exists – skipping"; \
	fi
	install -m 644 $(BINARY).service $(DESTDIR)$(UNITDIR)/$(BINARY).service
	systemctl daemon-reload
	systemctl enable --now $(BINARY).service
	@echo "Done. Run 'systemctl status $(BINARY)' to verify."

.PHONY: uninstall
uninstall:
	-systemctl disable --now $(BINARY).service 2>/dev/null || true
	rm -f $(DESTDIR)$(UNITDIR)/$(BINARY).service
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)
	systemctl daemon-reload
	@echo "Uninstalled. Config file $(CONFDIR)/$(BINARY).conf was left in place."
	@echo "Remove it manually if no longer needed."

# ── clean ────────────────────────────────────────────────────────────

.PHONY: clean
clean:
	rm -f $(BINARY)

# ── help ─────────────────────────────────────────────────────────────

.PHONY: help
help:
	@echo "Targets:"
	@echo "  make              Build the binary (default)"
	@echo "  make build        Build the binary"
	@echo "  make fmt          Format source code"
	@echo "  make vet          Run go vet"
	@echo "  make lint         Run fmt + vet"
	@echo "  make test         Run tests"
	@echo "  make install      Build, install binary/config/service (requires sudo)"
	@echo "  make uninstall    Stop service and remove binary/unit (requires sudo)"
	@echo "  make clean        Remove build artifacts"
	@echo "  make help         Show this help"
