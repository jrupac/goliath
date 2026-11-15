package fetch

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func testIcon(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestNormalizeFaviconIsASmallDeterministicPNG(t *testing.T) {
	src, _, err := image.Decode(bytes.NewReader(testIcon(t, 256)))
	if err != nil {
		t.Fatal(err)
	}
	a, err := normalizeFavicon(src)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := normalizeFavicon(src)

	cfg, format, err := image.DecodeConfig(bytes.NewReader(a))
	if err != nil {
		t.Fatal(err)
	}
	if format != "png" || cfg.Width != faviconSize || cfg.Height != faviconSize {
		t.Errorf("normalized to a %dx%d %s, want a %dx%d png", cfg.Width, cfg.Height, format, faviconSize, faviconSize)
	}
	// Feeds share a favicon URL only if the same image normalizes to the same
	// bytes every time.
	if !bytes.Equal(a, b) {
		t.Error("normalizing the same image twice gave different bytes")
	}
}
