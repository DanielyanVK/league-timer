//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ---------------------------------------------------------------------------
// Visual constants (matching the Python original)
// ---------------------------------------------------------------------------

const (
	globalOpacity = 0.85
	iconSize      = 38
	rowPaddingY   = 6
	showSeparator = true
	borderWidth   = 1
	innerPadding  = 4
	handleHeight  = 16
	handleGap     = 2
	separatorH    = 1
	separatorPadY = 2
	spellGap      = 6
	champSpellGap = 8

	colorBG          = 0x091428
	colorBorder      = 0x463714
	colorSeparator   = 0x000000
	colorTextActive  = 0xFFFFFF
	colorTextOutline = 0x000000
	colorPinned      = 0x8B0000
	colorHandle      = 0x666666

	fontFamily   = "Arial"
	baseFontSize = 14

	checkIntervalMs = 2000
)

// ---------------------------------------------------------------------------
// Item haste database
// ---------------------------------------------------------------------------

// itemHasteMap maps item ID → summoner spell haste value.
var itemHasteMap = map[int]int{
	3158:   10, // Ionian Boots of Lucidity
	3171:   20, // Crimson Lucidity (Ornn Upgrade)
	223158: 10, // Ionian Boots (Arena)
}

// ---------------------------------------------------------------------------
// Spell timers (mutable – updated from DDragon at startup)
// ---------------------------------------------------------------------------

var (
	spellTimersMu sync.RWMutex
	spellTimers   = map[string]int{
		"summonerflash":    300,
		"summonerteleport": 360,
		"summonerheal":     240,
		"summonerboost":    210, // Cleanse
		"summonerexhaust":  210,
		"summonerhaste":    210, // Ghost
		"summonerbarrier":  180,
		"summonerdot":      180, // Ignite
		"summonersmite":    15,
		"summonersnowball": 80, // ARAM Mark
		"summonerclarity":  240,
		"summonermana":     240,
	}
)

func getBaseCD(spellName string) int {
	spellTimersMu.RLock()
	defer spellTimersMu.RUnlock()
	if cd, ok := spellTimers[spellName]; ok {
		return cd
	}
	return 300
}

// ---------------------------------------------------------------------------
// API URLs
// ---------------------------------------------------------------------------

const (
	lclURL        = "https://127.0.0.1:2999/liveclientdata/allgamedata"
	ddragonVerURL = "https://ddragon.leagueoflegends.com/api/versions.json"
	ddragonDataURL = "https://ddragon.leagueoflegends.com/cdn/%s/data/en_US/summoner.json"
)

// ---------------------------------------------------------------------------
// Persistent config (position & pin state)
// ---------------------------------------------------------------------------

type persistedConfig struct {
	X      int  `json:"x"`
	Y      int  `json:"y"`
	Pinned bool `json:"pinned"`
}

func configFilePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}

func loadConfig() persistedConfig {
	cfg := persistedConfig{
		X: int(getSystemMetrics(smCxScreen)) - 250,
		Y: 100,
	}
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg
	}
	fmt.Printf("[Config] Loaded: Pos(%d,%d), Pinned(%v)\n", cfg.X, cfg.Y, cfg.Pinned)
	return cfg
}

func saveConfig(cfg persistedConfig) {
	data, err := json.Marshal(cfg)
	if err != nil {
		fmt.Printf("[Config] Save error: %v\n", err)
		return
	}
	if err := os.WriteFile(configFilePath(), data, 0644); err != nil {
		fmt.Printf("[Config] Save error: %v\n", err)
		return
	}
	fmt.Println("[Config] Settings saved.")
}

// ---------------------------------------------------------------------------
// Layout helpers
// ---------------------------------------------------------------------------

// rowWidth returns the pixel width of one enemy row.
func rowWidth() int32 {
	return int32(iconSize + champSpellGap + iconSize + spellGap + iconSize)
}

// windowWidth returns total overlay width including border & padding.
func windowWidth() int32 {
	return int32(borderWidth)*2 + int32(innerPadding)*2 + rowWidth()
}

// windowHeight computes total overlay height for n enemies.
func windowHeight(n int) int32 {
	if n == 0 {
		n = 1
	}
	h := int32(borderWidth)*2 + int32(innerPadding)*2 + int32(handleHeight) + int32(handleGap)
	h += int32(n) * int32(iconSize+rowPaddingY)
	h -= int32(rowPaddingY) // last row has no bottom padding
	if showSeparator && n > 1 {
		h += int32(n-1) * int32(separatorH+separatorPadY*2)
	}
	return h
}
