// Package matching holds the market rules: the daily like budget, pass limits
// and the normalised persona pair a match is stored under.
package matching

import (
	"bytes"

	"github.com/google/uuid"
)

// DailyLikeBudget is the number of likes every persona gets per game day.
// Received-like replies consume the same budget; there is no separate quota.
const DailyLikeBudget = 10

// MaxPassCount is the pass count at which a target is excluded for the day.
const MaxPassCount = 3

// NormalizePair orders a persona pair so a match is stored exactly once,
// regardless of who liked first.
func NormalizePair(a, b uuid.UUID) (low, high uuid.UUID) {
	// Byte order matches PostgreSQL's own uuid ordering.
	if bytes.Compare(a[:], b[:]) < 0 {
		return a, b
	}
	return b, a
}

// RemainingLikes clamps the remaining budget to a non-negative value.
func RemainingLikes(sent int64) int {
	remaining := DailyLikeBudget - int(sent)
	if remaining < 0 {
		return 0
	}
	return remaining
}
