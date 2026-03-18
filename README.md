# Spell Timer

Small Windows overlay that displays enemy summoner spell cooldowns for League of Legends.
Go rewrite of [lovelybbq/lol-spell-timer](https://github.com/lovelybbq/lol-spell-timer).

## Quick start

### 1. Download assets (icons)

```cmd
go run ./cmd/download-assets
```

### 2. Build

```cmd
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-H windowsgui" -o spell-timer.exe .
```

> The `-H windowsgui` flag hides the console window so the app runs silently in the tray.

### 3. Run

```cmd
spell-timer.exe
```

Make sure the `assets/` and `ico/` directories are next to the executable.

## Cross-compile from macOS / Linux

```bash
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o spell-timer.exe .
```

Then copy `spell-timer.exe`, `assets/`, and `ico/` to your Windows machine.

## Behavior & notes

- The app runs in the **system tray** after launch (right-click the tray icon → Quit).
- It **auto-detects** when a game starts and shows enemy summoner spells; hides when the game ends.
- The overlay **remembers its position** (saved in `config.json` next to the exe).
- **Drag** the `::::` handle to reposition. **Right-click** the handle to pin/unpin.
- **Left-click** a spell icon to start its cooldown timer.
- **Right-click** an active spell icon to reset its cooldown immediately.
- Cooldown calculations include enemy **summoner spell haste** from items (Ionian Boots, etc.).
- Spell cooldowns are updated from DDragon on each launch.

> **Important:** League of Legends must run in **Borderless** or **Windowed** mode
> (not Fullscreen) for the overlay to appear on top. This is the default LoL setting.
> Go to in-game Settings → Video → Window Mode → Borderless.

## Icon

The build script automatically:
1. Generates `ico/icon.ico` from the SummonerFlash spell PNG (multi-size: 16/32/48px)
2. Embeds it into the exe using [rsrc](https://github.com/akavel/rsrc) so it shows in Explorer and the taskbar
3. Uses the same icon for the system tray notification

## Project structure

```
├── main.go                    # Entry point, single-instance mutex
├── config.go                  # Constants, spell timers, config.json persistence
├── win32.go                   # Win32 API bindings (user32, gdi32, shell32, msimg32)
├── assets.go                  # PNG → HBITMAP loading, circular mask, alpha compositing
├── api.go                     # LoL Live Client API + DDragon API client
├── overlay.go                 # Overlay window, rendering, input, drag, tray
├── cmd/download-assets/
│   └── main.go                # Asset downloader + icon generator
├── build.bat                  # One-click build (assets + icon + exe)
├── download-assets.bat        # Update assets only
├── assets/
│   ├── champions/             # Champion icon PNGs
│   └── spells/                # Spell icon PNGs
└── ico/
    └── icon.ico               # Generated app icon
```

## Dependencies

- Go 1.22+
- `golang.org/x/sys` (for `windows` package)
- No CGO required — pure Go + Win32 syscalls.
