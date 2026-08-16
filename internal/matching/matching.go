// Package matching は市場のルールを持つ。1日の Like 予算、Pass の上限、
// および Match を保存する際の正規化済み Persona ペアを扱う。
package matching

import (
	"bytes"

	"github.com/google/uuid"
)

// DailyLikeBudget は各 Persona がゲーム日ごとに得る Like の数。
// Like へのお返しも同じ予算を消費し、別枠は存在しない。
const DailyLikeBudget = 10

// MaxPassCount は、相手がその日の表示対象から外れる Pass 回数。
const MaxPassCount = 3

// NormalizePair は Persona のペアを並べ替える。どちらが先に Like しても
// Match がちょうど1件だけ保存されるようにするため。
func NormalizePair(a, b uuid.UUID) (low, high uuid.UUID) {
	// バイト順は PostgreSQL の uuid の並び順と一致する。
	if bytes.Compare(a[:], b[:]) < 0 {
		return a, b
	}
	return b, a
}

// RemainingLikes は残り予算を負にならないよう丸める。
func RemainingLikes(sent int64) int {
	remaining := DailyLikeBudget - int(sent)
	if remaining < 0 {
		return 0
	}
	return remaining
}
