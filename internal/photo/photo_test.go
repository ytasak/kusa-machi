package photo

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"kusamachi/internal/apperr"
	"kusamachi/internal/clock"
)

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 90, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return buf.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return buf.Bytes()
}

func decodeConfig(t *testing.T, b []byte) (image.Config, string) {
	t.Helper()
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return cfg, format
}

func TestNormalizeScalesDownTheLongEdge(t *testing.T) {
	out, err := normalize(bytes.NewReader(jpegBytes(t, 2400, 1200)))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	cfg, format := decodeConfig(t, out)
	if format != "jpeg" {
		t.Fatalf("format = %s, want jpeg", format)
	}
	if cfg.Width != MaxEdge {
		t.Fatalf("width = %d, want %d", cfg.Width, MaxEdge)
	}
	if cfg.Height != MaxEdge/2 {
		t.Fatalf("height = %d, want %d (aspect ratio must be kept)", cfg.Height, MaxEdge/2)
	}
}

func TestNormalizeLeavesSmallImagesAlone(t *testing.T) {
	out, err := normalize(bytes.NewReader(jpegBytes(t, 300, 200)))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	cfg, _ := decodeConfig(t, out)
	if cfg.Width != 300 || cfg.Height != 200 {
		t.Fatalf("size = %dx%d, want 300x200", cfg.Width, cfg.Height)
	}
}

func TestNormalizeConvertsPNGToJPEG(t *testing.T) {
	out, err := normalize(bytes.NewReader(pngBytes(t, 400, 400)))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if _, format := decodeConfig(t, out); format != "jpeg" {
		t.Fatalf("format = %s, want jpeg", format)
	}
}

func TestNormalizeStripsMetadata(t *testing.T) {
	// A JPEG carrying an EXIF-looking APP1 segment must come out without it:
	// an anonymous app publishing someone's GPS coordinates would be a leak.
	original := jpegBytes(t, 200, 200)
	exif := []byte("Exif\x00\x00SECRET-GPS-PAYLOAD")
	app1 := append([]byte{0xFF, 0xE1, byte((len(exif) + 2) >> 8), byte((len(exif) + 2) & 0xFF)}, exif...)
	tampered := append(append(append([]byte{}, original[:2]...), app1...), original[2:]...)

	out, err := normalize(bytes.NewReader(tampered))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if bytes.Contains(out, []byte("SECRET-GPS-PAYLOAD")) {
		t.Fatal("metadata survived the re-encode")
	}
}

func TestNormalizeRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{"empty", nil},
		{"not an image", []byte("<html><script>alert(1)</script></html>")},
		{"gif is not accepted", []byte("GIF89a\x01\x00\x01\x00\x00\xff\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x00;")},
		{"too many bytes", bytes.Repeat([]byte{0xFF}, MaxUploadBytes+1)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalize(bytes.NewReader(tc.body))

			var domainErr *apperr.Error
			if !errors.As(err, &domainErr) || domainErr.Code != apperr.CodeInvalidProfileInput {
				t.Fatalf("err = %v, want InvalidProfileInput", err)
			}
		})
	}
}

func TestNormalizeRejectsDecompressionBombsBeforeDecoding(t *testing.T) {
	// A PNG header claiming a huge canvas compresses to almost nothing, so the
	// size cap alone would not catch it. The pixel count check must.
	huge := pngBytes(t, 12000, 12000)
	if len(huge) > MaxUploadBytes {
		t.Fatalf("fixture is %d bytes, expected the size cap not to be what rejects it", len(huge))
	}

	_, err := normalize(bytes.NewReader(huge))
	var domainErr *apperr.Error
	if !errors.As(err, &domainErr) || domainErr.Code != apperr.CodeInvalidProfileInput {
		t.Fatalf("err = %v, want InvalidProfileInput", err)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	day := time.Date(2026, 8, 16, 0, 0, 0, 0, clock.JST)
	id := uuid.New()

	if err := store.Save(day, id, bytes.NewReader(jpegBytes(t, 600, 600))); err != nil {
		t.Fatalf("save: %v", err)
	}

	f, err := store.Open(day, id)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	f.Close()

	if err := store.Delete(day, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Open(day, id); err == nil {
		t.Fatal("photo is still readable after delete")
	}
	// Deleting twice is not an error.
	if err := store.Delete(day, id); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestDeleteBeforeDropsOnlyPreviousDays(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	yesterday := time.Date(2026, 8, 15, 0, 0, 0, 0, clock.JST)
	today := time.Date(2026, 8, 16, 0, 0, 0, 0, clock.JST)
	oldID, newID := uuid.New(), uuid.New()

	if err := store.Save(yesterday, oldID, bytes.NewReader(jpegBytes(t, 100, 100))); err != nil {
		t.Fatalf("save yesterday: %v", err)
	}
	if err := store.Save(today, newID, bytes.NewReader(jpegBytes(t, 100, 100))); err != nil {
		t.Fatalf("save today: %v", err)
	}

	removed, err := store.DeleteBefore(today)
	if err != nil {
		t.Fatalf("delete before: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed %d day directories, want 1", removed)
	}

	if _, err := os.Stat(filepath.Join(dir, "2026-08-15")); !os.IsNotExist(err) {
		t.Fatal("yesterday's directory survived")
	}
	if _, err := store.Open(today, newID); err != nil {
		t.Fatalf("today's photo was removed: %v", err)
	}

	// Idempotent.
	if removed, err := store.DeleteBefore(today); err != nil || removed != 0 {
		t.Fatalf("second sweep removed %d (err %v), want 0", removed, err)
	}
}
