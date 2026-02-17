# auto-shutdown

A lightweight daemon that automatically powers off a Linux system when two conditions are met:

1. The current time is past a configurable threshold (default **22:00**).
2. No user activity has been detected for a configurable idle period (default **30 minutes**).

It runs as a systemd service and requires no runtime dependencies beyond the Go standard library and `gopkg.in/ini.v1` for configuration parsing.

## How it works

The daemon loops on a configurable interval (default 60 s). Each tick it checks:

- **Time gate** — is the current wall-clock time at or past the configured `after_time`? If not, it sleeps and tries again.
- **Idle detection** — it probes up to three sources and takes the *minimum* idle time (most recent activity wins):
  | Source | What it checks |
  |---|---|
  | `xprintidle` | X11 display server idle time (milliseconds) |
  | `loginctl` | systemd session idle hints for every logged-in session |
  | `/dev/input/mice` | Last access time of the mouse input device |

  If no source is available the daemon assumes the system is active (safe default).
- **Shutdown** — when idle time meets or exceeds the threshold, the daemon runs `shutdown -h now`.

## Installation

Prerequisites: Go 1.21+ and a Linux system with systemd.

```bash
git clone <repo-url> && cd auto-shutdown
sudo ./install.sh
# sudo -E env PATH=$PATH ./install.sh # if running from a Go environment where $GOPATH/bin is not in root's PATH
```

`install.sh` will:

1. Build the Go binary.
2. Copy the binary to `/usr/local/bin/auto-shutdown`.
3. Install the default config to `/etc/auto-shutdown.conf` (skips if it already exists).
4. Install the systemd unit to `/etc/systemd/system/auto-shutdown.service`.
5. Enable and start the service.

### Manual build & install

```bash
go build -o auto-shutdown .
sudo install -m 755 auto-shutdown /usr/local/bin/auto-shutdown
sudo install -m 644 auto-shutdown.conf /etc/auto-shutdown.conf
sudo install -m 644 auto-shutdown.service /etc/systemd/system/auto-shutdown.service
sudo systemctl daemon-reload
sudo systemctl enable --now auto-shutdown.service
```

## Configuration

Edit `/etc/auto-shutdown.conf` then restart the service.

```ini
[shutdown]
# Time after which shutdown is allowed (24-hour format)
after_time = 22:00

# System must be idle for this many minutes before shutting down
idle_minutes = 30

# How often (seconds) to check idle state
check_interval_seconds = 60

# Set to true to log what would happen without actually shutting down
dry_run = false
```

| Key | Default | Description |
|---|---|---|
| `after_time` | `22:00` | Earliest time of day (HH:MM, 24h) the daemon will consider shutting down |
| `idle_minutes` | `30` | Required continuous idle time before shutdown triggers |
| `check_interval_seconds` | `60` | Polling interval in seconds |
| `dry_run` | `false` | When `true`, logs the shutdown action without executing it |

## Useful commands

```bash
# Check service status
sudo systemctl status auto-shutdown

# Follow logs in real time
sudo journalctl -u auto-shutdown -f

# Stop the service (prevent auto-shutdown temporarily)
sudo systemctl stop auto-shutdown

# Disable the service entirely (won't start on boot)
sudo systemctl disable auto-shutdown

# Re-enable and start
sudo systemctl enable --now auto-shutdown

# Edit config, then restart to pick up changes
sudo vim /etc/auto-shutdown.conf
sudo systemctl restart auto-shutdown
```

## Testing with dry run

Set `dry_run = true` in the config and restart the service. The daemon will log when it *would* shut down without actually doing so:

```
2025/01/15 22:31:00 DRY-RUN: would execute 'shutdown -h now'
```

## Uninstall

```bash
sudo systemctl disable --now auto-shutdown.service
sudo rm /etc/systemd/system/auto-shutdown.service
sudo rm /usr/local/bin/auto-shutdown
sudo rm /etc/auto-shutdown.conf   # optional: remove config
sudo systemctl daemon-reload
```

## Files

| File | Installed to | Purpose |
|---|---|---|
| `main.go` | — | Main daemon source |
| `atime_linux.go` | — | Linux-specific file access time helper |
| `atime_other.go` | — | Fallback for non-Linux builds |
| `auto-shutdown.conf` | `/etc/auto-shutdown.conf` | Configuration file |
| `auto-shutdown.service` | `/etc/systemd/system/` | systemd unit file |
| `install.sh` | — | Build and install script |
