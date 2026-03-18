// Asset downloader for LoL Spell Timer.
// Downloads champion and spell icons from DDragon CDN.
//
// Usage: go run ./cmd/download-assets
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	assetsDir   = "assets"
	champDir    = "assets/champions"
	spellDir    = "assets/spells"
	versionFile = "assets/version.txt"
)

var spellIDs = []string{
	"SummonerFlash", "SummonerDot", "SummonerHeal", "SummonerBarrier",
	"SummonerExhaust", "SummonerTeleport", "SummonerSmite", "SummonerBoost",
	"SummonerMana", "SummonerHaste", "SummonerSnowball",
}

var client = &http.Client{Timeout: 15 * time.Second}

func main() {
	os.MkdirAll(champDir, 0755)
	os.MkdirAll(spellDir, 0755)

	fmt.Println("--- LoL Asset Manager ---")

	latestVer, err := getLatestVersion()
	if err != nil {
		fmt.Printf("Could not connect to Riot API: %v\n", err)
		return
	}

	localVer := getLocalVersion()
	forceUpdate := false

	if localVer != latestVer {
		fmt.Printf(" [!] Update detected! Local: %s -> Latest: %s\n", localVer, latestVer)
		fmt.Println(" [i] Doing a full sync...")
		forceUpdate = true
	} else {
		fmt.Printf(" [OK] Version %s is up to date.\n", latestVer)
		fmt.Println(" [i] Checking for missing files only...")
	}

	// Download champions
	fmt.Println("\n--- Syncing Champions ---")
	champs, err := getChampionList(latestVer)
	if err != nil {
		fmt.Printf("Failed to get champion list: %v\n", err)
		return
	}
	for _, champ := range champs {
		url := fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/img/champion/%s.png", latestVer, champ)
		path := filepath.Join(champDir, champ+".png")
		downloadFile(url, path, forceUpdate)
	}

	// Download spells
	fmt.Println("\n--- Syncing Spells ---")
	for _, spell := range spellIDs {
		url := fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/img/spell/%s.png", latestVer, spell)
		path := filepath.Join(spellDir, spell+".png")
		downloadFile(url, path, forceUpdate)
	}

	// Generate icon
	fmt.Println("\n--- Generating Icon ---")
	generateICO()

	// Save version
	saveLocalVersion(latestVer)
	fmt.Printf("\n[Success] All assets synced for version %s!\n", latestVer)
}

func getLatestVersion() (string, error) {
	resp, err := client.Get("https://ddragon.leagueoflegends.com/api/versions.json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var versions []string
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("empty version list")
	}
	return versions[0], nil
}

func getLocalVersion() string {
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return ""
	}
	return string(data)
}

func saveLocalVersion(version string) {
	os.WriteFile(versionFile, []byte(version), 0644)
}

func getChampionList(version string) ([]string, error) {
	url := fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/data/en_US/champion.json", version)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	champs := make([]string, 0, len(data.Data))
	for name := range data.Data {
		champs = append(champs, name)
	}
	return champs, nil
}

// ---------------------------------------------------------------------------
// Generate ico/icon.ico from SummonerFlash.png (PNG-in-ICO format)
// ---------------------------------------------------------------------------

func generateICO() {
	srcPath := filepath.Join(spellDir, "SummonerFlash.png")
	dstPath := filepath.Join("ico", "icon.ico")

	if _, err := os.Stat(srcPath); err != nil {
		fmt.Println(" [!] SummonerFlash.png not found, skipping icon generation")
		return
	}

	os.MkdirAll("ico", 0755)

	// Load source PNG
	f, err := os.Open(srcPath)
	if err != nil {
		fmt.Printf(" [!] Could not open %s: %v\n", srcPath, err)
		return
	}
	srcImg, err := png.Decode(f)
	f.Close()
	if err != nil {
		fmt.Printf(" [!] Could not decode PNG: %v\n", err)
		return
	}

	// Create multiple sizes
	sizes := []int{16, 32, 48}
	var pngDatas [][]byte
	for _, sz := range sizes {
		resized := resizeNearestNeighbor(srcImg, sz, sz)
		var buf bytes.Buffer
		png.Encode(&buf, resized)
		pngDatas = append(pngDatas, buf.Bytes())
	}

	// Write ICO file
	out, err := os.Create(dstPath)
	if err != nil {
		fmt.Printf(" [!] Could not create %s: %v\n", dstPath, err)
		return
	}
	defer out.Close()

	n := uint16(len(sizes))
	// ICO header: reserved(2) + type(2) + count(2) = 6 bytes
	binary.Write(out, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(out, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(out, binary.LittleEndian, n)          // image count

	// Calculate offsets: header(6) + entries(n*16) + data
	offset := uint32(6 + n*16)
	for i, sz := range sizes {
		w := byte(sz)
		if sz == 256 {
			w = 0
		}
		out.Write([]byte{w})    // width
		out.Write([]byte{w})    // height
		out.Write([]byte{0})    // color count
		out.Write([]byte{0})    // reserved
		binary.Write(out, binary.LittleEndian, uint16(1))                // planes
		binary.Write(out, binary.LittleEndian, uint16(32))               // bit count
		binary.Write(out, binary.LittleEndian, uint32(len(pngDatas[i]))) // data size
		binary.Write(out, binary.LittleEndian, offset)                   // data offset
		offset += uint32(len(pngDatas[i]))
	}

	// Write PNG data for each size
	for _, data := range pngDatas {
		out.Write(data)
	}

	fmt.Printf(" [+] Generated: %s (sizes: %v)\n", dstPath, sizes)
}

func resizeNearestNeighbor(src image.Image, w, h int) *image.RGBA {
	bounds := src.Bounds()
	sw, sh := bounds.Dx(), bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if sw == w && sh == h {
		draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
		return dst
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := x * sw / w
			sy := y * sh / h
			dst.Set(x, y, src.At(sx+bounds.Min.X, sy+bounds.Min.Y))
		}
	}
	return dst
}

func downloadFile(url, path string, force bool) {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return // already exists
		}
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf(" [!] Error: %s: %v\n", filepath.Base(path), err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf(" [!] Failed (Status %d): %s\n", resp.StatusCode, filepath.Base(path))
		return
	}

	f, err := os.Create(path)
	if err != nil {
		fmt.Printf(" [!] Create error: %s: %v\n", filepath.Base(path), err)
		return
	}
	defer f.Close()

	io.Copy(f, resp.Body)
	fmt.Printf(" [+] Downloaded: %s\n", filepath.Base(path))
}
