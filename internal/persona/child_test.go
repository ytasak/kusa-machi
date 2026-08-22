package persona

import (
	"testing"
)

// childSampleSize は、5% の学歴や 2% の職業まで何度も通る程度に大きい。
// この大きさなら 5% の割合の標準偏差は 0.05% 程度なので、下の許容幅は十分に緩い。
const childSampleSize = 200_000

// tolerance は割合を比べるときの許容幅。標準偏差の10倍以上を見ているので、
// 乱数のたまたまで落ちることはない。
const tolerance = 0.005

func childSample(t *testing.T, a, b Parent) []ChildAttributes {
	t.Helper()
	g := NewGeneratorWithSeed(0xc411d, 0x9e3779b9)
	out := make([]ChildAttributes, childSampleSize)
	for i := range out {
		out[i] = g.GenerateChild(a, b)
	}
	return out
}

func educationShares(children []ChildAttributes) map[string]float64 {
	counts := map[string]int{}
	for _, c := range children {
		counts[c.Education]++
	}

	shares := make(map[string]float64, len(counts))
	for edu, n := range counts {
		shares[edu] = float64(n) / float64(len(children))
	}
	return shares
}

func occupationsOf(children []ChildAttributes) map[string]bool {
	seen := map[string]bool{}
	for _, c := range children {
		seen[c.Occupation] = true
	}
	return seen
}

func TestGenerateChildInvariants(t *testing.T) {
	parents := [2]Parent{
		{HeightCm: 172, Education: EduUniversity},
		{HeightCm: 158, Education: EduVocational},
	}

	for i, c := range childSample(t, parents[0], parents[1]) {
		if c.Gender != GenderMale && c.Gender != GenderFemale {
			t.Fatalf("sample %d: unknown gender %q", i, c.Gender)
		}
		if c.HeightCm < minHeightCm || c.HeightCm > maxHeightCm {
			t.Fatalf("sample %d: height %d out of %d-%d", i, c.HeightCm, minHeightCm, maxHeightCm)
		}
		if _, ok := educationScores[c.Education]; !ok && c.Education != EduHoi {
			t.Fatalf("sample %d: unknown education %q", i, c.Education)
		}

		r, ok := incomeRanges[c.Occupation]
		if !ok {
			t.Fatalf("sample %d: unknown occupation %q", i, c.Occupation)
		}
		// 年収は職業レンジ内の10万円刻み。親の年収は入力にすら含まれない
		// （Parent が持っているのは身長と学歴だけ）。
		if c.AnnualIncome < r.lo || c.AnnualIncome > r.hi {
			t.Fatalf("sample %d: income %d out of %s range %d-%d",
				i, c.AnnualIncome, c.Occupation, r.lo, r.hi)
		}
		if c.AnnualIncome%incomeStep != 0 {
			t.Fatalf("sample %d: income %d is not a multiple of %d", i, c.AnnualIncome, incomeStep)
		}
	}
}

func TestChildGenderIsAnEvenCoinFlip(t *testing.T) {
	// 親の性別はそもそも Parent に無いので、確率に影響する余地がない。
	children := childSample(t, Parent{HeightCm: 180, Education: EduHighSchool},
		Parent{HeightCm: 160, Education: EduHighSchool})

	males := 0
	for _, c := range children {
		if c.Gender == GenderMale {
			males++
		}
	}

	share := float64(males) / float64(len(children))
	if share < 0.5-tolerance || share > 0.5+tolerance {
		t.Fatalf("male share = %.4f, want 0.5 +- %.3f", share, tolerance)
	}
}

func TestChildHeightStaysWithinFifteenOfTheParentAverage(t *testing.T) {
	// 親 180 / 160 の平均は 170。取りうるのは 155〜185 だけ。
	children := childSample(t, Parent{HeightCm: 180, Education: EduUniversity},
		Parent{HeightCm: 160, Education: EduUniversity})

	lowest, highest := maxHeightCm, minHeightCm
	for i, c := range children {
		if c.HeightCm < 155 || c.HeightCm > 185 {
			t.Fatalf("sample %d: height %d outside 170 +- %d", i, c.HeightCm, childHeightSpreadCm)
		}
		lowest = min(lowest, c.HeightCm)
		highest = max(highest, c.HeightCm)
	}

	// 幅の両端まで実際に出ていること。出ないなら補正が片側に寄っている。
	if lowest != 155 || highest != 185 {
		t.Fatalf("height range = %d-%d, want the full 155-185", lowest, highest)
	}
}

func TestChildHeightIsClampedToThePersonaRange(t *testing.T) {
	// 親 198 / 190 の平均は 194。+12 で 206 になるが、上限 200 に収める。
	tall := childSample(t, Parent{HeightCm: 198, Education: EduUniversity},
		Parent{HeightCm: 190, Education: EduUniversity})

	atCap := 0
	for i, c := range tall {
		if c.HeightCm > maxHeightCm {
			t.Fatalf("sample %d: height %d exceeded the cap", i, c.HeightCm)
		}
		if c.HeightCm == maxHeightCm {
			atCap++
		}
	}
	if atCap == 0 {
		t.Fatal("no child hit the 200cm cap; the clamp is never exercised")
	}

	// 逆側も同じ。平均 140 から -15 しても 140 より下には行かない。
	short := childSample(t, Parent{HeightCm: 141, Education: EduHighSchool},
		Parent{HeightCm: 140, Education: EduHighSchool})

	atFloor := 0
	for i, c := range short {
		if c.HeightCm < minHeightCm {
			t.Fatalf("sample %d: height %d fell below the floor", i, c.HeightCm)
		}
		if c.HeightCm == minHeightCm {
			atFloor++
		}
	}
	if atFloor == 0 {
		t.Fatal("no child hit the 140cm floor; the clamp is never exercised")
	}
}

// TestChildEducationWeightsSumToOneThousand は重みの組み直しが常に総和を保ち、
// ホイ卒の出現率を狙った値に固定できていることを、統計ではなく重みそのもので
// 確かめる。
func TestChildEducationWeightsSumToOneThousand(t *testing.T) {
	all := []string{
		EduJuniorHigh, EduHighSchool, EduVocational, EduJuniorColl,
		EduUniversity, EduGraduate, EduHoi,
	}

	for _, a := range all {
		for _, b := range all {
			weights := childEducationWeights(a, b)

			total, hoi := 0, 0
			for _, w := range weights {
				if w.weight < 0 {
					t.Fatalf("%s x %s: negative weight for %s", a, b, w.value)
				}
				total += w.weight
				if w.value == EduHoi {
					hoi = w.weight
				}
			}

			if total != perMilleTotal {
				t.Fatalf("%s x %s: weights total %d, want %d", a, b, total, perMilleTotal)
			}
			if want := childHoiPerMille[hoiParentCount(a, b)]; hoi != want {
				t.Fatalf("%s x %s: ホイ卒 weight = %d, want %d", a, b, hoi, want)
			}
		}
	}
}

// TestMidScoreParentsKeepTheBaseDistribution は、スコア 2.0〜3.0 の帯では
// 通常の学歴分布がほぼそのまま出ることを確かめる。
func TestMidScoreParentsKeepTheBaseDistribution(t *testing.T) {
	// 専門卒(2) と 短大卒(2) の平均は 2.0。補正なしの帯。
	shares := educationShares(childSample(t,
		Parent{HeightCm: 170, Education: EduVocational},
		Parent{HeightCm: 165, Education: EduJuniorColl}))

	want := map[string]float64{
		EduJuniorHigh: 0.05,
		EduHighSchool: 0.20,
		EduVocational: 0.15,
		EduJuniorColl: 0.10,
		EduUniversity: 0.35,
		EduGraduate:   0.10,
		EduHoi:        0.05,
	}

	for edu, expected := range want {
		if got := shares[edu]; got < expected-tolerance || got > expected+tolerance {
			t.Fatalf("%s share = %.4f, want %.2f +- %.3f", edu, got, expected, tolerance)
		}
	}
}

// TestParentEducationBiasesWithoutDeciding は、親の学歴が確率を動かしはするが
// 結果を固定はしないことを確かめる。ここが固定されると、引く意味が消える。
func TestParentEducationBiasesWithoutDeciding(t *testing.T) {
	low := educationShares(childSample(t,
		Parent{HeightCm: 170, Education: EduJuniorHigh},
		Parent{HeightCm: 165, Education: EduJuniorHigh}))

	high := educationShares(childSample(t,
		Parent{HeightCm: 170, Education: EduGraduate},
		Parent{HeightCm: 165, Education: EduGraduate}))

	// bias の向きは効いている。
	if high[EduGraduate] <= low[EduGraduate] {
		t.Fatalf("大学院卒 share did not rise with the parents: %.4f vs %.4f",
			high[EduGraduate], low[EduGraduate])
	}
	if low[EduJuniorHigh] <= high[EduJuniorHigh] {
		t.Fatalf("中卒 share did not rise for low-score parents: %.4f vs %.4f",
			low[EduJuniorHigh], high[EduJuniorHigh])
	}

	// それでも決定はしない。大学院卒どうしから高卒が出る。
	if high[EduHighSchool] < 0.05 {
		t.Fatalf("大学院卒 x 大学院卒 produced 高卒 only %.4f of the time", high[EduHighSchool])
	}

	// 中卒 x 高卒からでも大学院卒は出る。低確率だが 0 ではない。
	mixed := educationShares(childSample(t,
		Parent{HeightCm: 170, Education: EduJuniorHigh},
		Parent{HeightCm: 165, Education: EduHighSchool}))
	if mixed[EduGraduate] <= 0 {
		t.Fatal("中卒 x 高卒 never produced 大学院卒")
	}
}

func TestHoiSotsuParentsRaiseTheHoiRate(t *testing.T) {
	cases := []struct {
		name  string
		a, b  Parent
		share float64
	}{
		{
			"neither parent",
			Parent{HeightCm: 170, Education: EduVocational},
			Parent{HeightCm: 165, Education: EduJuniorColl},
			0.05,
		},
		{
			"one parent",
			Parent{HeightCm: 170, Education: EduHoi},
			Parent{HeightCm: 165, Education: EduUniversity},
			0.10,
		},
		{
			"both parents",
			Parent{HeightCm: 170, Education: EduHoi},
			Parent{HeightCm: 165, Education: EduHoi},
			0.20,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shares := educationShares(childSample(t, tc.a, tc.b))
			got := shares[EduHoi]
			if got < tc.share-tolerance || got > tc.share+tolerance {
				t.Fatalf("ホイ卒 share = %.4f, want %.2f +- %.3f", got, tc.share, tolerance)
			}
		})
	}

	// 親2人ともホイ卒でも、8割は別の学歴になる。遺伝はしない。
	both := educationShares(childSample(t,
		Parent{HeightCm: 170, Education: EduHoi},
		Parent{HeightCm: 165, Education: EduHoi}))
	if 1-both[EduHoi] < 0.75 {
		t.Fatalf("ホイ卒 x ホイ卒 produced ホイ卒 %.4f of the time; it must not be inherited", both[EduHoi])
	}
}

// TestChildOccupationKeepsEducationRules は学歴の制約が子でも効いていることを
// 確かめる。年齢の制約だけは子には無い（年齢属性を持たないため）。
func TestChildOccupationKeepsEducationRules(t *testing.T) {
	children := childSample(t,
		Parent{HeightCm: 170, Education: EduHighSchool},
		Parent{HeightCm: 165, Education: EduVocational})

	for i, c := range children {
		if c.Education == EduHoi {
			continue // ホイ卒は制約をすべて無効化する
		}
		switch c.Occupation {
		case OccDoctor, OccTeacher:
			if c.Education != EduUniversity && c.Education != EduGraduate {
				t.Fatalf("sample %d: %s with education %q", i, c.Occupation, c.Education)
			}
		}
	}
}

// TestChildOccupationIgnoresAgeRules は、通常 Persona なら年齢で弾かれる職業が
// 子では学歴だけで通ることを確かめる。
func TestChildOccupationIgnoresAgeRules(t *testing.T) {
	// 経営者は通常 25 歳以上に限られる。学歴の条件は持たないので、子では
	// 中卒でも出てよい。
	juniorHighExecutive := false
	// 医師は 24 歳以上かつ大卒以上。子では大卒以上でありさえすれば出てよい。
	universityDoctor := false

	children := childSample(t,
		Parent{HeightCm: 170, Education: EduJuniorHigh},
		Parent{HeightCm: 165, Education: EduHighSchool})

	for _, c := range children {
		if c.Education == EduHoi {
			continue
		}
		if c.Education == EduJuniorHigh && c.Occupation == OccExecutive {
			juniorHighExecutive = true
		}
		if c.Education == EduUniversity && c.Occupation == OccDoctor {
			universityDoctor = true
		}
	}

	if !juniorHighExecutive {
		t.Fatal("中卒の経営者が出なかった。子ガチャは年齢制約を使ってはいけない")
	}
	if !universityDoctor {
		t.Fatal("大卒の医師が出なかった。子ガチャは年齢制約を使ってはいけない")
	}
}

func TestHoiSotsuChildIgnoresOccupationRules(t *testing.T) {
	children := childSample(t,
		Parent{HeightCm: 170, Education: EduHoi},
		Parent{HeightCm: 165, Education: EduHoi})

	waived := map[string]bool{}
	for _, c := range children {
		if c.Education != EduHoi {
			continue
		}
		waived[c.Occupation] = true
	}

	for _, occ := range []string{OccDoctor, OccTeacher, OccExecutive} {
		if !waived[occ] {
			t.Fatalf("ホイ卒の子が %s になれていない。制約の無効化が効いていない", occ)
		}
	}
}

// TestChildOccupationIsNotInheritedFromParents は、親の職業が子に引き継がれ
// ないことを確かめる。
//
// Parent が身長と学歴しか持たないので継承のしようがないが、これはガチャとして
// いちばん大事な性質なので、結果の側からも確かめておく。親が大学院卒どうし
// （医師になれる学歴）でも無職やフリーターが出るし、親が中卒どうしでも
// 子は医師になれる。
func TestChildOccupationIsNotInheritedFromParents(t *testing.T) {
	elite := occupationsOf(childSample(t,
		Parent{HeightCm: 175, Education: EduGraduate},
		Parent{HeightCm: 160, Education: EduGraduate}))

	for _, occ := range []string{OccUnemployed, OccPartTimer} {
		if !elite[occ] {
			t.Fatalf("大学院卒どうしの子が %s になれていない", occ)
		}
	}

	humble := occupationsOf(childSample(t,
		Parent{HeightCm: 175, Education: EduJuniorHigh},
		Parent{HeightCm: 160, Education: EduJuniorHigh}))

	if !humble[OccDoctor] {
		t.Fatal("中卒どうしの子が医師になれていない")
	}
}

// TestChildIncomeCoversTheOccupationRange は年収がレンジの端まで出ることを
// 確かめる。年齢補正を持ち込んでいれば分布が片側に寄る。
func TestChildIncomeCoversTheOccupationRange(t *testing.T) {
	children := childSample(t,
		Parent{HeightCm: 170, Education: EduUniversity},
		Parent{HeightCm: 165, Education: EduUniversity})

	lowest := map[string]int{}
	highest := map[string]int{}
	for _, c := range children {
		if v, ok := lowest[c.Occupation]; !ok || c.AnnualIncome < v {
			lowest[c.Occupation] = c.AnnualIncome
		}
		if c.AnnualIncome > highest[c.Occupation] {
			highest[c.Occupation] = c.AnnualIncome
		}
	}

	for occ, r := range incomeRanges {
		if _, seen := lowest[occ]; !seen {
			continue // その学歴では出ない職業
		}
		if lowest[occ] != r.lo {
			t.Fatalf("%s: lowest income %d, want %d", occ, lowest[occ], r.lo)
		}
		if highest[occ] != r.hi {
			t.Fatalf("%s: highest income %d, want %d", occ, highest[occ], r.hi)
		}
	}
}
