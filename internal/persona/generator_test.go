package persona

import (
	"testing"
)

// sampleSize is large enough that every weighted branch, including the 2%
// occupations, is exercised many times.
const sampleSize = 200_000

func sample(t *testing.T) []Attributes {
	t.Helper()
	g := NewGeneratorWithSeed(0x5ea50d, 0x9e3779b9)
	out := make([]Attributes, sampleSize)
	for i := range out {
		out[i] = g.Generate()
	}
	return out
}

func TestGenerateInvariants(t *testing.T) {
	for i, a := range sample(t) {
		if a.Age < 20 || a.Age > 50 {
			t.Fatalf("sample %d: age %d out of 20-50", i, a.Age)
		}
		if a.HeightCm < 140 || a.HeightCm > 200 {
			t.Fatalf("sample %d: height %d out of 140-200", i, a.HeightCm)
		}
		if a.Gender != GenderMale && a.Gender != GenderFemale {
			t.Fatalf("sample %d: unknown gender %q", i, a.Gender)
		}
		if _, ok := incomeRanges[a.Occupation]; !ok {
			t.Fatalf("sample %d: unknown occupation %q", i, a.Occupation)
		}
	}
}

func TestEducationAgeRestrictions(t *testing.T) {
	for i, a := range sample(t) {
		switch a.Education {
		case EduUniversity:
			if a.Age < 22 {
				t.Fatalf("sample %d: 大卒 at age %d", i, a.Age)
			}
		case EduGraduate:
			if a.Age < 24 {
				t.Fatalf("sample %d: 大学院卒 at age %d", i, a.Age)
			}
		case EduJuniorHigh, EduHighSchool, EduVocational, EduJuniorColl, EduHoi:
			// no additional restriction
		default:
			t.Fatalf("sample %d: unknown education %q", i, a.Education)
		}
	}
}

func TestOccupationRestrictions(t *testing.T) {
	for i, a := range sample(t) {
		if a.Education == EduHoi {
			continue // ホイ卒 waives every occupation restriction
		}
		switch a.Occupation {
		case OccDoctor:
			if a.Age < 24 {
				t.Fatalf("sample %d: 医師 at age %d", i, a.Age)
			}
			if a.Education != EduUniversity && a.Education != EduGraduate {
				t.Fatalf("sample %d: 医師 with education %q", i, a.Education)
			}
		case OccTeacher:
			if a.Age < 22 {
				t.Fatalf("sample %d: 教員 at age %d", i, a.Age)
			}
			if a.Education != EduUniversity && a.Education != EduGraduate {
				t.Fatalf("sample %d: 教員 with education %q", i, a.Education)
			}
		case OccExecutive:
			if a.Age < 25 {
				t.Fatalf("sample %d: 経営者 at age %d", i, a.Age)
			}
		}
	}
}

func TestHoiSotsuWaivesOccupationRestrictions(t *testing.T) {
	// The joke only works if a ホイ卒 can actually be a doctor / teacher /
	// executive at an age the normal rules would forbid.
	waived := map[string]bool{}
	for _, a := range sample(t) {
		if a.Education != EduHoi {
			continue
		}
		switch {
		case a.Occupation == OccDoctor && a.Age < 24:
			waived[OccDoctor] = true
		case a.Occupation == OccTeacher && a.Age < 22:
			waived[OccTeacher] = true
		case a.Occupation == OccExecutive && a.Age < 25:
			waived[OccExecutive] = true
		}
	}
	for _, occ := range []string{OccDoctor, OccTeacher, OccExecutive} {
		if !waived[occ] {
			t.Errorf("no ホイ卒 %s bypassed the restriction in %d samples", occ, sampleSize)
		}
	}
}

func TestIncomeRangeAndStep(t *testing.T) {
	for i, a := range sample(t) {
		r := incomeRanges[a.Occupation]
		if a.AnnualIncome < r.lo || a.AnnualIncome > r.hi {
			t.Fatalf("sample %d: %s income %d outside %d-%d", i, a.Occupation, a.AnnualIncome, r.lo, r.hi)
		}
		if a.AnnualIncome%10 != 0 {
			t.Fatalf("sample %d: income %d is not a multiple of 10", i, a.AnnualIncome)
		}
	}
}

func TestIncomeReachesBothExtremes(t *testing.T) {
	// "Extreme combinations must still remain possible": the age skew must not
	// clip either end of a range.
	sawMin, sawMax := map[string]bool{}, map[string]bool{}
	for _, a := range sample(t) {
		r := incomeRanges[a.Occupation]
		if a.AnnualIncome == r.lo {
			sawMin[a.Occupation] = true
		}
		if a.AnnualIncome == r.hi {
			sawMax[a.Occupation] = true
		}
	}
	for _, o := range occupations {
		if !sawMin[o.value] {
			t.Errorf("%s never hit its minimum income", o.value)
		}
		if !sawMax[o.value] {
			t.Errorf("%s never hit its maximum income", o.value)
		}
	}
}

func TestEveryEducationAndOccupationIsReachable(t *testing.T) {
	seenEdu, seenOcc := map[string]int{}, map[string]int{}
	for _, a := range sample(t) {
		seenEdu[a.Education]++
		seenOcc[a.Occupation]++
	}
	for _, e := range educations {
		if seenEdu[e.value] == 0 {
			t.Errorf("education %s never generated", e.value)
		}
	}
	for _, o := range occupations {
		if seenOcc[o.value] == 0 {
			t.Errorf("occupation %s never generated", o.value)
		}
	}
}

func TestAgeSkewsIncomeWeakly(t *testing.T) {
	// The adjustment must be visible but weak, and it must never reorder the
	// occupation ranges. Compare average income within one occupation.
	sum := map[string]float64{}
	count := map[string]float64{}

	g := NewGeneratorWithSeed(7, 11)
	for i := 0; i < sampleSize; i++ {
		a := g.Generate()
		if a.Occupation != OccEngineer {
			continue
		}
		bucket := "30s"
		if a.Age < 30 {
			bucket = "20s"
		} else if a.Age >= 40 {
			bucket = "40plus"
		}
		sum[bucket] += float64(a.AnnualIncome)
		count[bucket]++
	}

	avg := func(b string) float64 { return sum[b] / count[b] }
	young, mid, old := avg("20s"), avg("30s"), avg("40plus")

	if !(young < mid && mid < old) {
		t.Fatalf("expected 20s < 30s < 40+ average income, got %.1f / %.1f / %.1f", young, mid, old)
	}
	if old-young > 150 {
		t.Fatalf("age adjustment is too strong: %.1f vs %.1f", young, old)
	}
}

func TestSeededGeneratorIsDeterministic(t *testing.T) {
	a := NewGeneratorWithSeed(42, 43)
	b := NewGeneratorWithSeed(42, 43)
	for i := 0; i < 1000; i++ {
		if a.Generate() != b.Generate() {
			t.Fatalf("same seed produced different personas at %d", i)
		}
	}
}
