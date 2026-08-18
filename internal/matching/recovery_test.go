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

func TestEvalTimeRecovery(t *testing.T) {
	tests := []struct {
		name  string
		state TimeRecoveryState
		now   time.Time
		want  TimeRecovery
	}{
		{
			// Like を1つも使っていない間はタイマーが動かない。初期の10に
			// 時間回復が自動で上積みされていくのを避けるため。
			name:  "an untouched budget never starts the timer",
			state: TimeRecoveryState{Balance: DailyLikeBudget, AnchorAt: nil},
			now:   jst(23, 59),
			want:  TimeRecovery{},
		},
		{
			name:  "less than the interval grants nothing",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(13, 10)},
			now:   jst(16, 9),
			want:  TimeRecovery{},
		},
		{
			name:  "exactly the interval grants one",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(13, 10)},
			now:   jst(16, 10),
			want:  TimeRecovery{Units: 1, Grant: 1},
		},
		{
			name:  "two intervals grant two",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(13, 10)},
			now:   jst(19, 10),
			want:  TimeRecovery{Units: 2, Grant: 2},
		},
		{
			name:  "three intervals grant three",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(13, 10)},
			now:   jst(22, 10),
			want:  TimeRecovery{Units: 3, Grant: 3},
		},
		{
			// 回数の天井は無い。抑えるのは3時間という間隔と所持上限だけ。
			name:  "there is no daily count limit",
			state: TimeRecoveryState{Balance: 0, AnchorAt: at(1, 0)},
			now:   jst(23, 0),
			want:  TimeRecovery{Units: 7, Grant: 7},
		},
		{
			// 上限まで1つしか空いていなければ、3つぶん経っていても増えるのは1つ。
			// 残り2つぶんの3時間は Units として消費され、失われる。
			name:  "the like cap clips the grant but not the units",
			state: TimeRecoveryState{Balance: LikeCap - 1, AnchorAt: at(9, 0)},
			now:   jst(18, 30),
			want:  TimeRecovery{Units: 3, Grant: 1},
		},
		{
			// 満タンなら何も増えない。それでも Units は立つので、起点は進み、
			// この3時間は取り置かれない（回復ストックを作らない）。
			name:  "a full balance loses the elapsed intervals",
			state: TimeRecoveryState{Balance: LikeCap, AnchorAt: at(12, 0)},
			now:   jst(18, 0),
			want:  TimeRecovery{Units: 2, Grant: 0},
		},
		{
			// 起点が未来にある状態は本来起きないが、時計が巻き戻っても
			// 負の回復を作らないことを確かめておく。
			name:  "an anchor in the future grants nothing",
			state: TimeRecoveryState{Balance: 4, AnchorAt: at(20, 0)},
			now:   jst(13, 0),
			want:  TimeRecovery{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvalTimeRecovery(tc.state, tc.now)
			if got != tc.want {
				t.Fatalf("EvalTimeRecovery() = %+v, want %+v", got, tc.want)
			}
			if got.Pending() != (tc.want.Units > 0) {
				t.Fatalf("Pending() = %v, want %v", got.Pending(), tc.want.Units > 0)
			}
		})
	}
}

// 上限で受け取れなかった3時間は失われる。後から Like を使っても戻らない。
func TestGrantLostToTheCapDoesNotComeBack(t *testing.T) {
	anchor := jst(9, 0)
	now := jst(18, 30)

	// 11/12 に3回ぶんの時間が経つと、増えるのは1つだけ。
	full := TimeRecoveryState{Balance: LikeCap - 1, AnchorAt: &anchor}
	got := EvalTimeRecovery(full, now)
	if want := (TimeRecovery{Units: 3, Grant: 1}); got != want {
		t.Fatalf("EvalTimeRecovery() = %+v, want %+v", got, want)
	}

	// 起点は受け取れなかった分も含めて3回ぶん進む。
	advanced := AdvanceAnchor(anchor, got.Units)
	if !advanced.Equal(jst(18, 0)) {
		t.Fatalf("advanced anchor = %s, want 18:00", advanced)
	}

	// この後 Like を2つ使っても、失われた2回ぶんは戻ってこない。
	spent := TimeRecoveryState{Balance: LikeCap - 2, AnchorAt: &advanced}
	if got := EvalTimeRecovery(spent, now); got.Pending() {
		t.Fatalf("EvalTimeRecovery() = %+v, want nothing restored", got)
	}
}

func TestAdvanceAnchorMovesByTheElapsedUnits(t *testing.T) {
	anchor := jst(13, 10)

	// 3時間ちょうどに来なくても、3時間に満たない余りは次に持ち越される。
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
