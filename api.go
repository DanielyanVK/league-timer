//go:build windows

package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Enemy data
// ---------------------------------------------------------------------------

type enemy struct {
	Champ  string
	Spell1 string
	Spell2 string
	Haste  int
}

// ---------------------------------------------------------------------------
// HTTP client (skip TLS verification for the local LoL client)
// ---------------------------------------------------------------------------

var httpClient = &http.Client{
	Timeout: 500 * time.Millisecond,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

var httpClientLong = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

// ---------------------------------------------------------------------------
// Fetch live game data from the LoL client
// ---------------------------------------------------------------------------

func fetchGameData() (map[string]interface{}, error) {
	resp, err := httpClient.Get(lclURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Parse enemies from game data
// ---------------------------------------------------------------------------

func parseEnemies(data map[string]interface{}) []enemy {
	allPlayersRaw, _ := data["allPlayers"].([]interface{})
	activePlayer, _ := data["activePlayer"].(map[string]interface{})
	myName, _ := activePlayer["summonerName"].(string)

	// Find my team
	myTeam := ""
	if myName != "" {
		for _, pRaw := range allPlayersRaw {
			p, _ := pRaw.(map[string]interface{})
			if p["summonerName"] == myName {
				myTeam, _ = p["team"].(string)
				break
			}
		}
	}
	if myTeam == "" {
		if t, ok := activePlayer["team"].(string); ok {
			myTeam = t
		} else {
			myTeam = "ORDER"
		}
	}

	var enemies []enemy
	for _, pRaw := range allPlayersRaw {
		p, _ := pRaw.(map[string]interface{})
		if p == nil {
			continue
		}
		team, _ := p["team"].(string)
		if team == myTeam {
			continue
		}

		rawName, _ := p["rawChampionName"].(string)
		if rawName == "" {
			rawName, _ = p["championName"].(string)
		}
		champName := rawName
		if idx := strings.LastIndex(rawName, "_"); idx >= 0 {
			champName = rawName[idx+1:]
		}

		spells, _ := p["summonerSpells"].(map[string]interface{})

		// Calculate haste from items
		haste := 0
		if items, ok := p["items"].([]interface{}); ok {
			for _, itemRaw := range items {
				item, _ := itemRaw.(map[string]interface{})
				if item == nil {
					continue
				}
				// itemID can be float64 from JSON
				if idF, ok := item["itemID"].(float64); ok {
					if val, found := itemHasteMap[int(idF)]; found {
						haste += val
					}
				}
			}
		}

		enemies = append(enemies, enemy{
			Champ:  champName,
			Spell1: cleanSpellName(getSpellRawName(spells, "summonerSpellOne")),
			Spell2: cleanSpellName(getSpellRawName(spells, "summonerSpellTwo")),
			Haste:  haste,
		})
	}
	return enemies
}

func getSpellRawName(spells map[string]interface{}, key string) string {
	if spells == nil {
		return ""
	}
	spell, _ := spells[key].(map[string]interface{})
	if spell == nil {
		return ""
	}
	name, _ := spell["rawDisplayName"].(string)
	return name
}

func cleanSpellName(raw string) string {
	if raw == "" {
		return "Unknown"
	}
	low := strings.ToLower(raw)
	switch {
	case strings.Contains(low, "teleport"):
		return "SummonerTeleport"
	case strings.Contains(low, "smite"):
		return "SummonerSmite"
	case strings.Contains(low, "flash"):
		return "SummonerFlash"
	case strings.Contains(low, "ignite") || strings.Contains(low, "dot"):
		return "SummonerDot"
	case strings.Contains(low, "barrier"):
		return "SummonerBarrier"
	case strings.Contains(low, "heal"):
		return "SummonerHeal"
	case strings.Contains(low, "exhaust"):
		return "SummonerExhaust"
	case strings.Contains(low, "cleanse") || strings.Contains(low, "boost"):
		return "SummonerBoost"
	case strings.Contains(low, "ghost") || strings.Contains(low, "haste"):
		return "SummonerHaste"
	case strings.Contains(low, "snowball") || strings.Contains(low, "mark"):
		return "SummonerSnowball"
	case strings.Contains(low, "clarity"):
		return "SummonerClarity"
	}

	// Fallback: try to extract "Summoner..." from underscore-separated parts
	parts := strings.Split(raw, "_")
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasPrefix(parts[i], "Summoner") && parts[i] != "SummonerSpell" {
			return parts[i]
		}
	}
	return "Unknown"
}

// ---------------------------------------------------------------------------
// DDragon: update spell cooldowns from latest patch data
// ---------------------------------------------------------------------------

func updateTimersFromDDragon() {
	fmt.Println("[DDragon] Checking for updates...")

	resp, err := httpClientLong.Get(ddragonVerURL)
	if err != nil {
		fmt.Printf("[DDragon] Update failed: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}

	var versions []string
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil || len(versions) == 0 {
		return
	}
	version := versions[0]

	url := fmt.Sprintf(ddragonDataURL, version)
	resp2, err := httpClientLong.Get(url)
	if err != nil {
		fmt.Printf("[DDragon] Update failed: %v\n", err)
		return
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		return
	}

	var summonerData struct {
		Data map[string]struct {
			Cooldown []float64 `json:"cooldown"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&summonerData); err != nil {
		fmt.Printf("[DDragon] Parse failed: %v\n", err)
		return
	}

	spellTimersMu.Lock()
	defer spellTimersMu.Unlock()
	for spellID, info := range summonerData.Data {
		if len(info.Cooldown) > 0 {
			spellTimers[strings.ToLower(spellID)] = int(info.Cooldown[0])
		}
	}
	fmt.Printf("[DDragon] Updated timers for patch %s\n", version)
}
