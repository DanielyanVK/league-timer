# Spell Timer

Small Windows overlay that displays enemy summoner spell cooldowns for League of Legends.
Inspired by [lovelybbq/lol-spell-timer](https://github.com/lovelybbq/lol-spell-timer).

<img width="142" height="261" alt="image" src="https://github.com/user-attachments/assets/13561614-3114-4160-8407-43978944e2b3" />

## Quick start
Download and run latest .exe from release page

## Behavior & notes

> **Important:** League of Legends must run in **Borderless** or **Windowed** mode
> (not Fullscreen) for the overlay to appear on top. This is the default LoL setting.
> Go to in-game Settings → Video → Window Mode → Borderless.

- The app runs in the **system tray** after launch (right-click the tray icon → Quit).
- It **auto-detects** when a game starts and shows enemy summoner spells; hides when the game ends.
- The overlay **remembers its position** (saved in `config.json` next to the exe).
- **Drag** the `::::` handle to reposition. **Right-click** the handle to pin/unpin.
- **Left-click** a spell icon to start its cooldown timer.
- **Right-click** an active spell icon to reset its cooldown immediately.
- Cooldown calculations include enemy **summoner spell haste** from items (Ionian Boots, etc.).
- Spell cooldowns are updated from DDragon on each launch.

## Local .bat build
Run build.bat to create .exe file

## Local native build
### 1. Build

```cmd
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-H windowsgui" -o spell-timer.exe .
```

> The `-H windowsgui` flag hides the console window so the app runs silently in the tray.

### 2. Run

```cmd
spell-timer.exe
```

Make sure the `assets/` and `ico/` directories are next to the executable.

## Cross-compile from macOS / Linux

```bash
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o spell-timer.exe .
```

Then copy `spell-timer.exe`, `assets/`, and `ico/` to your Windows machine.

## Dependencies

- Go 1.22+
- `golang.org/x/sys` (for `windows` package)
- No CGO required — pure Go + Win32 syscalls.
