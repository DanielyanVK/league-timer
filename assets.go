//go:build windows

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	_ "image/jpeg"
)

// ---------------------------------------------------------------------------
// Bitmap handle that holds an HBITMAP and its dimensions.
// ---------------------------------------------------------------------------

type bitmap struct {
	hbm    syscall.Handle
	width  int32
	height int32
}

// ---------------------------------------------------------------------------
// Asset cache
// ---------------------------------------------------------------------------

var bitmapCache = map[string]*bitmap{}

func getCachedBitmap(key string) *bitmap {
	return bitmapCache[key]
}

// ---------------------------------------------------------------------------
// Resource path helper (equivalent to Python resource_path)
// ---------------------------------------------------------------------------

func resourcePath(rel string) string {
	exe, err := os.Executable()
	if err != nil {
		return rel
	}
	return filepath.Join(filepath.Dir(exe), rel)
}

// ---------------------------------------------------------------------------
// Load a spell or champion icon as a premultiplied-alpha DIB (HBITMAP).
// folder: "spells" or "champions"
// name:   e.g. "SummonerFlash" or "Darius"
// size:   target pixel size (square)
// round:  whether to apply circular mask (for champion icons)
// ---------------------------------------------------------------------------

func loadIcon(folder, name string, size int, round bool) *bitmap {
	key := fmt.Sprintf("%s/%s/%d/%v", folder, name, size, round)
	if bm := getCachedBitmap(key); bm != nil {
		return bm
	}

	path := resourcePath(filepath.Join("assets", folder, name+".png"))
	img := loadPNG(path, size)
	if img == nil {
		img = placeholderImage(name, size)
	}

	// Resize to target size using nearest-neighbour (simple, fast).
	img = resizeRGBA(img, size, size)

	if round {
		img = circularMask(img)
	}

	hbm := rgbaToDIB(img)
	bm := &bitmap{hbm: hbm, width: int32(size), height: int32(size)}
	bitmapCache[key] = bm
	return bm
}

// loadDimLayer creates a semi-transparent black overlay for dimming spell icons.
func loadDimLayer(size int) *bitmap {
	key := fmt.Sprintf("__dim/%d", size)
	if bm := getCachedBitmap(key); bm != nil {
		return bm
	}

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	c := color.RGBA{R: 0, G: 0, B: 0, A: 180}
	draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)

	hbm := rgbaToDIB(img)
	bm := &bitmap{hbm: hbm, width: int32(size), height: int32(size)}
	bitmapCache[key] = bm
	return bm
}

// ---------------------------------------------------------------------------
// PNG loading
// ---------------------------------------------------------------------------

func loadPNG(path string, size int) *image.RGBA {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}

	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	return rgba
}

// ---------------------------------------------------------------------------
// Placeholder for missing assets
// ---------------------------------------------------------------------------

func placeholderImage(name string, size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	// Dark gray background
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{R: 34, G: 34, B: 34, A: 255}}, image.Point{}, draw.Src)
	// Draw a border
	for x := 0; x < size; x++ {
		img.Set(x, 0, color.RGBA{R: 85, G: 85, B: 85, A: 255})
		img.Set(x, size-1, color.RGBA{R: 85, G: 85, B: 85, A: 255})
	}
	for y := 0; y < size; y++ {
		img.Set(0, y, color.RGBA{R: 85, G: 85, B: 85, A: 255})
		img.Set(size-1, y, color.RGBA{R: 85, G: 85, B: 85, A: 255})
	}
	return img
}

// ---------------------------------------------------------------------------
// Simple nearest-neighbour resize
// ---------------------------------------------------------------------------

func resizeRGBA(src *image.RGBA, w, h int) *image.RGBA {
	srcBounds := src.Bounds()
	sw, sh := srcBounds.Dx(), srcBounds.Dy()
	if sw == w && sh == h {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := y * sh / h
		for x := 0; x < w; x++ {
			sx := x * sw / w
			dst.Set(x, y, src.At(sx+srcBounds.Min.X, sy+srcBounds.Min.Y))
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// Circular mask for champion icons
// ---------------------------------------------------------------------------

func circularMask(src *image.RGBA) *image.RGBA {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	cx, cy := float64(w)/2, float64(h)/2
	r := math.Min(cx, cy)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx + 0.5
			dy := float64(y) - cy + 0.5
			if dx*dx+dy*dy <= r*r {
				dst.Set(x, y, src.At(x+bounds.Min.X, y+bounds.Min.Y))
			}
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// Convert Go RGBA image → premultiplied-alpha DIB section (HBITMAP).
// This is required for correct AlphaBlend rendering.
// ---------------------------------------------------------------------------

func rgbaToDIB(img *image.RGBA) syscall.Handle {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	bi := bitmapInfo{
		BmiHeader: bitmapInfoHeader{
			BiSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
			BiWidth:       int32(w),
			BiHeight:      -int32(h), // top-down
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: biRGB,
		},
	}

	var bits unsafe.Pointer
	hbm := createDIBSection(0, &bi, dibRGBColors, &bits, 0, 0)
	if hbm == 0 {
		return 0
	}

	// Copy pixels in BGRA order, premultiplied alpha.
	pixels := unsafe.Slice((*byte)(bits), w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.RGBAAt(x+bounds.Min.X, y+bounds.Min.Y)
			idx := (y*w + x) * 4
			a := float64(c.A) / 255.0
			pixels[idx+0] = byte(float64(c.B) * a) // B
			pixels[idx+1] = byte(float64(c.G) * a) // G
			pixels[idx+2] = byte(float64(c.R) * a) // R
			pixels[idx+3] = c.A                     // A
		}
	}

	return hbm
}

// ---------------------------------------------------------------------------
// Create HICON from a PNG file (for tray icon fallback)
// ---------------------------------------------------------------------------

func createHIconFromPNG(pngPath string, size int) syscall.Handle {
	img := loadPNG(pngPath, size)
	if img == nil {
		return 0
	}
	img = resizeRGBA(img, size, size)

	// Create color bitmap (32-bit ARGB, premultiplied)
	colorBM := rgbaToDIB(img)
	if colorBM == 0 {
		return 0
	}

	// Create monochrome mask bitmap (all zeros = fully opaque)
	maskBM := createBitmap(int32(size), int32(size), 1, 1, nil)
	if maskBM == 0 {
		deleteObject(colorBM)
		return 0
	}

	info := iconInfo{
		FIcon:   1, // TRUE = icon (not cursor)
		HbmMask: maskBM,
		HbmColor: colorBM,
	}
	hIcon := createIconIndirect(&info)

	// CreateIconIndirect copies the bitmaps, so we can delete ours
	deleteObject(maskBM)
	deleteObject(colorBM)

	return hIcon
}

// ---------------------------------------------------------------------------
// Cleanup all cached bitmaps
// ---------------------------------------------------------------------------

func cleanupBitmaps() {
	for _, bm := range bitmapCache {
		if bm.hbm != 0 {
			deleteObject(bm.hbm)
		}
	}
	bitmapCache = map[string]*bitmap{}
}
