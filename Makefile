.PHONY: dev dev-backend dev-frontend build build-frontend install clean

# ── Dev ───────────────────────────────────────────────────────────────────────
# Run backend dev server. Frontend dev server runs if npm/vite available.
# Backend on :8080, Vite on :5173 (proxies /api → backend).
dev: setup-dist
	@make -j2 dev-backend dev-frontend || true

dev-backend:
	go run . dashboard --no-browser

dev-frontend:
	cd frontend && npm run dev 2>/dev/null || echo "Frontend dev server not available (run 'make deps' to install npm dependencies)"

# Create minimal frontend/dist for development (satisfies go:embed)
setup-dist:
	mkdir -p frontend/dist
	echo '<!DOCTYPE html><html><head><meta name="clauditor-dev-mode" content="1"></head><body></body></html>' > frontend/dist/index.html

# ── Build ─────────────────────────────────────────────────────────────────────
# Full production build: compile frontend → embed into Go binary.
build: build-frontend
	go build -o bin/clauditor .

build-frontend:
	cd frontend && npm run build

# ── Install ───────────────────────────────────────────────────────────────────
# Install Go binary to $GOPATH/bin (or ~/go/bin).
install: build
	go install .

# ── Deps ──────────────────────────────────────────────────────────────────────
# Install frontend npm dependencies (run once after cloning).
deps:
	cd frontend && npm install

# ── Clean ─────────────────────────────────────────────────────────────────────
clean:
	rm -f bin/clauditor
	rm -rf frontend/dist
