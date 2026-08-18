package matching

import (
	"time"

	"kusamachi/internal/clock"
)

// TimeRecoveryInterval は時間回復1つぶんの待ち時間。
const TimeRecoveryInterval = 3 * time.Hour

// MaxTimeRecoveries は時間経過だけで受け取れる1日の回数。時間回復は
// Like 不足のときに再訪する理由を作るための補助であって、供給の主役ではない。
// ここで打ち止めにするので、放置しているだけで Like が増え続けることはない。
const MaxTimeRecoveries = 2

// TimeRecoveryState は時間回復の判定に必要な当日の状態。
type TimeRecoveryState struct {
	// Balance は現在の Like 所持数。
	Balance int

	// RecoveryCount はその日すでに時間回復で受け取った回数。
	RecoveryCount int

	// AnchorAt は次の回復を計る起点。nil は「まだ Like を1つも使っていない」で、
	// この状態ではタイマーが動かない。初期の10 Like に時間回復が自動で
	// 上積みされていくのを避けるため、起点は初回の消費まで置かない。
	AnchorAt *time.Time
}

// TimeRecoveryGrant は now の時点で付与できる Like 数を返す。
//
// 経過時間・1日の回数・所持上限の3つで抑える。所持上限で1つも増えないときは
// 0 を返し、回数は消費されない。起点も進まないので、その経過時間は
// 使い切るまで残る。「上限に達していたせいで回復の機会を失う」ことは無い。
func TimeRecoveryGrant(s TimeRecoveryState, now time.Time) int {
	if s.AnchorAt == nil {
		return 0
	}

	elapsed := now.Sub(*s.AnchorAt)
	if elapsed < TimeRecoveryInterval {
		return 0
	}

	grant := int(elapsed / TimeRecoveryInterval)
	if left := MaxTimeRecoveries - s.RecoveryCount; grant > left {
		grant = left
	}
	if room := LikeCap - s.Balance; grant > room {
		grant = room
	}
	if grant < 0 {
		return 0
	}
	return grant
}

// AdvanceAnchor は grant 個ぶん進めた起点を返す。
//
// 進めるのは実際に付与した回数ぶんだけ。余った経過時間を捨てないので、
// 3時間の境目ちょうどにアクセスしなくても回復が遅れて積み残ることはない。
func AdvanceAnchor(anchor time.Time, grant int) time.Time {
	return anchor.Add(TimeRecoveryInterval * time.Duration(grant))
}

// NextTimeRecoveryAt は次に時間回復が起きる時刻を返す。起きないなら nil。
//
// nil になるのは次の3つ。画面はこの nil をそのまま「タイマーを出さない」
// 条件として使える。
//   - まだ Like を使っておらず、タイマーが始まっていない
//   - その日の時間回復の回数を使い切った
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
	if s.RecoveryCount >= MaxTimeRecoveries {
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
