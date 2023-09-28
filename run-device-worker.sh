sudo -E YGG_LOG_LEVEL=debug \
    YGG_CONFIG_DIR="/tmp/device" \
    YGG_SOCKET_ADDR="unix:@yggd" \
    YGG_CLIENT_ID="$(cat /etc/machine-id)" \
    FLOTTA_XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
    go run cmd/device-worker/main.go