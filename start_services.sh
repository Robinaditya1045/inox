#!/usr/bin/env bash
# ==============================================================================
# Inox Background Service Launcher (start_services.sh)
# Starts Backend (Go), Frontend (Vite), and Admin Portal in a detached tmux
# session without attaching or blocking the terminal window.
# ==============================================================================

SESSION_NAME="inox"

echo "🚀 Starting Docker infrastructure..."
docker compose up -d

tmux has-session -t $SESSION_NAME 2>/dev/null
if [ $? != 0 ]; then
    echo "Creating detached tmux session: $SESSION_NAME"
    # Create new session with the Go Backend in the left pane
    tmux new-session -d -s $SESSION_NAME -n "services" -c "$PWD/backend" "go run ./cmd/server"

    # Split horizontally (right half) for the main Frontend (port 5173)
    tmux split-window -h -t $SESSION_NAME:0 -c "$PWD/frontend" "npm run dev -- --host 0.0.0.0 --port 5173"

    # Split the right half vertically (bottom right) for the Admin Portal (port 3000)
    tmux split-window -v -t $SESSION_NAME:0.1 -c "$PWD/admin-portal" "npm run dev -- --host 0.0.0.0 --port 3000"

    # Set layout so left half is Backend, right half is stacked frontends
    tmux select-layout -t $SESSION_NAME:0 main-vertical
    tmux select-pane -t $SESSION_NAME:0.0
else
    echo "Session $SESSION_NAME already running in background."
fi

echo "✅ Services are running cleanly in detached tmux session '$SESSION_NAME'."
echo "💡 To view interactive service logs anytime, run: tmux attach-session -t $SESSION_NAME"
echo "💡 To stop all services, run: tmux kill-session -t $SESSION_NAME"
