package matching

import (
	"testing"
	"time"

	"kusamachi/internal/clock"
)

// jst は JST の壁時計時刻を組み立てる短縮形。
func jst(hour, min int) time.Time {
	return time.Date(2026, time.August, 17, hour, min, 0, 0, clock.JST)
}

func at(hour, min int) *time.Time {
	t := jst(hour, min)
	return &t
}

func TestTimeRecoveryGrant(t *testing.T) {
	tests := []struct {
		name  string
		state TimeRecoveryState
		now   time.Time
		want  int
	}{
		{
			// Like を1つも使っていない間はタイマーが動かない。初期の10に
			// 時間回復が自動で上積みされていくのを避けるため。
			name:  "an untouched budget never starts the timer",
			state: TimeRecoveryState{Balance: DailyLikeBudget, AnchorAt: nil},
			now:   jst(23, 59),
			want:  0,
		},
		{
			name:  "less than the interval grants nothing",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(13, 10)},
			now:   jst(16, 9),
			want:  0,
		},
		{
			name:  "exactly the interval grants one",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(13, 10)},
			now:   jst(16, 10),
			want:  1,
		},
		{
			name:  "two intervals grant two",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(13, 10)},
			now:   jst(19, 10),
			want:  2,
		},
		{
			// 何時間放置しても、その日に時間で得られるのは2つまで。
			name:  "the daily count caps the grant",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(1, 0)},
			now:   jst(23, 0),
			want:  MaxTimeRecoveries,
		},
		{
			name:  "an already used recovery leaves only one",
			state: TimeRecoveryState{Balance: 4, RecoveryCount: 1, AnchorAt: at(13, 10)},
			now:   jst(19, 10),
			want:  1,
		},
		{
			name:  "a spent daily count grants nothing",
			state: TimeRecoveryState{Balance: 4, RecoveryCount: MaxTimeRecoveries, AnchorAt: at(13, 10)},
			now:   jst(23, 0),
			want:  0,
		},
		{
			// 上限まで1つしか空いていなければ、2つぶん経っていても1つだけ。
			name:  "the like cap clips the grant",
			state: TimeRecoveryState{Balance: LikeCap - 1, AnchorAt: at(13, 10)},
			now:   jst(19, 10),
			want:  1,
		},
		{
			name:  "a full balance grants nothing",
			state: TimeRecoveryState{Balance: LikeCap, AnchorAt: at(13, 10)},
			now:   jst(23, 0),
			want:  0,
		},
		{
			// 起点が未来にある状態は本来起きないが、時計が巻き戻っても
			// 負の回復を作らないことを確かめておく。
			name:  "an anchor in the future grants nothing",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(20, 0)},
			now:   jst(13, 0),
			want:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := TimeRecoveryGrant(tc.state, tc.now); got != tc.want {
				t.Fatalf("TimeRecoveryGrant() = %d, want %d", got, tc.want)
			}
		})
	}
}

// 上限に達していて1つも増えなかったときは、回数も起点も動かさない。
// 使い始めればその経過時間がそのまま回復に変わる。
func TestGrantIsRestoredAfterSpendingAtTheCap(t *testing.T) {
	full := TimeRecoveryState{Balance: LikeCap, AnchorAt: at(13, 10)}
	now := jst(19, 10)

	if got := TimeRecoveryGrant(full, now); got != 0 {
		t.Fatalf("at the cap the grant = %d, want 0", got)
	}

	// 上限のままなので回数は消費されていない。2つ使えば2つ戻る。
	spent := TimeRecoveryState{Balance: LikeCap - 2, RecoveryCount: 0, AnchorAt: full.AnchorAt}
	if got := TimeRecoveryGrant(spent, now); got != 2 {
		t.Fatalf("after spending the grant = %d, want 2", got)
	}
}

func TestAdvanceAnchorOnlyMovesByWhatWasGranted(t *testing.T) {
	anchor := jst(13, 10)

	// 3時間ちょうどに来なくても、余った経過時間は次に持ち越される。
	// 16:40 に1つ受け取った場合、次は 19:10 で、20:10 にはならない。
	if got := AdvanceAnchor(anchor, 1); !got.Equal(jst(16, 10)) {
		t.Fatalf("AdvanceAnchor(1) = %s, want 16:10", got)
	}
	if got := AdvanceAnchor(anchor, 2); !got.Equal(jst(19, 10)) {
		t.Fatalf("AdvanceAnchor(2) = %s, want 19:10", got)
	}
	if got := AdvanceAnchor(anchor, 0); !got.Equal(anchor) {
		t.Fatalf("AdvanceAnchor(0) = %s, want the anchor untouched", got)
	}
}

func TestNextTimeRecoveryAt(t *testing.T) {
	tests := []struct {
		name  string
		state TimeRecoveryState
		now   time.Time
		want  *time.Time
	}{
		{
			name:  "no timer before the first like is spent",
			state: TimeRecoveryState{Balance: DailyLikeBudget, AnchorAt: nil},
			now:   jst(12, 0),
			want:  nil,
		},
		{
			name:  "one interval after the anchor",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(13, 10)},
			now:   jst(13, 10),
			want:  at(16, 10),
		},
		{
			// 上限に達したら待つ意味が無いので、タイマーを出さない。
			name:  "a full balance stops the timer",
			state: TimeRecoveryState{Balance: LikeCap, AnchorAt: at(13, 10)},
			now:   jst(13, 10),
			want:  nil,
		},
		{
			name:  "a spent daily count stops the timer",
			state: TimeRecoveryState{Balance: 4, RecoveryCount: MaxTimeRecoveries, AnchorAt: at(13, 10)},
			now:   jst(13, 10),
			want:  nil,
		},
		{
			// 0:00 で全部リセットされるため、日をまたぐタイマーは満了しない。
			name:  "a timer that would fire after the reset is not shown",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(22, 0)},
			now:   jst(22, 0),
			want:  nil,
		},
		{
			name:  "a timer that fires just before the reset is shown",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(20, 59)},
			now:   jst(20, 59),
			want:  at(23, 59),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NextTimeRecoveryAt(tc.state, tc.now)
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("NextTimeRecoveryAt() = %s, want nil", got)
			case tc.want != nil && got == nil:
				t.Fatalf("NextTimeRecoveryAt() = nil, want %s", tc.want)
			case tc.want != nil && !got.Equal(*tc.want):
				t.Fatalf("NextTimeRecoveryAt() = %s, want %s", got, tc.want)
			}
		})
	}
}
