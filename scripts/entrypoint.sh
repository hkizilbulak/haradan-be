#!/bin/sh
set -e

# If specific command or binary passed (e.g. "./worker", "./api", "sh"), execute it directly
if [ "$#" -gt 0 ] && [ "$1" != "all" ] && [ "$1" != "start" ]; then
    exec "$@"
fi

echo "Starting Haradan Worker..."
./worker &
WORKER_PID=$!

echo "Starting Haradan API..."
./api &
API_PID=$!

cleanup() {
    echo "Shutting down Haradan services..."
    kill -TERM "$WORKER_PID" 2>/dev/null || true
    kill -TERM "$API_PID" 2>/dev/null || true
    wait "$WORKER_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
    exit 0
}

trap cleanup SIGTERM SIGINT SIGHUP

# Wait while both processes are alive; if either exits, shut down
while kill -0 "$API_PID" 2>/dev/null && kill -0 "$WORKER_PID" 2>/dev/null; do
    sleep 2 &
    wait $! 2>/dev/null || true
done

cleanup
