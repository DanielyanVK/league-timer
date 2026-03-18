//go:build windows

package main

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Spell timer state (per spell icon)
// ---------------------------------------------------------------------------

type spellState struct {
	champName string
	spellName string
	isActive  bool
	remaining int // seconds
	rect      rect
}

// ---------------------------------------------------------------------------
// Enemy row in the overlay
// ---------------------------------------------------------------------------

type enemyRow struct {
	champName string
	champRect rect
	spells    [2]*spellState
}

// ---------------------------------------------------------------------------
// Overlay holds all state for the overlay window.
// ---------------------------------------------------------------------------

type overlay struct {
	hwnd       syscall.Handle
	hInstance  syscall.Handle
	trayAdded  bool

	// Game state
	mu         sync.Mutex
	gameActive bool
	enemies    []enemyRow
	hasteCache map[string]int // champName → haste

	// Position & drag
	cfg     persistedConfig
	dragging bool
	dragStartX, dragStartY int32

	// Tray icon handle
	trayIconHandle syscall.Handle
}

// Global overlay pointer (needed for WndProc callback).
var theOverlay *overlay

// ---------------------------------------------------------------------------
// Create and run the overlay
// ---------------------------------------------------------------------------

func newOverlay() *overlay {
	o := &overlay{
		hasteCache: make(map[string]int),
	}
	o.cfg = loadConfig()
	theOverlay = o
	return o
}

func (o *overlay) run() {
	// Get module handle
	o.hInstance = getModuleHandle()

	className := mustUTF16Ptr("SpellTimerOverlay")

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     o.hInstance,
		LpszClassName: className,
		HbrBackground: createSolidBrush(hexToCOLORREF(colorBG)),
	}
	registerClassEx(&wc)

	exStyle := uint32(exLayered | exTopmost | exToolWindow | exNoActivate)

	hwnd, err := createWindowEx(
		exStyle,
		className,
		mustUTF16Ptr("Spell Timer"),
		wsPopup,
		int32(o.cfg.X), int32(o.cfg.Y),
		windowWidth(), windowHeight(5),
		0, 0, o.hInstance, nil,
	)
	if err != nil {
		fmt.Printf("[Overlay] CreateWindowEx failed: %v\n", err)
		return
	}
	o.hwnd = hwnd

	// Set global alpha (0.85 * 255 ≈ 216)
	var alphaF float64 = globalOpacity * 255
	setLayeredWindowAttributes(o.hwnd, 0, byte(alphaF), lwaAlpha)

	// Start hidden (will show when game detected)
	showWindow(o.hwnd, swHide)

	// Setup system tray
	o.setupTray()

	// Start background game polling
	go o.pollGameLoop()

	// Start 1-second timer for countdown ticks
	setTimer(o.hwnd, timerIDTick, 1000)

	// Message loop
	var m msg
	for {
		got, err := getMessage(&m)
		if !got || err != nil {
			break
		}
		translateMessage(&m)
		dispatchMessage(&m)
	}

	// Cleanup
	o.removeTray()
	cleanupBitmaps()
}

// ---------------------------------------------------------------------------
// WndProc callback
// ---------------------------------------------------------------------------

func wndProc(hwnd syscall.Handle, message uint32, wParam, lParam uintptr) uintptr {
	o := theOverlay
	if o == nil {
		return defWindowProc(hwnd, message, wParam, lParam)
	}

	switch message {
	case wmPaint:
		o.onPaint(hwnd)
		return 0

	case wmTimer:
		if wParam == timerIDTick {
			o.onTick()
		}
		return 0

	case wmLButtonDown:
		o.onLButtonDown(hwnd, lParam)
		return 0

	case wmLButtonUp:
		o.onLButtonUp()
		return 0

	case wmMouseMove:
		o.onMouseMove(hwnd, lParam)
		return 0

	case wmRButtonDown:
		o.onRButtonDown(hwnd, lParam)
		return 0

	case wmMouseActivate:
		return uintptr(maNoActivate)

	case wmCommand:
		if wParam == idTrayQuit {
			o.quit()
		}
		return 0

	case wmTrayIcon:
		// lParam holds the mouse message
		switch uint32(lParam) {
		case wmRButtonDown:
			o.showTrayMenu()
		}
		return 0

	case wmDestroy:
		killTimer(hwnd, timerIDTick)
		postQuitMessage(0)
		return 0
	}

	return defWindowProc(hwnd, message, wParam, lParam)
}

// ---------------------------------------------------------------------------
// Paint
// ---------------------------------------------------------------------------

func (o *overlay) onPaint(hwnd syscall.Handle) {
	var ps paintStruct
	hdc := beginPaint(hwnd, &ps)
	defer endPaint(hwnd, &ps)

	var cr rect
	getClientRect(hwnd, &cr)
	w := cr.Right - cr.Left
	h := cr.Bottom - cr.Top

	// Double buffer
	memDC := createCompatibleDC(hdc)
	memBM := createCompatibleBitmap(hdc, w, h)
	oldBM := selectObject(memDC, memBM)
	defer func() {
		selectObject(memDC, oldBM)
		deleteObject(memBM)
		deleteDC(memDC)
	}()

	// Background
	bgBrush := createSolidBrush(hexToCOLORREF(colorBG))
	fillRect(memDC, &cr, bgBrush)
	deleteObject(bgBrush)

	// Border
	borderBrush := createSolidBrush(hexToCOLORREF(colorBorder))
	// Top
	fillRect(memDC, &rect{0, 0, w, borderWidth}, borderBrush)
	// Bottom
	fillRect(memDC, &rect{0, h - borderWidth, w, h}, borderBrush)
	// Left
	fillRect(memDC, &rect{0, 0, borderWidth, h}, borderBrush)
	// Right
	fillRect(memDC, &rect{w - borderWidth, 0, w, h}, borderBrush)
	deleteObject(borderBrush)

	// Handle text
	o.drawHandle(memDC)

	// Enemy rows
	o.mu.Lock()
	enemies := o.enemies
	o.mu.Unlock()
	o.drawEnemyRows(memDC, enemies)

	// Blit to screen
	bitBlt(hdc, 0, 0, w, h, memDC, 0, 0, srcCopy)
}

func (o *overlay) drawHandle(hdc syscall.Handle) {
	text := "::::"
	col := uint32(colorHandle)
	if o.cfg.Pinned {
		text = "PINNED"
		col = uint32(colorPinned)
	}

	font := createFont(12, 0, 0, 0, 700, 0, 0, 0, 0, 0, 0, 0, 0, mustUTF16Ptr(fontFamily))
	oldFont := selectObject(hdc, font)
	setBkMode(hdc, transparent)
	setTextColor(hdc, hexToCOLORREF(col))

	utf16Text := utf16.Encode([]rune(text))
	var sz point
	getTextExtentPoint32(hdc, &utf16Text[0], int32(len(utf16Text)), &sz)

	x := borderWidth + innerPadding + (rowWidth()-int32(sz.X))/2
	y := int32(borderWidth + innerPadding)
	textOut(hdc, x, y, &utf16Text[0], int32(len(utf16Text)))

	selectObject(hdc, oldFont)
	deleteObject(font)
}

func (o *overlay) drawEnemyRows(hdc syscall.Handle, enemies []enemyRow) {
	startY := int32(borderWidth + innerPadding + handleHeight + handleGap)
	startX := int32(borderWidth + innerPadding)

	bf := blendFunction{
		BlendOp:             acSrcOver,
		SourceConstantAlpha: 255,
		AlphaFormat:         acSrcAlpha,
	}

	for i, row := range enemies {
		if i > 0 && showSeparator {
			startY += separatorPadY
			sepBrush := createSolidBrush(hexToCOLORREF(colorSeparator))
			fillRect(hdc, &rect{startX + 10, startY, startX + rowWidth() - 10, startY + separatorH}, sepBrush)
			deleteObject(sepBrush)
			startY += separatorH + separatorPadY
		}

		y := startY
		x := startX

		// Champion icon (circular)
		champBM := loadIcon("champions", row.champName, iconSize, true)
		if champBM != nil && champBM.hbm != 0 {
			tmpDC := createCompatibleDC(hdc)
			oldObj := selectObject(tmpDC, champBM.hbm)
			alphaBlend(hdc, x, y, champBM.width, champBM.height, tmpDC, 0, 0, champBM.width, champBM.height, bf)
			selectObject(tmpDC, oldObj)
			deleteDC(tmpDC)
		}
		x += iconSize + champSpellGap

		// Spell icons
		for si := 0; si < 2; si++ {
			sp := row.spells[si]
			if sp == nil {
				x += iconSize + spellGap
				continue
			}
			// Update hit rect
			sp.rect = rect{x, y, x + iconSize, y + iconSize}

			spellBM := loadIcon("spells", sp.spellName, iconSize, false)
			if spellBM != nil && spellBM.hbm != 0 {
				tmpDC := createCompatibleDC(hdc)
				oldObj := selectObject(tmpDC, spellBM.hbm)
				alphaBlend(hdc, x, y, spellBM.width, spellBM.height, tmpDC, 0, 0, spellBM.width, spellBM.height, bf)
				selectObject(tmpDC, oldObj)
				deleteDC(tmpDC)
			}

			// Dim overlay + timer text if active
			if sp.isActive {
				dimBM := loadDimLayer(iconSize)
				if dimBM != nil && dimBM.hbm != 0 {
					tmpDC := createCompatibleDC(hdc)
					oldObj := selectObject(tmpDC, dimBM.hbm)
					alphaBlend(hdc, x, y, dimBM.width, dimBM.height, tmpDC, 0, 0, dimBM.width, dimBM.height, bf)
					selectObject(tmpDC, oldObj)
					deleteDC(tmpDC)
				}
				o.drawTimerText(hdc, sp, x, y)
			}

			if si == 0 {
				x += iconSize + spellGap
			} else {
				x += iconSize
			}
		}

		startY += iconSize + rowPaddingY
	}
}

func (o *overlay) drawTimerText(hdc syscall.Handle, sp *spellState, x, y int32) {
	text := formatTime(sp.remaining)
	fontSize := adaptiveFontSize(text)

	font := createFont(int32(fontSize), 0, 0, 0, 700, 0, 0, 0, 0, 0, 0, 0, 0, mustUTF16Ptr(fontFamily))
	oldFont := selectObject(hdc, font)
	setBkMode(hdc, transparent)

	utf16Text := utf16.Encode([]rune(text))
	var sz point
	getTextExtentPoint32(hdc, &utf16Text[0], int32(len(utf16Text)), &sz)

	cx := x + iconSize/2 - int32(sz.X)/2
	cy := y + iconSize/2 - int32(sz.Y)/2

	// Draw outline (black text at 8 offsets)
	setTextColor(hdc, hexToCOLORREF(colorTextOutline))
	offsets := [][2]int32{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}
	for _, off := range offsets {
		textOut(hdc, cx+off[0], cy+off[1], &utf16Text[0], int32(len(utf16Text)))
	}

	// Draw fill (white text)
	setTextColor(hdc, hexToCOLORREF(colorTextActive))
	textOut(hdc, cx, cy, &utf16Text[0], int32(len(utf16Text)))

	selectObject(hdc, oldFont)
	deleteObject(font)
}

func formatTime(seconds int) string {
	if seconds >= 60 {
		m := seconds / 60
		s := seconds % 60
		return fmt.Sprintf("%d:%02d", m, s)
	}
	return fmt.Sprintf("%d", seconds)
}

func adaptiveFontSize(text string) int {
	n := len(text)
	switch {
	case n <= 2:
		return baseFontSize + 2
	case n == 3:
		return baseFontSize + 1
	case n == 4:
		return baseFontSize - 1
	default:
		return baseFontSize - 3
	}
}

// ---------------------------------------------------------------------------
// Timer tick (every 1 second)
// ---------------------------------------------------------------------------

func (o *overlay) onTick() {
	o.mu.Lock()
	needRepaint := false
	for _, row := range o.enemies {
		for _, sp := range row.spells {
			if sp != nil && sp.isActive {
				sp.remaining--
				if sp.remaining <= 0 {
					sp.isActive = false
					sp.remaining = 0
				}
				needRepaint = true
			}
		}
	}
	o.mu.Unlock()

	if needRepaint {
		invalidateRect(o.hwnd, nil, false)
	}
}

// ---------------------------------------------------------------------------
// Mouse input
// ---------------------------------------------------------------------------

func (o *overlay) onLButtonDown(hwnd syscall.Handle, lParam uintptr) {
	mx := int32(int16(lParam & 0xFFFF))
	my := int32(int16((lParam >> 16) & 0xFFFF))

	// Check if click is on handle area (start drag)
	handleBottom := int32(borderWidth + innerPadding + handleHeight)
	if my < handleBottom && !o.cfg.Pinned {
		o.dragging = true
		var pt point
		getCursorPos(&pt)
		var wr rect
		getWindowRect(hwnd, &wr)
		o.dragStartX = pt.X - wr.Left
		o.dragStartY = pt.Y - wr.Top
		setCapture(hwnd)
		return
	}

	// Check if click is on a spell icon → start timer
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, row := range o.enemies {
		for _, sp := range row.spells {
			if sp == nil || sp.isActive {
				continue
			}
			if pointInRect(mx, my, sp.rect) {
				o.startSpellTimer(sp, row.champName)
				invalidateRect(o.hwnd, nil, false)
				return
			}
		}
	}
}

func (o *overlay) onLButtonUp() {
	if o.dragging {
		o.dragging = false
		releaseCapture()
		// Save position
		var wr rect
		getWindowRect(o.hwnd, &wr)
		o.cfg.X = int(wr.Left)
		o.cfg.Y = int(wr.Top)
	}
}

func (o *overlay) onMouseMove(hwnd syscall.Handle, lParam uintptr) {
	if !o.dragging {
		return
	}
	var pt point
	getCursorPos(&pt)
	newX := pt.X - o.dragStartX
	newY := pt.Y - o.dragStartY
	var wr rect
	getWindowRect(hwnd, &wr)
	moveWindow(hwnd, newX, newY, wr.Right-wr.Left, wr.Bottom-wr.Top, true)
	o.cfg.X = int(newX)
	o.cfg.Y = int(newY)
}

func (o *overlay) onRButtonDown(hwnd syscall.Handle, lParam uintptr) {
	mx := int32(int16(lParam & 0xFFFF))
	my := int32(int16((lParam >> 16) & 0xFFFF))

	// Right-click on handle → toggle pin
	handleBottom := int32(borderWidth + innerPadding + handleHeight)
	if my < handleBottom {
		o.cfg.Pinned = !o.cfg.Pinned
		saveConfig(o.cfg)
		invalidateRect(o.hwnd, nil, false)
		return
	}

	// Right-click on active spell → reset timer
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, row := range o.enemies {
		for _, sp := range row.spells {
			if sp == nil || !sp.isActive {
				continue
			}
			if pointInRect(mx, my, sp.rect) {
				sp.isActive = false
				sp.remaining = 0
				invalidateRect(o.hwnd, nil, false)
				return
			}
		}
	}
}

func pointInRect(x, y int32, r rect) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

// ---------------------------------------------------------------------------
// Start spell timer
// ---------------------------------------------------------------------------

func (o *overlay) startSpellTimer(sp *spellState, champName string) {
	key := strings.ToLower(sp.spellName)
	baseCD := getBaseCD(key)

	haste := o.hasteCache[champName]
	finalCD := baseCD
	if haste > 0 {
		finalCD = int(float64(baseCD) * (100.0 / float64(100+haste)))
		fmt.Printf("[Timer] %s (%s): Base %ds -> Haste %d -> %ds\n", champName, sp.spellName, baseCD, haste, finalCD)
	}

	sp.isActive = true
	sp.remaining = finalCD
}

// ---------------------------------------------------------------------------
// Game polling (runs in a background goroutine)
// ---------------------------------------------------------------------------

func (o *overlay) pollGameLoop() {
	ticker := time.NewTicker(time.Duration(checkIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		data, err := fetchGameData()
		if err != nil {
			// Game not running or ended
			o.mu.Lock()
			wasActive := o.gameActive
			if wasActive {
				fmt.Println("[Spell Timer] Match ended.")
				o.gameActive = false
				o.enemies = nil
				o.hasteCache = make(map[string]int)
				o.mu.Unlock()
				showWindow(o.hwnd, swHide)
				saveConfig(o.cfg)
			} else {
				o.mu.Unlock()
			}
			continue
		}

		enemies := parseEnemies(data)
		o.mu.Lock()

		// Update haste cache
		for _, e := range enemies {
			o.hasteCache[e.Champ] = e.Haste
		}

		if !o.gameActive {
			fmt.Println("[Spell Timer] Match found!")
			o.gameActive = true
			o.buildEnemyRows(enemies)
			o.mu.Unlock()

			// Resize and show window
			n := len(enemies)
			ww := windowWidth()
			wh := windowHeight(n)
			moveWindow(o.hwnd, int32(o.cfg.X), int32(o.cfg.Y), ww, wh, true)
			showWindow(o.hwnd, swShow)
			setWindowPos(o.hwnd, hwndTopmost, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoActivate)
			invalidateRect(o.hwnd, nil, false)
		} else {
			o.mu.Unlock()
			// No UI rebuild – haste is already updated, timers preserved.
		}
	}
}

func (o *overlay) buildEnemyRows(enemies []enemy) {
	o.enemies = make([]enemyRow, len(enemies))
	for i, e := range enemies {
		o.enemies[i] = enemyRow{
			champName: e.Champ,
			spells: [2]*spellState{
				{champName: e.Champ, spellName: e.Spell1},
				{champName: e.Champ, spellName: e.Spell2},
			},
		}
	}
}

// ---------------------------------------------------------------------------
// System tray
// ---------------------------------------------------------------------------

func (o *overlay) setupTray() {
	// Try to load icon from file
	icoPath := resourcePath("ico\\icon.ico")
	hIcon := loadImage(0, mustUTF16Ptr(icoPath), imageIcon, 0, 0, lrLoadFromFile|lrDefaultSize)
	if hIcon == 0 {
		// Fallback: use a default system icon
		hIcon = loadImage(0, mustUTF16Ptr("#32512"), imageIcon, 0, 0, lrDefaultSize) // IDI_APPLICATION-like
	}
	o.trayIconHandle = hIcon

	nid := notifyIconDataW{
		HWnd:             o.hwnd,
		UID:              1,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: wmTrayIcon,
		HIcon:            hIcon,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copy(nid.SzTip[:], utf16.Encode([]rune("Spell Timer\x00")))

	shellNotifyIcon(nimAdd, &nid)
	o.trayAdded = true
}

func (o *overlay) removeTray() {
	if !o.trayAdded {
		return
	}
	nid := notifyIconDataW{
		HWnd: o.hwnd,
		UID:  1,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	shellNotifyIcon(nimDelete, &nid)
}

func (o *overlay) showTrayMenu() {
	menu := createPopupMenu()
	appendMenu(menu, mfString, idTrayQuit, mustUTF16Ptr("Quit"))

	var pt point
	getCursorPos(&pt)

	// Required for the menu to disappear when clicking outside
	setForegroundWindow(o.hwnd)
	trackPopupMenu(menu, tpmLeftAlign|tpmRightButton, pt.X, pt.Y, o.hwnd)
	destroyMenu(menu)
}

func (o *overlay) quit() {
	o.cfg.X, o.cfg.Y = func() (int, int) {
		var wr rect
		getWindowRect(o.hwnd, &wr)
		return int(wr.Left), int(wr.Top)
	}()
	saveConfig(o.cfg)
	o.removeTray()
	destroyWindow(o.hwnd)
}
