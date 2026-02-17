#!/usr/bin/env bash
set -euo pipefail

echo "Building auto-shutdown..."
go build -o auto-shutdown .

echo "Installing auto-shutdown..."

# Install the binary
install -m 755 auto-shutdown /usr/local/bin/auto-shutdown

# Install the config (don't overwrite if it already exists)
if [ ! -f /etc/auto-shutdown.conf ]; then
    install -m 644 auto-shutdown.conf /etc/auto-shutdown.conf
    echo "Installed default config to /etc/auto-shutdown.conf"
else
    echo "/etc/auto-shutdown.conf already exists – skipping"
fi

# Install the systemd unit
install -m 644 auto-shutdown.service /etc/systemd/system/auto-shutdown.service

# Reload and enable
systemctl daemon-reload
systemctl enable --now auto-shutdown.service

echo "Done. Service status:"
systemctl status auto-shutdown.service --no-pager
