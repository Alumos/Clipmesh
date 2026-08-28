#!/bin/sh
set -eu

/usr/local/bin/clipmesh &
API_PID=$!

shutdown() {
    kill -TERM "$API_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
}
trap shutdown INT TERM

nginx -g 'daemon off;' &
NGINX_PID=$!

wait "$NGINX_PID"
STATUS=$?
kill -TERM "$API_PID" 2>/dev/null || true
wait "$API_PID" 2>/dev/null || true
exit "$STATUS"
