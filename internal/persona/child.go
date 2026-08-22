package persona

import "math"

// 子ガチャ。Match した2人から「子」を1人だけ抽選する。
//
// これは遺伝の予測ではない。親から受け取るのは身長と学歴の2つだけで、
// どちらも弱い bias として重みを動かすだけの、ゲーム上の演出である。
// 性別・職業・年収は親と無関係に引き直す。親が医師どうしでも無職は出るし、
// 無職とフリーターからでも医師は出る。それが引く意味になる。
//
// 抽選の素材は通常 Persona 生成（generator.go）と同じ重み表・職業制約・
// 年収レンジをそのまま使う。子ガチャ専用の分布や専用のレア度体系は作らない。

// Parent は子ガチャが親1人から受け取る値。
//
// 身長と学歴しか持たないのは意図的。職業・年収・年齢・レア度は子の抽選に
// 一切影響させないので、そもそも渡さない。「継承させない」という約束を
// どこかの if ではなく型の形で残しておく。
type Parent struct {
	HeightCm  int
	Education string
}

// ChildAttributes は子ガチャの結果。
//
// 年齢は持たない。名前・趣味・一言も持たない。表示するのは「子供本人の現在の
// プロフィール」ではなく、2人から生まれた架空の将来ステータスであるため。
type ChildAttributes struct {
	Gender       string
	HeightCm     int
	Education    string
	Occupation   string
	AnnualIncome int // 万円。常に10の倍数
}

// childHeightSpreadCm は親の平均身長に足し引きする幅。1cm 刻みの一様乱数。
const childHeightSpreadCm = 15

// educationScores は学歴を親の影響を計るための内部スコアに変換する。
// ホイ卒は数値のスコアを持たない（特殊ネタ枠として別に扱う）。
var educationScores = map[string]int{
	EduJuniorHigh: 0,
	EduHighSchool: 1,
	EduVocational: 2,
	EduJuniorColl: 2,
	EduUniversity: 3,
	EduGraduate:   4,
}

// educationBias は親の学歴スコアの帯ごとの補正。deltas は通常分布の重みに
// そのまま足す値（重みの合計が 100 なので、実質パーセントポイント）。
type educationBias struct {
	// below はこの帯の上限。親の平均スコアがこの値未満なら deltas を適用する。
	below  float64
	deltas map[string]int
}

// childEducationBiases は上から順に評価し、最初に当てはまった帯を採用する。
//
// どれも弱い bias に留めてある。親の学歴で子の学歴が決まることはなく、
// 大学院卒どうしから高卒も、中卒と高卒から大学院卒も出る。
var childEducationBiases = []educationBias{
	{below: 1.0, deltas: map[string]int{
		EduJuniorHigh: +5, EduHighSchool: +5, EduUniversity: -5, EduGraduate: -5,
	}},
	{below: 2.0, deltas: map[string]int{
		EduHighSchool: +5, EduVocational: +3, EduUniversity: -3, EduGraduate: -5,
	}},
	// 2.0 以上 3.0 未満はほぼ通常分布。大きな補正を掛けない。
	{below: 3.0, deltas: nil},
	{below: 3.5, deltas: map[string]int{
		EduUniversity: +7, EduGraduate: +3, EduHighSchool: -5, EduJuniorHigh: -5,
	}},
	// 上限なしの帯。3.5 以上はここに落ちる。
	{below: math.Inf(1), deltas: map[string]int{
		EduUniversity: +5, EduGraduate: +10,
		EduHighSchool: -5, EduJuniorHigh: -5, EduVocational: -5,
	}},
}

// perMilleTotal は学歴の重みを組み直すときの分母。ホイ卒の出現率を丸め誤差
// なしで固定したいので、パーセントより1桁細かいパーミルで持つ。
const perMilleTotal = 1000

// childHoiPerMille はホイ卒の出現率。添字は親のうちホイ卒だった人数。
//
// 基本は通常どおり 5%。親にホイ卒がいると少し上がるが、必ず遺伝するわけでは
// なく、親2人ともホイ卒でも 80% は別の学歴になる。
var childHoiPerMille = [3]int{50, 100, 200}

// GenerateChild は親2人から子を1人抽選する。
//
// 抽選順は通常 Persona と同じ並びから年齢だけを抜いたもの:
// 性別 -> 身長 -> 学歴 -> 職業 -> 年収。
func (g *Generator) GenerateChild(a, b Parent) ChildAttributes {
	g.mu.Lock()
	defer g.mu.Unlock()

	gender := pick(g.rnd, genders)
	height := g.childHeight(a.HeightCm, b.HeightCm)
	education := pick(g.rnd, childEducationWeights(a.Education, b.Education))
	// 子は年齢を持たないので、職業の年齢条件は使わない。学歴の条件はそのまま
	// 効き、ホイ卒はここでも全部の制約を無効化する。
	occupation := g.occupationFor(nil, education)
	// 年収も年齢補正を持たない。職業レンジからの一様抽選になる。
	income := g.incomeIn(occupation, 1)

	return ChildAttributes{
		Gender:       gender,
		HeightCm:     height,
		Education:    education,
		Occupation:   occupation,
		AnnualIncome: income,
	}
}

// childHeight は親の平均身長に ±15cm の一様乱数を足し、Persona と同じ
// 140〜200cm に収める。
//
// 身長だけは親の影響をはっきり出す。性別による補正は入れない。
func (g *Generator) childHeight(a, b int) int {
	average := (a + b) / 2
	offset := g.rnd.IntN(childHeightSpreadCm*2+1) - childHeightSpreadCm
	return min(max(average+offset, minHeightCm), maxHeightCm)
}

// childEducationWeights は通常の学歴分布に親の弱い bias を掛けた重みを返す。
// 合計は必ず perMilleTotal になる。
func childEducationWeights(a, b string) []weighted[string] {
	base := make(map[string]int, len(educations))
	for _, e := range educations {
		base[e.value] = e.weight
	}

	for edu, delta := range childEducationBias(a, b) {
		base[edu] = max(0, base[edu]+delta)
	}

	return spreadEducationWeights(base, childHoiPerMille[hoiParentCount(a, b)])
}

// childEducationBias は親の平均学歴スコアに対応する補正を返す。
//
// どちらかがホイ卒なら平均そのものが定義できないので、スコアによる補正は
// 掛けない。その場合に効くのはホイ卒の出現率の上乗せだけ。
func childEducationBias(a, b string) map[string]int {
	if a == EduHoi || b == EduHoi {
		return nil
	}

	score := float64(educationScores[a]+educationScores[b]) / 2
	for _, band := range childEducationBiases {
		if score < band.below {
			return band.deltas
		}
	}
	return nil
}

func hoiParentCount(a, b string) int {
	count := 0
	if a == EduHoi {
		count++
	}
	if b == EduHoi {
		count++
	}
	return count
}

// spreadEducationWeights はホイ卒の重みを hoiPerMille に固定し、残りを
// ホイ卒以外の重みの比で埋める。合計は必ず perMilleTotal になる。
//
// ホイ卒の出現率だけを狙った値にしたいので、比例縮小の切り捨てで余ったぶんは
// いちばん重い候補に寄せる。ずれを他へ散らさないことで、ホイ卒 10% / 20% が
// 丸め誤差で動かない。
func spreadEducationWeights(base map[string]int, hoiPerMille int) []weighted[string] {
	othersTotal := 0
	for _, e := range educations {
		if e.value != EduHoi {
			othersTotal += base[e.value]
		}
	}
	if othersTotal <= 0 {
		// 補正はどれも全候補を 0 にはしないため、ここには来ない。
		// 0 除算だけは通さないための保険。
		othersTotal = 1
	}

	budget := perMilleTotal - hoiPerMille
	out := make([]weighted[string], len(educations))
	assigned, heaviest := 0, -1

	for i, e := range educations {
		if e.value == EduHoi {
			out[i] = weighted[string]{EduHoi, hoiPerMille}
			continue
		}
		weight := base[e.value] * budget / othersTotal
		out[i] = weighted[string]{e.value, weight}
		assigned += weight
		if heaviest < 0 || weight > out[heaviest].weight {
			heaviest = i
		}
	}

	out[heaviest].weight += budget - assigned
	return out
}
