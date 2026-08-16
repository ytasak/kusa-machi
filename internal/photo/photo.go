// Package photo はプロフィール写真をディスク上のファイルとして保存する。
//
// クライアントから届いたものは、保存前に必ずデコードして再エンコードする。
// この一手間がこの機能を出荷可能な安全性にしている。EXIF が落ち（匿名アプリが
// 誰かの GPS 座標を公開してはならない）、画素に紛れ込ませたペイロードが捨てられ、
// サイズと形式が正規化される。クライアント側のリサイズは通信量を減らすだけで、
// 信用の対象ではない。
package photo

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // デコード用に PNG を登録する
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
	// MaxUploadBytes はリクエストボディの上限。クライアントが先に縮小するため、
	// 実際のアップロードは数百キロバイト程度になる。
	MaxUploadBytes = 8 << 20

	// MaxPixels は展開爆弾をデコード前に弾くための画素数上限。
	MaxPixels = 40_000_000

	// MaxEdge は保存する画像の長辺。
	MaxEdge = 1024

	jpegQuality = 85

	// ContentType は入力が何であれ、保存・配信する形式。
	ContentType = "image/jpeg"
)

// Store は Persona ごとに1ファイルを書き出す。ゲーム日ごとのディレクトリに
// 分けてあるため、日次クリーンアップはディレクトリ削除だけで1日分を消せる。
type Store struct {
	dir string
}

// NewStore はルートディレクトリが無ければ作成する。
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

// Save はアップロードされた画像を正規化して書き込む。既存の写真は置き換わる。
func (s *Store) Save(gameDate time.Time, personaID uuid.UUID, r io.Reader) error {
	encoded, err := normalize(r)
	if err != nil {
		return err
	}

	dir := s.dayDir(gameDate)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create photo day dir: %w", err)
	}

	// 先に一時ファイルへ書く。読み手が書きかけの画像を目にしないようにするため。
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

// Open は保存済みの写真を返す。ファイルのクローズは呼び出し側の責任。
func (s *Store) Open(gameDate time.Time, personaID uuid.UUID) (*os.File, error) {
	f, err := os.Open(s.path(gameDate, personaID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, apperr.New(apperr.CodeTargetPersonaUnavailable, "写真がありません")
	}
	if err != nil {
		return nil, fmt.Errorf("open photo: %w", err)
	}
	return f, nil
}

// Delete は写真を削除する。存在しなくてもエラーにしない。
func (s *Store) Delete(gameDate time.Time, personaID uuid.UUID) error {
	if err := os.Remove(s.path(gameDate, personaID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete photo: %w", err)
	}
	return nil
}

// DeleteBefore は今日より古いゲーム日のディレクトリをすべて削除し、消した
// 日数を返す。クリーンアップの他の処理と同じく冪等。
func (s *Store) DeleteBefore(today time.Time) (int, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, fmt.Errorf("read photo dir: %w", err)
	}

	cutoff := clock.FormatGameDate(today)
	removed := 0
	for _, entry := range entries {
		// ディレクトリ名は YYYY-MM-DD なので、文字列比較がそのまま日付比較になる。
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

// normalize はアップロードをデコードし、上限内の JPEG として再エンコードする。
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

	// まずヘッダだけを読む。展開爆弾をメモリ上に展開する前に弾くため。
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

// fitWithin は長辺が edge ピクセル以下になるよう画像を縮小する。
// すでに十分小さい画像はそのまま返す。
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
