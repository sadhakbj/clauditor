.PHONY: dev dev-backend dev-frontend build build-frontend install clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# ── Dev ───────────────────────────────────────────────────────────────────────
# Run both backend and frontend dev servers in parallel.
# Backend on :8080, Vite on :5173 (proxies /api → backend).
dev:
	@make -j2 dev-backend dev-frontend

dev-backend:
	go run . dashboard --no-browser

dev-frontend:
	cd frontend && npm run dev

# ── Build ─────────────────────────────────────────────────────────────────────
# Full production build: compile frontend → embed into Go binary.
build: build-frontend
	go build -ldflags "-X main.version=$(VERSION)" -o bin/clauditor .

build-frontend:
	cd frontend && npm run build

# ── Install ───────────────────────────────────────────────────────────────────
# Install Go binary to $GOPATH/bin (or ~/go/bin).
install: build
	go install -ldflags "-X main.version=$(VERSION)" .

# ── Deps ──────────────────────────────────────────────────────────────────────
# Install frontend npm dependencies (run once after cloning).
deps:
	cd frontend && npm install

# ── Clean ─────────────────────────────────────────────────────────────────────
clean:
	rm -f bin/clauditor
	rm -rf frontend/dist
