// Package matching は市場のルールを持つ。1日の Like 予算、回復報酬と時間回復、
// 所持上限、Pass の上限、および Match を保存する際の正規化済み Persona ペアを扱う。
package matching

import (
	"bytes"

	"github.com/google/uuid"
)

// DailyLikeBudget は各 Persona がゲーム日の開始時に得る Like の数。
// Like へのお返しも同じ予算を消費し、別枠は存在しない。
//
// 残数はこの値から導出せず、personas.like_balance として明示的に持つ。
// 時間回復は「Like を送っていないのに残数が増える」ため、送信済み Like 数
// からの引き算では表せない。この定数が使われるのは初期値の宣言だけ。
const DailyLikeBudget = 10

// LikeCap は現在の所持数の上限。回復がこれを超える分は失われる。
// 予算より少しだけ大きい。回復に意味を持たせつつ、Like を貯め込めないようにする。
const LikeCap = 12

// 回復報酬。Like が希少なままであるよう、どちらもごく少量にしてある。
const (
	// ProfileCompletionReward は名前・趣味・一言をすべて埋めたときの回復量。1日1回。
	ProfileCompletionReward = 1

	// MatchReward は Match が1つ成立したときの回復量。
	MatchReward = 2

	// MaxMatchRewards は Match 報酬を受け取れる1日の回数。ここで打ち止めにするので、
	// Match が増えるほど Like が増え続けるという正のフィードバックは起きない。
	MaxMatchRewards = 2
)

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

// GrantableLikes は報酬のうち実際に回復できる数を返す。
//
// 所持上限に収まらない分は捨てる。呼び出し側はこの戻り値をそのまま
// like_balance に足すので、失われた分が後から復活することはない。
// 上限に達していれば 0 を返すが、それでも報酬の受け取り枠は消費される。
// 時間回復だけは例外で、枠を消費しない（recovery.go を参照）。
func GrantableLikes(current, reward int) int {
	room := LikeCap - current
	if room <= 0 {
		return 0
	}
	if reward < room {
		return reward
	}
	return room
}

// ProfileComplete は Like 回復の対象になる「プロフィールを整えた」状態か。
// 3つの B属性がすべて埋まっていることを求める。値は検証済みで、空白だけの
// 入力はすでに未設定に正規化されている。
func ProfileComplete(name, hobby, bio *string) bool {
	return name != nil && hobby != nil && bio != nil
}
