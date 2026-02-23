# claude-limit-cli

## THIS APP WAS MOSTLY VIBE CODED, YOU HAVE BEEN WARNED!

A small CLI tool that shows your Claude Pro usage limits (5-hour and 7-day windows) and the time remaining until each resets.

There is no official Anthropic API for subscription quota data. This tool calls the same internal endpoint the Claude web UI uses, authenticated with your browser session key.

Requires a [Nerd Font](https://www.nerdfonts.com/) to render the icons correctly.

## Installation

```sh
git clone https://github.com/koriwi/claude-limit-cli
cd claude-limit-cli
go build -o claude-limit .
```

Copy the binary somewhere on your `$PATH`, for example `~/.local/bin/`.

## Getting your session key

1. Open `https://claude.ai/settings/usage` in your browser.
2. Open DevTools (F12) and go to **Application > Cookies > claude.ai**.
3. Copy the value of the `sessionKey` cookie. It starts with `sk-ant-`.

## Configuration

The config file lives at `~/.config/claude-usage/config`. The directory is created automatically on first run.

```ini
session_key=sk-ant-your-key-here
# org_id is optional, auto-fetched if not set
# org_id=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

You can also pass credentials via environment variables or flags:

| Method | Session key | Org ID |
|---|---|---|
| Config file | `session_key=` | `org_id=` |
| Environment | `CLAUDE_SESSION_KEY` | `CLAUDE_ORG_ID` |
| Flag | `--session-key` | `--org-id` |

Flags take precedence over environment variables, which take precedence over the config file.

## Usage

```
claude-limit [flags]
```

| Flag | Description |
|---|---|
| `--compact` | One-line output, suitable for status bars |
| `--no-color` | Disable ANSI color codes |
| `--refresh` | Bypass the cache and fetch live data |
| `--session-key` | Session key (overrides config and env) |
| `--org-id` | Organization UUID (overrides config and env) |

Default output:

```
  󰧱 Claude Pro -- Personal

  󰔟  5-Hour     ████████████░░░░░░░░   60.0%   󰑓 1h 45m

  󰸗  7-Day      ████░░░░░░░░░░░░░░░░   20.0%   󰑓 3d 14h
```

Compact output (`--compact`):

```
󰔟 60.0% 1h 45m   󰸗 20.0% 3d 14h
```

The progress bar and icon color reflects utilization: green below 60%, yellow from 60%, red from 85%.

## Caching

Results are cached at `~/.config/claude-usage/cache.json`. The cache is considered stale and refreshed automatically when:

- It is older than 30 minutes.
- A usage window's reset time has passed.
- `--refresh` is passed.

## Waybar

Add a custom module to your Waybar config:

```json
"custom/claude": {
  "exec": "~/.local/bin/claude-limit --compact --no-color",
  "on-click": "~/.local/bin/claude-limit --refresh",
  "format": "Claude {}",
  "interval": 1
}
```

`interval: 1` re-runs the binary every second, but since results are cached the output only changes when the cache is refreshed. The `on-click` binding triggers a live fetch immediately.

Add `custom/claude` to one of your bar arrays to display it:

```json
"modules-right": ["custom/claude", "clock"]
```
