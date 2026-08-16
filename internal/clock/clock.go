// Package clock はゲーム日付の時刻抽象を提供する。
//
// このアプリのゲームルールはすべて JST の暦日にスコープされるため、
// 日付に関わる処理は time.Now() を直接使わず必ずこのパッケージを経由する。
package clock

import (
	"sync"
	"time"
)

// JST はこのゲーム唯一のタイムゾーン（Asia/Tokyo）。
var JST = mustJST()

func mustJST() *time.Location {
	// scratch コンテナには tzdata があるとは限らないため、
	// 日本が1951年から使っている固定オフセット +09:00 にフォールバックする。
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		return loc
	}
	return time.FixedZone("JST", 9*60*60)
}

// Clock は差し替え可能な時刻の供給源。本番は Real、テストは Fake を使う。
type Clock interface {
	Now() time.Time
}

// Real はシステム時刻を読む。
type Real struct{}

func (Real) Now() time.Time { return time.Now() }

// Fake はテスト用に手動で操作できる Clock。並行利用しても安全。
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake は時刻を t に固定した Fake を作る。
func NewFake(t time.Time) *Fake { return &Fake{now: t} }

// NewFakeJST は指定した JST の壁時計時刻に固定した Fake を作る。
func NewFakeJST(year int, month time.Month, day, hour, min, sec int) *Fake {
	return NewFake(time.Date(year, month, day, hour, min, sec, 0, JST))
}

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Set は現在時刻を差し替える。
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}

// Advance は時計を d だけ進める。
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// GameDate は t が属する JST の暦日を、JST の 00:00 として返す。
// participants.game_date に保存されるのはこの値。
func GameDate(t time.Time) time.Time {
	jt := t.In(JST)
	return time.Date(jt.Year(), jt.Month(), jt.Day(), 0, 0, 0, 0, JST)
}

// Today は時計の現在時刻に対する GameDate。
func Today(c Clock) time.Time { return GameDate(c.Now()) }

// DayEnd は t を含むゲーム日が終わる瞬間、つまり次の JST 00:00 を返す。
// ちょうど JST 00:00 の時刻は「始まったばかりの日」に属するので、
// その DayEnd は24時間後になる。
func DayEnd(t time.Time) time.Time { return GameDate(t).AddDate(0, 0, 1) }

// Remaining は t を含むゲーム日の残り時間。
func Remaining(t time.Time) time.Duration { return DayEnd(t).Sub(t) }

// FormatGameDate は game_date を YYYY-MM-DD 形式で文字列化する。
func FormatGameDate(d time.Time) string { return d.In(JST).Format("2006-01-02") }
