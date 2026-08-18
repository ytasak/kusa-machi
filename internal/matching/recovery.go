package matching

import (
	"time"

	"kusamachi/internal/clock"
)

// TimeRecoveryInterval は時間回復1つぶんの待ち時間。
const TimeRecoveryInterval = 3 * time.Hour

// TimeRecoveryState は時間回復の判定に必要な当日の状態。
type TimeRecoveryState struct {
	// Balance は現在の Like 所持数。
	Balance int

	// AnchorAt は次の回復を計る起点。nil は「まだ Like を1つも使っていない」で、
	// この状態ではタイマーが動かない。初期の10 Like に時間回復が自動で
	// 上積みされていくのを避けるため、起点は初回の消費まで置かない。
	AnchorAt *time.Time
}

// TimeRecovery は now の時点で確定した時間回復の内容。
//
// Units と Grant を分けているのは、所持上限で受け取れなかった分を失わせる
// ため。経過した3時間はすべて Units として消費され、そのうち残高に入るのは
// Grant だけ。差は取り置かれず、後から Like を使っても戻らない。
// 未取得ぶんをストックする仕組みは持たない。
type TimeRecovery struct {
	// Units は起点から経過していた3時間の単位数。起点はこの数ぶん進む。
	Units int

	// Grant は残高に実際に足す Like 数。所持上限に収まらない分は落ちる。
	Grant int
}

// Pending は反映すべきものがあるか。Grant が 0 でも Units があれば、起点を
// 進めるために書き込みが必要になる。満タンのまま3時間が過ぎた場合がこれで、
// その3時間は何も増やさずに消費される。
func (r TimeRecovery) Pending() bool { return r.Units > 0 }

// EvalTimeRecovery は now の時点で確定する回復内容を返す。
//
// 抑えるのは経過時間と所持上限の2つだけで、回数の天井は持たない。3時間ごとに
// 1つという間隔と、所持上限 12 で溢れた分を失わせることの2つで、Like が
// 希少なままであることは保たれる。
//
// アクセスしていない時間も経過時間に含まれる。3時間の境目ごとに見に来る必要は
// なく、次に開いた時点でまとめて計算される。
func EvalTimeRecovery(s TimeRecoveryState, now time.Time) TimeRecovery {
	if s.AnchorAt == nil {
		return TimeRecovery{}
	}

	elapsed := now.Sub(*s.AnchorAt)
	if elapsed < TimeRecoveryInterval {
		return TimeRecovery{}
	}

	units := int(elapsed / TimeRecoveryInterval)
	grant := units
	if room := LikeCap - s.Balance; grant > room {
		grant = room
	}
	if grant < 0 {
		grant = 0
	}
	return TimeRecovery{Units: units, Grant: grant}
}

// AdvanceAnchor は units 個ぶん進めた起点を返す。
//
// 進めるのは経過していた3時間単位ぶんすべて。所持上限で受け取れなかった分も
// ここで消費されるため、満タンの間に過ぎた3時間は貯まらない。3時間に満たない
// 余りは進めないので、次回の計算へそのまま持ち越される。
func AdvanceAnchor(anchor time.Time, units int) time.Time {
	return anchor.Add(TimeRecoveryInterval * time.Duration(units))
}

// NextTimeRecoveryAt は次に時間回復が起きる時刻を返す。起きないなら nil。
//
// nil になるのは次の2つ。画面はこの nil をそのまま「タイマーを出さない」
// 条件として使える。
//   - まだ Like を使っておらず、タイマーが始まっていない
//   - 所持数が上限に達している（満タンなので待つ意味が無い）
//
// 加えて、次の回復がその日の終わりより後になる場合も nil にする。0:00 で
// すべてリセットされるため、そのタイマーは決して満了しない。
//
// 付与できるぶんを反映した persona に対して呼ぶこと。付与前の状態に対して
// 呼ぶと、すでに過ぎた時刻が返る。
func NextTimeRecoveryAt(s TimeRecoveryState, now time.Time) *time.Time {
	if s.AnchorAt == nil {
		return nil
	}
	if s.Balance >= LikeCap {
		return nil
	}

	next := AdvanceAnchor(*s.AnchorAt, 1)
	if !next.Before(clock.DayEnd(now)) {
		return nil
	}
	return &next
}
