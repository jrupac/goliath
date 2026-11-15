package fetch

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
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

func TestShrinkStoredFaviconsReencodesOnlyWhatIsNotNormalized(t *testing.T) {
	user := models.User{UserId: "u", Username: "u"}
	stored := map[models.FeedId]string{
		1: "image/png;base64," + base64.StdEncoding.EncodeToString(testIcon(t, 256)),
		2: "image/png;base64," + base64.StdEncoding.EncodeToString(testIcon(t, faviconSize)),
	}
	written := map[models.FeedId][]byte{}
	d := &storage.MockDB{
		OnGetAllUsers: func() ([]models.User, error) { return []models.User{user}, nil },
		OnGetAllFaviconsForUser: func(models.User) (map[models.FeedId]string, error) {
			return stored, nil
		},
		OnInsertFaviconForUser: func(_ models.User, id models.FeedId, mime string, favicon []byte) error {
			if mime != "image/png" {
				t.Errorf("feed %d stored as %s, want image/png", id, mime)
			}
			written[id] = favicon
			return nil
		},
	}

	ShrinkStoredFavicons(context.Background(), d)

	if len(written) != 1 || written[1] == nil {
		t.Fatalf("re-encoded feeds %v, want only feed 1", keys(written))
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(written[1]))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != faviconSize || cfg.Height != faviconSize {
		t.Errorf("re-encoded at %dx%d, want %dx%d", cfg.Width, cfg.Height, faviconSize, faviconSize)
	}
}

func keys(m map[models.FeedId][]byte) []models.FeedId {
	var ks []models.FeedId
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
