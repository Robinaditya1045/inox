.PHONY: dev up down clean

# Start Docker containers
up:
	@echo "🚀 Starting Docker containers (Postgres, Redis, MinIO)..."
	@docker compose up -d

# Run all 3 services concurrently with clean color-coded logging
dev: up
	@npx -y concurrently -n "BACKEND,CLIENT,ADMIN" -c "cyan,green,magenta" --kill-others \
		"cd backend && go run ./cmd/server" \
		"cd frontend && npm run dev -- --host 0.0.0.0 --port 5173" \
		"cd admin-portal && npm run dev -- --host 0.0.0.0 --port 3000"

# Stop Docker containers
down:
	@echo "🛑 Stopping Docker containers..."
	@docker compose down

# Stop everything (kill tmux sessions and background docker)
clean: down
	@tmux kill-session -t inox 2>/dev/null || true
	@echo "✨ Cleaned up session and containers."
