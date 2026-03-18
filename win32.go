//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ---------------------------------------------------------------------------
// DLL handles
// ---------------------------------------------------------------------------

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	msimg32  = windows.NewLazySystemDLL("msimg32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
)

// ---------------------------------------------------------------------------
// Proc addresses
// ---------------------------------------------------------------------------

var (
	// user32
	pRegisterClassExW          = user32.NewProc("RegisterClassExW")
	pCreateWindowExW           = user32.NewProc("CreateWindowExW")
	pDefWindowProcW            = user32.NewProc("DefWindowProcW")
	pGetMessageW               = user32.NewProc("GetMessageW")
	pTranslateMessage          = user32.NewProc("TranslateMessage")
	pDispatchMessageW          = user32.NewProc("DispatchMessageW")
	pShowWindow                = user32.NewProc("ShowWindow")
	pDestroyWindow             = user32.NewProc("DestroyWindow")
	pPostQuitMessage           = user32.NewProc("PostQuitMessage")
	pPostMessageW              = user32.NewProc("PostMessageW")
	pSetWindowPos              = user32.NewProc("SetWindowPos")
	pSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	pGetWindowLongPtrW         = user32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtrW         = user32.NewProc("SetWindowLongPtrW")
	pBeginPaint                = user32.NewProc("BeginPaint")
	pEndPaint                  = user32.NewProc("EndPaint")
	pInvalidateRect            = user32.NewProc("InvalidateRect")
	pGetDC                     = user32.NewProc("GetDC")
	pReleaseDC                 = user32.NewProc("ReleaseDC")
	pSetCapture                = user32.NewProc("SetCapture")
	pReleaseCapture            = user32.NewProc("ReleaseCapture")
	pGetCursorPos              = user32.NewProc("GetCursorPos")
	pSetTimer                  = user32.NewProc("SetTimer")
	pKillTimer                 = user32.NewProc("KillTimer")
	pLoadImageW                = user32.NewProc("LoadImageW")
	pMoveWindow                = user32.NewProc("MoveWindow")
	pGetWindowRect             = user32.NewProc("GetWindowRect")
	pCreatePopupMenu           = user32.NewProc("CreatePopupMenu")
	pAppendMenuW               = user32.NewProc("AppendMenuW")
	pTrackPopupMenu            = user32.NewProc("TrackPopupMenu")
	pDestroyMenu               = user32.NewProc("DestroyMenu")
	pSetForegroundWindow       = user32.NewProc("SetForegroundWindow")
	pFillRect                  = user32.NewProc("FillRect")
	pGetSystemMetrics          = user32.NewProc("GetSystemMetrics")
	pGetClientRect             = user32.NewProc("GetClientRect")
	pLoadCursorW               = user32.NewProc("LoadCursorW")

	// gdi32
	pCreateCompatibleDC      = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap  = gdi32.NewProc("CreateCompatibleBitmap")
	pDeleteDC                = gdi32.NewProc("DeleteDC")
	pSelectObject            = gdi32.NewProc("SelectObject")
	pDeleteObject            = gdi32.NewProc("DeleteObject")
	pCreateDIBSection        = gdi32.NewProc("CreateDIBSection")
	pCreateSolidBrush        = gdi32.NewProc("CreateSolidBrush")
	pCreateFontW             = gdi32.NewProc("CreateFontW")
	pSetBkMode               = gdi32.NewProc("SetBkMode")
	pSetTextColor            = gdi32.NewProc("SetTextColor")
	pTextOutW                = gdi32.NewProc("TextOutW")
	pGetTextExtentPoint32W   = gdi32.NewProc("GetTextExtentPoint32W")
	pBitBlt                  = gdi32.NewProc("BitBlt")

	// msimg32
	pAlphaBlend = msimg32.NewProc("AlphaBlend")

	// shell32
	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")

	// kernel32
	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	// Window styles
	wsPopup     = 0x80000000
	wsVisible   = 0x10000000
	wsChild     = 0x40000000
	exLayered   = 0x00080000
	exTopmost   = 0x00000008
	exToolWindow = 0x00000080
	exNoActivate = 0x08000000

	// Window messages
	wmDestroy      = 0x0002
	wmPaint        = 0x000F
	wmClose        = 0x0010
	wmCommand      = 0x0111
	wmTimer        = 0x0113
	wmLButtonDown  = 0x0201
	wmLButtonUp    = 0x0202
	wmMouseMove    = 0x0200
	wmRButtonDown  = 0x0204
	wmMouseActivate = 0x0021
	wmApp          = 0x8000

	// ShowWindow
	swHide = 0
	swShow = 5

	// SetWindowPos
	hwndTopmost   = ^uintptr(0) // (HWND)-1 = HWND_TOPMOST
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010

	// SetLayeredWindowAttributes
	lwaAlpha = 0x02

	// GetWindowLong
	gwlExStyle = -20

	// Mouse activate
	maNoActivateAndEat = 4
	maNoActivate       = 3

	// GDI
	srcCopy    = 0x00CC0020
	transparent = 1 // SetBkMode

	biRGB = 0 // BI_RGB
	dibRGBColors = 0

	// AlphaBlend
	acSrcOver  = 0x00
	acSrcAlpha = 0x01

	// Shell_NotifyIcon
	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	// LoadImage
	imageIcon    = 1
	lrLoadFromFile = 0x00000010
	lrDefaultSize  = 0x00000040

	// Menu
	mfString  = 0x00000000
	tpmLeftAlign = 0x0000
	tpmRightButton = 0x0002

	// System metrics
	smCxScreen = 0
	smCyScreen = 1

	// Cursors
	idcArrow = 32512

	// ShowWindow
	swShowNoActivate = 8

	// Timer IDs
	timerIDTick    = 1
	timerIDTopmost = 2
)

// Custom messages
const (
	wmTrayIcon   = wmApp + 1
	wmGameUpdate = wmApp + 2
)

// Tray menu command IDs
const (
	idTrayQuit = 1001
)

// ---------------------------------------------------------------------------
// Structures
// ---------------------------------------------------------------------------

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

type msg struct {
	Hwnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type point struct {
	X, Y int32
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type paintStruct struct {
	Hdc         syscall.Handle
	FErase      int32
	RcPaint     rect
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type bitmapInfo struct {
	BmiHeader bitmapInfoHeader
	BmiColors [1]uint32
}

type blendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type notifyIconDataW struct {
	CbSize           uint32
	HWnd             syscall.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            syscall.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     syscall.Handle
}

// ---------------------------------------------------------------------------
// Helper: COLORREF from R,G,B
// ---------------------------------------------------------------------------

func rgb(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func hexToRGB(hex uint32) (r, g, b byte) {
	r = byte((hex >> 16) & 0xFF)
	g = byte((hex >> 8) & 0xFF)
	b = byte(hex & 0xFF)
	return
}

func hexToCOLORREF(hex uint32) uint32 {
	r, g, b := hexToRGB(hex)
	return rgb(r, g, b)
}

// ---------------------------------------------------------------------------
// Thin wrappers
// ---------------------------------------------------------------------------

func registerClassEx(wc *wndClassExW) (uint16, error) {
	ret, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(wc)))
	if ret == 0 {
		return 0, err
	}
	return uint16(ret), nil
}

func createWindowEx(exStyle uint32, className, windowName *uint16, style uint32, x, y, w, h int32, parent, menu, instance syscall.Handle, param unsafe.Pointer) (syscall.Handle, error) {
	ret, _, err := pCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(menu), uintptr(instance),
		uintptr(param),
	)
	if ret == 0 {
		return 0, err
	}
	return syscall.Handle(ret), nil
}

func defWindowProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func getMessage(m *msg) (bool, error) {
	ret, _, err := pGetMessageW.Call(uintptr(unsafe.Pointer(m)), 0, 0, 0)
	switch int32(ret) {
	case -1:
		return false, err
	case 0:
		return false, nil
	default:
		return true, nil
	}
}

func translateMessage(m *msg) {
	pTranslateMessage.Call(uintptr(unsafe.Pointer(m)))
}

func dispatchMessage(m *msg) {
	pDispatchMessageW.Call(uintptr(unsafe.Pointer(m)))
}

func showWindow(hwnd syscall.Handle, cmdShow int32) {
	pShowWindow.Call(uintptr(hwnd), uintptr(cmdShow))
}

func destroyWindow(hwnd syscall.Handle) {
	pDestroyWindow.Call(uintptr(hwnd))
}

func postQuitMessage(exitCode int32) {
	pPostQuitMessage.Call(uintptr(exitCode))
}

func postMessage(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) {
	pPostMessageW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
}

func setWindowPos(hwnd syscall.Handle, insertAfter uintptr, x, y, cx, cy int32, flags uint32) {
	pSetWindowPos.Call(uintptr(hwnd), insertAfter, uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), uintptr(flags))
}

func setLayeredWindowAttributes(hwnd syscall.Handle, crKey uint32, alpha byte, flags uint32) {
	pSetLayeredWindowAttributes.Call(uintptr(hwnd), uintptr(crKey), uintptr(alpha), uintptr(flags))
}

func setWindowLongPtr(hwnd syscall.Handle, index int32, newLong uintptr) uintptr {
	ret, _, _ := pSetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index), newLong)
	return ret
}

func getWindowLongPtr(hwnd syscall.Handle, index int32) uintptr {
	ret, _, _ := pGetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index))
	return ret
}

func beginPaint(hwnd syscall.Handle, ps *paintStruct) syscall.Handle {
	ret, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(ps)))
	return syscall.Handle(ret)
}

func endPaint(hwnd syscall.Handle, ps *paintStruct) {
	pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(ps)))
}

func invalidateRect(hwnd syscall.Handle, r *rect, erase bool) {
	var e uintptr
	if erase {
		e = 1
	}
	pInvalidateRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(r)), e)
}

func getDC(hwnd syscall.Handle) syscall.Handle {
	ret, _, _ := pGetDC.Call(uintptr(hwnd))
	return syscall.Handle(ret)
}

func releaseDC(hwnd syscall.Handle, hdc syscall.Handle) {
	pReleaseDC.Call(uintptr(hwnd), uintptr(hdc))
}

func setCapture(hwnd syscall.Handle) {
	pSetCapture.Call(uintptr(hwnd))
}

func releaseCapture() {
	pReleaseCapture.Call()
}

func getCursorPos(pt *point) {
	pGetCursorPos.Call(uintptr(unsafe.Pointer(pt)))
}

func setTimer(hwnd syscall.Handle, id uintptr, elapse uint32) {
	pSetTimer.Call(uintptr(hwnd), id, uintptr(elapse), 0)
}

func killTimer(hwnd syscall.Handle, id uintptr) {
	pKillTimer.Call(uintptr(hwnd), id)
}

func moveWindow(hwnd syscall.Handle, x, y, w, h int32, repaint bool) {
	var r uintptr
	if repaint {
		r = 1
	}
	pMoveWindow.Call(uintptr(hwnd), uintptr(x), uintptr(y), uintptr(w), uintptr(h), r)
}

func getWindowRect(hwnd syscall.Handle, r *rect) {
	pGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(r)))
}

func getClientRect(hwnd syscall.Handle, r *rect) {
	pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(r)))
}

func fillRect(hdc syscall.Handle, r *rect, brush syscall.Handle) {
	pFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(r)), uintptr(brush))
}

func getSystemMetrics(index int32) int32 {
	ret, _, _ := pGetSystemMetrics.Call(uintptr(index))
	return int32(ret)
}

func loadImage(hInst syscall.Handle, name *uint16, typ uint32, cx, cy int32, flags uint32) syscall.Handle {
	ret, _, _ := pLoadImageW.Call(uintptr(hInst), uintptr(unsafe.Pointer(name)), uintptr(typ), uintptr(cx), uintptr(cy), uintptr(flags))
	return syscall.Handle(ret)
}

// --- GDI ---

func createCompatibleDC(hdc syscall.Handle) syscall.Handle {
	ret, _, _ := pCreateCompatibleDC.Call(uintptr(hdc))
	return syscall.Handle(ret)
}

func createCompatibleBitmap(hdc syscall.Handle, w, h int32) syscall.Handle {
	ret, _, _ := pCreateCompatibleBitmap.Call(uintptr(hdc), uintptr(w), uintptr(h))
	return syscall.Handle(ret)
}

func deleteDC(hdc syscall.Handle) {
	pDeleteDC.Call(uintptr(hdc))
}

func selectObject(hdc syscall.Handle, obj syscall.Handle) syscall.Handle {
	ret, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(obj))
	return syscall.Handle(ret)
}

func deleteObject(obj syscall.Handle) {
	pDeleteObject.Call(uintptr(obj))
}

func createDIBSection(hdc syscall.Handle, bi *bitmapInfo, usage uint32, bits *unsafe.Pointer, section syscall.Handle, offset uint32) syscall.Handle {
	ret, _, _ := pCreateDIBSection.Call(
		uintptr(hdc),
		uintptr(unsafe.Pointer(bi)),
		uintptr(usage),
		uintptr(unsafe.Pointer(bits)),
		uintptr(section), uintptr(offset),
	)
	return syscall.Handle(ret)
}

func createSolidBrush(color uint32) syscall.Handle {
	ret, _, _ := pCreateSolidBrush.Call(uintptr(color))
	return syscall.Handle(ret)
}

func createFont(height, width, escapement, orientation, weight int32, italic, underline, strikeOut, charSet, outputPrecision, clipPrecision, quality, pitchAndFamily uint32, faceName *uint16) syscall.Handle {
	ret, _, _ := pCreateFontW.Call(
		uintptr(height), uintptr(width), uintptr(escapement), uintptr(orientation), uintptr(weight),
		uintptr(italic), uintptr(underline), uintptr(strikeOut),
		uintptr(charSet), uintptr(outputPrecision), uintptr(clipPrecision), uintptr(quality), uintptr(pitchAndFamily),
		uintptr(unsafe.Pointer(faceName)),
	)
	return syscall.Handle(ret)
}

func setBkMode(hdc syscall.Handle, mode int32) {
	pSetBkMode.Call(uintptr(hdc), uintptr(mode))
}

func setTextColor(hdc syscall.Handle, color uint32) {
	pSetTextColor.Call(uintptr(hdc), uintptr(color))
}

func textOut(hdc syscall.Handle, x, y int32, text *uint16, length int32) {
	pTextOutW.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(unsafe.Pointer(text)), uintptr(length))
}

func getTextExtentPoint32(hdc syscall.Handle, text *uint16, length int32, size *point) {
	pGetTextExtentPoint32W.Call(uintptr(hdc), uintptr(unsafe.Pointer(text)), uintptr(length), uintptr(unsafe.Pointer(size)))
}

func bitBlt(hdcDest syscall.Handle, xDest, yDest, w, h int32, hdcSrc syscall.Handle, xSrc, ySrc int32, rop uint32) {
	pBitBlt.Call(
		uintptr(hdcDest), uintptr(xDest), uintptr(yDest), uintptr(w), uintptr(h),
		uintptr(hdcSrc), uintptr(xSrc), uintptr(ySrc), uintptr(rop),
	)
}

func alphaBlend(hdcDest syscall.Handle, xDest, yDest, wDest, hDest int32, hdcSrc syscall.Handle, xSrc, ySrc, wSrc, hSrc int32, ftn blendFunction) {
	bf := *(*uint32)(unsafe.Pointer(&ftn))
	pAlphaBlend.Call(
		uintptr(hdcDest), uintptr(xDest), uintptr(yDest), uintptr(wDest), uintptr(hDest),
		uintptr(hdcSrc), uintptr(xSrc), uintptr(ySrc), uintptr(wSrc), uintptr(hSrc),
		uintptr(bf),
	)
}

// --- Shell ---

func shellNotifyIcon(message uint32, data *notifyIconDataW) bool {
	ret, _, _ := pShellNotifyIconW.Call(uintptr(message), uintptr(unsafe.Pointer(data)))
	return ret != 0
}

func createPopupMenu() syscall.Handle {
	ret, _, _ := pCreatePopupMenu.Call()
	return syscall.Handle(ret)
}

func appendMenu(menu syscall.Handle, flags uint32, idNewItem uintptr, newItem *uint16) {
	pAppendMenuW.Call(uintptr(menu), uintptr(flags), idNewItem, uintptr(unsafe.Pointer(newItem)))
}

func trackPopupMenu(menu syscall.Handle, flags uint32, x, y int32, hwnd syscall.Handle) {
	pTrackPopupMenu.Call(uintptr(menu), uintptr(flags), uintptr(x), uintptr(y), 0, uintptr(hwnd), 0)
}

func destroyMenu(menu syscall.Handle) {
	pDestroyMenu.Call(uintptr(menu))
}

func setForegroundWindow(hwnd syscall.Handle) {
	pSetForegroundWindow.Call(uintptr(hwnd))
}

// ---------------------------------------------------------------------------
// UTF-16 helper
// ---------------------------------------------------------------------------

func mustUTF16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func getModuleHandle() syscall.Handle {
	ret, _, _ := pGetModuleHandleW.Call(0)
	return syscall.Handle(ret)
}

func loadCursor(instance syscall.Handle, cursorName uintptr) syscall.Handle {
	ret, _, _ := pLoadCursorW.Call(uintptr(instance), cursorName)
	return syscall.Handle(ret)
}
