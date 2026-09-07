# macos-same-window-switcher

One-shot CLI that cycles windows of the focused macOS app. Bind it from skhd (or similar). The binary has **no hotkeys** and **no daemon**.

Design notes: [docs/design.md](docs/design.md).

## Install

Primary path is the unsigned CLI at a fixed location:

```
make install
```

That copies the binary to `~/.local/bin/same-window-switcher`. Keep using that path so the Accessibility (TCC) entry stays stable across rebuilds.

If System Settings → Privacy & Security → Accessibility does not list that CLI (some macOS 26 Tahoe builds hide unsigned command-line tools), fall back to the `.app` wrapper:

```
make install-app
```

That creates `~/Applications/SameWindowSwitcher.app`. Point skhd at `Contents/MacOS/same-window-switcher` inside the bundle.

## Accessibility

Grant **this binary** (the installed CLI, or the `.app` if you used `install-app`), **not skhd**.

```
same-window-switcher doctor
```

`doctor` reports trust, codesign, bundle path, config, and AX timing. If Settings still does not list the executable, use `make install-app`.

## Disable the OS shortcut

Disable or rebind System Settings → Keyboard → Keyboard Shortcuts → Keyboard → *Move focus to next window* (the OS ⌘-grave shortcut). Otherwise the system steals the chord.

## skhd

Bind `next` / `prev` in `~/.skhdrc`. Use **keycodes**, not a literal grave/backtick (skhd's parser breaks on it). Use an **absolute path**; skhd's `PATH` is not your login shell.

ANSI US physical grave is `0x32`. Copy-pasteable snippet: [docs/skhdrc.example](docs/skhdrc.example).

```
# ANSI US grave. Confirm with: skhd --observe
# Disable OS "Move focus to next window" first.
cmd - \` : $HOME/.local/bin/same-window-switcher next
cmd + shift - \` : $HOME/.local/bin/same-window-switcher prev
```

After `make install-app`, point at the bundle binary instead:

```
cmd - \` : $HOME/Applications/SameWindowSwitcher.app/Contents/MacOS/same-window-switcher next
cmd + shift - \` : $HOME/Applications/SameWindowSwitcher.app/Contents/MacOS/same-window-switcher prev
```

**JIS and other layouts:** run `skhd --observe` and bind the keycode you see. Do not guess. This project does not publish a JIS keycode.

Key-repeat is serialized by flock: overlapping `next`/`prev` **wait** rather than skip. Wait is OK.

## Commands

| Command | What it does |
|---|---|
| `next` | raise the next eligible window of the focused app |
| `prev` | raise the previous eligible window |
| `list` | print eligible windows (`-v` also shows dropped ones) |
| `doctor` | diagnose Accessibility, path, codesign, AX timing |
| `version` | print version |

Success is silent. `-v` (or `SAME_WINDOW_SWITCHER_DEBUG=1`) prints `pid=… policy=spatial action=reuse index=2/4 id=w:1235 elapsed=12ms` on stderr. `--dry-run` selects without raising or writing cycle state. `--policy NAME` overrides the sort policy for one run.

Zero or one eligible window is a successful no-op (exit 0).

## Config

Optional. Defaults work with no file.

Search order (first wins): `--config`, `$SAME_WINDOW_SWITCHER_CONFIG`, `$XDG_CONFIG_HOME/same-window-switcher/config.toml`, `~/.config/same-window-switcher/config.toml`. `~/Library/Application Support` is never searched. Unknown keys are errors.

```toml
[cycle]
policy = "spatial"          # spatial | window-id | z-order | mru
sticky_ms = 2000
wrap = true

[filter]
include_minimized = false
include_dialogs = false
include_floating = false
# allowed_subroles = ["AXStandardWindow"]
```

`include_minimized=true` also unminimizes before raise.

## Limitations

- Other Spaces are not hopped. Only windows Accessibility returns for the focused app (same restriction as native ⌘-grave).
- Stage Manager: native ⌘-grave changes meaning; this tool still cycles AX windows of the focused app when AX returns them. Disable the OS shortcut.
- Minimized windows are excluded by default.
- Empty / unknown subrole is dropped (`list -v` shows `dropped=subrole:`).
- 0–1 eligible windows: no-op, exit 0.
- No overlay, thumbnails, or in-process hotkeys.

## Build

Requires Go 1.22+ (and macOS Command Line Tools for `make build`).

```
make test
make build
```

The binary is written to `bin/same-window-switcher`. `make install` copies it to `~/.local/bin/same-window-switcher`.
