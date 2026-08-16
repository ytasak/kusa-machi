// Package clock provides the game-day time abstraction.
//
// Every game rule in this app is scoped to a JST calendar day, so all
// date logic must go through this package rather than time.Now().
package clock

import (
	"sync"
	"time"
)

// JST is the single timezone the game is played in (Asia/Tokyo).
var JST = mustJST()

func mustJST() *time.Location {
	// The tzdata database is not guaranteed to exist in a scratch container,
	// so fall back to the fixed +09:00 offset Japan has used since 1951.
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		return loc
	}
	return time.FixedZone("JST", 9*60*60)
}

// Clock is the injectable source of time. Production uses Real; tests use Fake.
type Clock interface {
	Now() time.Time
}

// Real reads the system clock.
type Real struct{}

func (Real) Now() time.Time { return time.Now() }

// Fake is a manually controlled Clock for tests. Safe for concurrent use.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake creates a Fake clock pinned to t.
func NewFake(t time.Time) *Fake { return &Fake{now: t} }

// NewFakeJST creates a Fake clock pinned to the given JST wall-clock time.
func NewFakeJST(year int, month time.Month, day, hour, min, sec int) *Fake {
	return NewFake(time.Date(year, month, day, hour, min, sec, 0, JST))
}

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Set replaces the current time.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}

// Advance moves the clock forward by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// GameDate returns the JST calendar day that t belongs to, as midnight JST.
// This is the value stored in participants.game_date.
func GameDate(t time.Time) time.Time {
	jt := t.In(JST)
	return time.Date(jt.Year(), jt.Month(), jt.Day(), 0, 0, 0, 0, JST)
}

// Today is GameDate of the clock's current time.
func Today(c Clock) time.Time { return GameDate(c.Now()) }

// DayEnd returns the instant the game day containing t ends, i.e. the next
// 00:00 JST. A time exactly at 00:00 JST belongs to the day that is starting,
// so its DayEnd is 24h later.
func DayEnd(t time.Time) time.Time { return GameDate(t).AddDate(0, 0, 1) }

// Remaining is how long is left in the game day containing t.
func Remaining(t time.Time) time.Duration { return DayEnd(t).Sub(t) }

// FormatGameDate renders a game_date as YYYY-MM-DD.
func FormatGameDate(d time.Time) string { return d.In(JST).Format("2006-01-02") }
