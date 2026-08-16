// Package persona は日替わり Persona を担う。生成方法と、生成後に参加者が
// 編集できる範囲を定義する。
package persona

import (
	"math"
	"math/rand/v2"
	"sync"
)

// DB に保存する性別の値。日本語ラベルは UI 側が持つ。
const (
	GenderMale   = "male"
	GenderFemale = "female"
)

// 学歴の値。
const (
	EduJuniorHigh = "中卒"
	EduHighSchool = "高卒"
	EduVocational = "専門卒"
	EduJuniorColl = "短大卒"
	EduUniversity = "大卒"
	EduGraduate   = "大学院卒"
	// EduHoi は kusa の内輪ネタ。職業制約をすべて無効化する。
	EduHoi = "ホイ卒"
)

// 職業の値。
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

// Attributes はシステムが生成する不変の「A属性」。ゲーム日ごとに一度だけ
// 決まり、その日のうちに再生成されることはない。
type Attributes struct {
	Age          int
	Gender       string
	HeightCm     int
	Education    string
	Occupation   string
	AnnualIncome int // 万円。常に10の倍数
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
	lo, hi int // 両端を含む
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

// minAgeForEducation は、その学歴が成立しうる最年少の年齢。
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

// occupationRule は職業に対する最低限の妥当性チェック。
// ゼロ値は「制約なし」を意味する。
type occupationRule struct {
	minAge     int
	educations []string // 空でなければ、学歴はこのいずれかである必要がある
}

var occupationRules = map[string]occupationRule{
	OccDoctor:    {minAge: 24, educations: []string{EduUniversity, EduGraduate}},
	OccTeacher:   {minAge: 22, educations: []string{EduUniversity, EduGraduate}},
	OccExecutive: {minAge: 25},
}

// incomeRange は職業ごとの年収レンジ（万円、両端を含む）。
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

// Generator は日替わり Persona を生成する。並行利用しても安全。
type Generator struct {
	mu  sync.Mutex
	rnd *rand.Rand
}

// NewGenerator はランタイムのエントロピー源からシードした生成器を作る。
func NewGenerator() *Generator {
	return &Generator{rnd: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))}
}

// NewGeneratorWithSeed はテスト用に決定的な生成器を作る。
func NewGeneratorWithSeed(seed1, seed2 uint64) *Generator {
	return &Generator{rnd: rand.New(rand.NewPCG(seed1, seed2))}
}

// Generate は仕様が定める固定順で Persona を1体抽選する:
// 年齢 -> 性別 -> 身長 -> 学歴 -> 職業 -> 年収。
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

// education は年齢的にありえない候補を除外する。残った候補だけで抽選するため
// 重みは自動的に再正規化される。
func (g *Generator) education(age int) string {
	candidates := make([]weighted[string], 0, len(educations))
	for _, e := range educations {
		if age >= minAgeForEducation[e.value] {
			candidates = append(candidates, e)
		}
	}
	return pick(g.rnd, candidates)
}

// occupation は制約を満たさない候補を除外する。ホイ卒はすべての制約を
// 無効化する。それがこのネタの肝。
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

// income は職業のレンジ内から10万円刻みで値を選ぶ。
// 年齢による補正は意図的に弱くしてあり、分布を寄せるだけで
// 両端に到達不能な値を作らない。
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

// ageIncomeExponent は一様分布 [0,1) を歪める。1より大きいと低め、小さいと高めに寄る。
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

// pick は重みに比例した確率で値を1つ選ぶ。
// 候補は空でなく重みの合計が正である必要がある。このパッケージの絞り込みは
// 必ず制約なしの候補を1つ以上残すため、その条件は常に満たされる。
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
