//go:build windows

package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"golang.org/x/sys/windows"
)

// ---------------------------------------------------------------------------
// Single-instance check via a named mutex
// ---------------------------------------------------------------------------

func isAlreadyRunning() bool {
	name, _ := syscall.UTF16PtrFromString("Global\\LoLSpellTimer_Go_v1")
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		// ERROR_ALREADY_EXISTS
		if err == windows.ERROR_ALREADY_EXISTS {
			return true
		}
	}
	// Keep handle alive for process lifetime (GC won't collect it since we
	// don't close it). If h is 0 something unexpected happened, but we still
	// allow the app to start.
	_ = h
	return false
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

func main() {
	// Win32 GUI must run on the main OS thread.
	runtime.LockOSThread()

	if isAlreadyRunning() {
		fmt.Println("[Spell Timer] Already running. Exiting.")
		os.Exit(0)
	}

	// Update spell cooldowns from DDragon (best-effort, non-blocking for UX)
	updateTimersFromDDragon()

	// Create and run the overlay (blocks on the Win32 message loop)
	o := newOverlay()
	o.run()
}
