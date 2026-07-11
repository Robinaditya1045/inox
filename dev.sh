#!/usr/bin/env bash
# Inox Multi-Service Dev Orchestrator using tmux
SESSION_NAME="inox"

# 1. Start Docker containers (Postgres, Redis, MinIO)
echo "🚀 Starting Docker infrastructure..."
docker compose up -d

# 2. Check if tmux session already exists
tmux has-session -t $SESSION_NAME 2>/dev/null
if [ $? != 0 ]; then
    echo "Creating new tmux session: $SESSION_NAME"
    # Create new session with the Go Backend in the left pane
    tmux new-session -d -s $SESSION_NAME -n "services" -c "$PWD/backend" "go run ./cmd/server"

    # Split horizontally (right half) for the main Frontend (port 5173)
    tmux split-window -h -t $SESSION_NAME:0 -c "$PWD/frontend" "npm run dev -- --port 5173"

    # Split the right half vertically (bottom right) for the Admin Portal (port 3000)
    tmux split-window -v -t $SESSION_NAME:0.1 -c "$PWD/admin-portal" "npm run dev -- --port 3000"

    # Set layout so left half is Backend, right half is stacked frontends
    tmux select-layout -t $SESSION_NAME:0 main-vertical

    # Select the backend pane by default
    tmux select-pane -t $SESSION_NAME:0.0
else
    echo "Session $SESSION_NAME already exists. Attaching..."
fi

# 3. Attach to the session
tmux attach-session -t $SESSION_NAME
