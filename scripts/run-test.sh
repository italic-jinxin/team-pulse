#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEST_DATA_DIR="$ROOT_DIR/tmp/manual-test"
PRODUCTION_DATA_DIR="$HOME/Library/Application Support/TeamPulse"
BACKEND_HOST="127.0.0.1"
BACKEND_PORT="19421"
FRONTEND_PORT="5174"
BACKEND_URL="http://$BACKEND_HOST:$BACKEND_PORT"
FRONTEND_URL="http://$BACKEND_HOST:$FRONTEND_PORT"
BACKEND_PID=""
VITE_PID=""
CLEANED_UP=0
FRESH=0

usage() {
    echo "Usage: ./scripts/run-test.sh [--fresh]"
    echo "  --fresh  Delete only the isolated test database before starting."
}

check_test_server_pid() {
    pid_file="$TEST_DATA_DIR/run/server.pid"
    [ -e "$pid_file" ] || return 0

    pid=$(sed -n '1p' "$pid_file")
    case "$pid" in
        ''|*[!0-9]*)
            echo "Refusing to clean test data: invalid server PID file at $pid_file." >&2
            return 1
            ;;
    esac

    if kill -0 "$pid" 2>/dev/null; then
        echo "Refusing to clean test data while test server PID $pid is running." >&2
        echo "Stop the existing test launcher and retry." >&2
        return 1
    fi
}

check_port() {
    port="$1"
    service="$2"
    if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
        echo "Cannot start $service: port $port is already in use." >&2
        lsof -nP -iTCP:"$port" -sTCP:LISTEN >&2 || true
        return 1
    fi
}

terminate_process() {
    pid="$1"
    [ -n "$pid" ] || return 0

    if kill -0 "$pid" 2>/dev/null; then
        kill "$pid" 2>/dev/null || true
    fi
}

wait_for_process() {
    pid="$1"
    [ -n "$pid" ] || return 0

    if wait "$pid" 2>/dev/null; then
        :
    else
        :
    fi
}

cleanup() {
    [ "$CLEANED_UP" -eq 0 ] || return 0
    CLEANED_UP=1
    terminate_process "$VITE_PID"
    terminate_process "$BACKEND_PID"
    wait_for_process "$VITE_PID"
    wait_for_process "$BACKEND_PID"
}

on_exit() {
    status=$?
    trap - 0 HUP INT TERM
    cleanup
    exit "$status"
}

on_signal() {
    status="$1"
    trap - 0 HUP INT TERM
    cleanup
    exit "$status"
}

wait_for_backend() {
    attempts=0
    while [ "$attempts" -lt 60 ]; do
        if curl -fsS "$BACKEND_URL/api/health" >/dev/null 2>&1; then
            return 0
        fi
        if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
            if wait "$BACKEND_PID"; then
                status=0
            else
                status=$?
            fi
            BACKEND_PID=""
            echo "Backend exited before becoming healthy (status $status)." >&2
            return 1
        fi
        attempts=$((attempts + 1))
        sleep 0.5
    done

    echo "Backend did not become healthy at $BACKEND_URL/api/health within 30 seconds." >&2
    return 1
}

supervise() {
    while :; do
        if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
            if wait "$BACKEND_PID"; then
                status=0
            else
                status=$?
            fi
            BACKEND_PID=""
            echo "Backend exited (status $status); stopping frontend." >&2
            return "$status"
        fi
        if ! kill -0 "$VITE_PID" 2>/dev/null; then
            if wait "$VITE_PID"; then
                status=0
            else
                status=$?
            fi
            VITE_PID=""
            echo "Frontend exited (status $status); stopping backend." >&2
            return "$status"
        fi
        sleep 1
    done
}

trap 'on_exit' 0
trap 'on_signal 129' HUP
trap 'on_signal 130' INT
trap 'on_signal 143' TERM

case "${1:-}" in
    "")
        ;;
    --fresh)
        FRESH=1
        ;;
    --help|-h)
        usage
        exit 0
        ;;
    *)
        usage >&2
        exit 2
        ;;
esac

command -v lsof >/dev/null 2>&1 || {
    echo "Cannot check development ports: lsof is not installed." >&2
    exit 1
}
command -v curl >/dev/null 2>&1 || {
    echo "Cannot check backend health: curl is not installed." >&2
    exit 1
}

if [ "$FRESH" -eq 1 ]; then
    if [ "$TEST_DATA_DIR" != "$ROOT_DIR/tmp/manual-test" ]; then
        echo "Refusing to clean an unexpected data directory." >&2
        exit 1
    fi
    check_test_server_pid
fi

check_port "$BACKEND_PORT" "TeamPulse backend"
check_port "$FRONTEND_PORT" "Vite frontend"

if [ "$FRESH" -eq 1 ]; then
    rm -rf "$TEST_DATA_DIR"
    echo "Cleared test data: $TEST_DATA_DIR"
fi

mkdir -p "$TEST_DATA_DIR"

echo "TeamPulse test data: $TEST_DATA_DIR"
echo "Production data is untouched: $PRODUCTION_DATA_DIR"
echo "Backend URL: $BACKEND_URL"
echo "Frontend URL (Vite HMR): $FRONTEND_URL"

make -C "$ROOT_DIR" backend
(cd "$ROOT_DIR/web" && npm install)

"$ROOT_DIR/build/teampulse-server" \
    -data-dir "$TEST_DATA_DIR" \
    -host "$BACKEND_HOST" \
    -port "$BACKEND_PORT" &
BACKEND_PID=$!

wait_for_backend

(
    cd "$ROOT_DIR/web"
    exec "$ROOT_DIR/web/node_modules/.bin/vite" \
        --host "$BACKEND_HOST" \
        --port "$FRONTEND_PORT" \
        --strictPort
) &
VITE_PID=$!

echo "Open $FRONTEND_URL (API proxy: $BACKEND_URL)."

if supervise; then
    exit 0
else
    exit $?
fi
