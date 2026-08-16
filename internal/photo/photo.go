// Package photo stores profile pictures as files on disk.
//
// Everything a client sends is decoded and re-encoded before it is stored.
// That single step is what makes the feature safe enough to ship: it drops EXIF
// (an anonymous app must not publish someone's GPS coordinates), discards any
// payload smuggled alongside the pixels, and normalises size and format.
// Client-side resizing only saves bandwidth; it is never trusted.
package photo

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG for decoding
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"golang.org/x/image/draw"

	"kusamachi/internal/apperr"
	"kusamachi/internal/clock"
)

const (
	// MaxUploadBytes caps the request body. The client resizes first, so a real
	// upload is a couple of hundred kilobytes.
	MaxUploadBytes = 8 << 20

	// MaxPixels rejects decompression bombs before the image is decoded.
	MaxPixels = 40_000_000

	// MaxEdge is the stored long edge.
	MaxEdge = 1024

	jpegQuality = 85

	// ContentType is what is stored and served, whatever came in.
	ContentType = "image/jpeg"
)

// Store writes one file per persona, in a directory per game date so the daily
// cleanup can drop a whole day by removing a directory.
type Store struct {
	dir string
}

// NewStore creates the root directory if it does not exist.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create photo dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) dayDir(gameDate time.Time) string {
	return filepath.Join(s.dir, clock.FormatGameDate(gameDate))
}

func (s *Store) path(gameDate time.Time, personaID uuid.UUID) string {
	return filepath.Join(s.dayDir(gameDate), personaID.String()+".jpg")
}

// Save normalises the uploaded image and writes it, replacing any previous one.
func (s *Store) Save(gameDate time.Time, personaID uuid.UUID, r io.Reader) error {
	encoded, err := normalize(r)
	if err != nil {
		return err
	}

	dir := s.dayDir(gameDate)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create photo day dir: %w", err)
	}

	// Write to a temporary file first so a reader never sees a half-written
	// picture.
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return fmt.Errorf("create temp photo: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(encoded); err != nil {
		tmp.Close()
		return fmt.Errorf("write photo: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close photo: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o640); err != nil {
		return fmt.Errorf("chmod photo: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path(gameDate, personaID)); err != nil {
		return fmt.Errorf("store photo: %w", err)
	}
	return nil
}

// Open returns the stored picture. The caller closes the file.
func (s *Store) Open(gameDate time.Time, personaID uuid.UUID) (*os.File, error) {
	f, err := os.Open(s.path(gameDate, personaID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, apperr.New(apperr.CodeTargetPersonaUnavailable, "no photo")
	}
	if err != nil {
		return nil, fmt.Errorf("open photo: %w", err)
	}
	return f, nil
}

// Delete removes the picture. Missing is not an error.
func (s *Store) Delete(gameDate time.Time, personaID uuid.UUID) error {
	if err := os.Remove(s.path(gameDate, personaID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete photo: %w", err)
	}
	return nil
}

// DeleteBefore removes every game-date directory older than today and reports
// how many days it dropped. Idempotent, like the rest of the cleanup.
func (s *Store) DeleteBefore(today time.Time) (int, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, fmt.Errorf("read photo dir: %w", err)
	}

	cutoff := clock.FormatGameDate(today)
	removed := 0
	for _, entry := range entries {
		// Directory names are YYYY-MM-DD, so a string compare is a date compare.
		if !entry.IsDir() || entry.Name() >= cutoff {
			continue
		}
		if err := os.RemoveAll(filepath.Join(s.dir, entry.Name())); err != nil {
			return removed, fmt.Errorf("remove photo day: %w", err)
		}
		removed++
	}
	return removed, nil
}

// normalize decodes the upload and re-encodes it as a bounded JPEG.
func normalize(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return nil, invalidImage("画像を読み取れませんでした")
	}
	if len(raw) > MaxUploadBytes {
		return nil, invalidImage("画像が大きすぎます")
	}
	if len(raw) == 0 {
		return nil, invalidImage("画像が空です")
	}

	// Read the header only, so a decompression bomb is rejected before it is
	// expanded into memory.
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, invalidImage("対応していない画像形式です")
	}
	if format != "jpeg" && format != "png" {
		return nil, invalidImage("JPEGかPNGを選んでください")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > MaxPixels {
		return nil, invalidImage("画像の画素数が大きすぎます")
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, invalidImage("画像を読み取れませんでした")
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, fitWithin(src, MaxEdge), &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encode photo: %w", err)
	}
	return buf.Bytes(), nil
}

// fitWithin scales the image down so its long edge is at most edge pixels.
// Images already small enough are returned untouched.
func fitWithin(src image.Image, edge int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= edge && h <= edge {
		return src
	}

	if w >= h {
		h = h * edge / w
		w = edge
	} else {
		w = w * edge / h
		h = edge
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func invalidImage(message string) error {
	return apperr.New(apperr.CodeInvalidProfileInput, message)
}
