// Package persona owns the daily persona: how it is generated and what the
// participant is allowed to edit afterwards.
package persona

import (
	"math"
	"math/rand/v2"
	"sync"
)

// Gender values stored in the database. The Japanese labels live in the UI.
const (
	GenderMale   = "male"
	GenderFemale = "female"
)

// Education values.
const (
	EduJuniorHigh = "中卒"
	EduHighSchool = "高卒"
	EduVocational = "専門卒"
	EduJuniorColl = "短大卒"
	EduUniversity = "大卒"
	EduGraduate   = "大学院卒"
	// EduHoi is a kusa in-joke. It waives every occupation restriction.
	EduHoi = "ホイ卒"
)

// Occupation values.
const (
	OccCivilServant = "公務員"
	OccDoctor       = "医師"
	OccNurse        = "看護師"
	OccTeacher      = "教員"
	OccEngineer     = "ITエンジニア"
	OccSales        = "営業"
	OccClerical     = "事務"
	OccRetail       = "販売・接客"
	OccFood         = "飲食"
	OccConstruction = "建設"
	OccCreator      = "クリエイター"
	OccSelfEmployed = "自営業"
	OccExecutive    = "経営者"
	OccPartTimer    = "フリーター"
	OccUnemployed   = "無職"
)

// Attributes are the system-generated, immutable "A" attributes. They are
// decided once per game day and never regenerated.
type Attributes struct {
	Age          int
	Gender       string
	HeightCm     int
	Education    string
	Occupation   string
	AnnualIncome int // 万円, always a multiple of 10
}

const (
	minAge      = 20
	maxAge      = 50
	minHeightCm = 140
	maxHeightCm = 200
	incomeStep  = 10
)

type weighted[T any] struct {
	value  T
	weight int
}

type ageBand struct {
	lo, hi int // inclusive
}

var ageBands = []weighted[ageBand]{
	{ageBand{20, 24}, 20},
	{ageBand{25, 29}, 25},
	{ageBand{30, 34}, 20},
	{ageBand{35, 39}, 15},
	{ageBand{40, 44}, 10},
	{ageBand{45, 50}, 10},
}

var genders = []weighted[string]{
	{GenderMale, 50},
	{GenderFemale, 50},
}

var educations = []weighted[string]{
	{EduJuniorHigh, 5},
	{EduHighSchool, 20},
	{EduVocational, 15},
	{EduJuniorColl, 10},
	{EduUniversity, 35},
	{EduGraduate, 10},
	{EduHoi, 5},
}

// minAgeForEducation is the youngest age at which an education is plausible.
var minAgeForEducation = map[string]int{
	EduUniversity: 22,
	EduGraduate:   24,
}

var occupations = []weighted[string]{
	{OccCivilServant, 7},
	{OccDoctor, 2},
	{OccNurse, 6},
	{OccTeacher, 6},
	{OccEngineer, 10},
	{OccSales, 10},
	{OccClerical, 10},
	{OccRetail, 10},
	{OccFood, 8},
	{OccConstruction, 8},
	{OccCreator, 6},
	{OccSelfEmployed, 6},
	{OccExecutive, 2},
	{OccPartTimer, 5},
	{OccUnemployed, 4},
}

// occupationRule is the minimum plausibility gate for an occupation.
// A zero value means "no restriction".
type occupationRule struct {
	minAge     int
	educations []string // if non-empty, education must be one of these
}

var occupationRules = map[string]occupationRule{
	OccDoctor:    {minAge: 24, educations: []string{EduUniversity, EduGraduate}},
	OccTeacher:   {minAge: 22, educations: []string{EduUniversity, EduGraduate}},
	OccExecutive: {minAge: 25},
}

// incomeRange is the inclusive 万円 range for an occupation.
type incomeRange struct {
	lo, hi int
}

var incomeRanges = map[string]incomeRange{
	OccCivilServant: {300, 750},
	OccDoctor:       {700, 1800},
	OccNurse:        {300, 750},
	OccTeacher:      {300, 750},
	OccEngineer:     {300, 900},
	OccSales:        {300, 900},
	OccClerical:     {250, 550},
	OccRetail:       {250, 550},
	OccFood:         {250, 550},
	OccConstruction: {300, 900},
	OccCreator:      {200, 1200},
	OccSelfEmployed: {200, 1200},
	OccExecutive:    {300, 3000},
	OccPartTimer:    {100, 300},
	OccUnemployed:   {0, 100},
}

// Generator produces daily personas. Safe for concurrent use.
type Generator struct {
	mu  sync.Mutex
	rnd *rand.Rand
}

// NewGenerator seeds a generator from the runtime's entropy source.
func NewGenerator() *Generator {
	return &Generator{rnd: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))}
}

// NewGeneratorWithSeed builds a deterministic generator for tests.
func NewGeneratorWithSeed(seed1, seed2 uint64) *Generator {
	return &Generator{rnd: rand.New(rand.NewPCG(seed1, seed2))}
}

// Generate rolls one persona in the spec's fixed order:
// age -> gender -> height -> education -> occupation -> annual income.
func (g *Generator) Generate() Attributes {
	g.mu.Lock()
	defer g.mu.Unlock()

	age := g.age()
	gender := pick(g.rnd, genders)
	height := minHeightCm + g.rnd.IntN(maxHeightCm-minHeightCm+1)
	education := g.education(age)
	occupation := g.occupation(age, education)
	income := g.income(age, occupation)

	return Attributes{
		Age:          age,
		Gender:       gender,
		HeightCm:     height,
		Education:    education,
		Occupation:   occupation,
		AnnualIncome: income,
	}
}

func (g *Generator) age() int {
	band := pick(g.rnd, ageBands)
	return band.lo + g.rnd.IntN(band.hi-band.lo+1)
}

// education drops candidates the age makes implausible, which renormalises the
// remaining weights automatically.
func (g *Generator) education(age int) string {
	candidates := make([]weighted[string], 0, len(educations))
	for _, e := range educations {
		if age >= minAgeForEducation[e.value] {
			candidates = append(candidates, e)
		}
	}
	return pick(g.rnd, candidates)
}

// occupation drops candidates whose rule the persona fails. ホイ卒 waives every
// rule, which is the whole point of the joke.
func (g *Generator) occupation(age int, education string) string {
	candidates := make([]weighted[string], 0, len(occupations))
	for _, o := range occupations {
		if allowsOccupation(age, education, o.value) {
			candidates = append(candidates, o)
		}
	}
	return pick(g.rnd, candidates)
}

func allowsOccupation(age int, education, occupation string) bool {
	if education == EduHoi {
		return true
	}
	rule, ok := occupationRules[occupation]
	if !ok {
		return true
	}
	if age < rule.minAge {
		return false
	}
	if len(rule.educations) > 0 && !contains(rule.educations, education) {
		return false
	}
	return true
}

// income picks a value inside the occupation's range in 10万円 steps.
// The age adjustment is deliberately weak: it shifts the distribution without
// ever making the extremes unreachable.
func (g *Generator) income(age int, occupation string) int {
	r := incomeRanges[occupation]
	steps := (r.hi - r.lo) / incomeStep

	u := math.Pow(g.rnd.Float64(), ageIncomeExponent(age))

	step := int(u * float64(steps+1))
	if step > steps {
		step = steps
	}
	return r.lo + step*incomeStep
}

// ageIncomeExponent skews a uniform [0,1) sample: >1 leans low, <1 leans high.
func ageIncomeExponent(age int) float64 {
	switch {
	case age < 30:
		return 1.3
	case age < 40:
		return 1.0
	default:
		return 0.8
	}
}

// pick chooses one value with probability proportional to its weight.
// Candidates must be non-empty with positive total weight; every filtered list
// in this package always keeps at least one unrestricted candidate.
func pick[T any](rnd *rand.Rand, items []weighted[T]) T {
	total := 0
	for _, it := range items {
		total += it.weight
	}

	roll := rnd.IntN(total)
	for _, it := range items {
		roll -= it.weight
		if roll < 0 {
			return it.value
		}
	}
	return items[len(items)-1].value
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
