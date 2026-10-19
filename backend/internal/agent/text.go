package agent

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// skillTerms are abilities worth showing on a profile.
// extraTerms help judge job requirements but are not skills by themselves.
var skillTerms = []string{
	"Spring Boot", "TypeScript", "Kubernetes", "PyTorch", "AutoCAD", "Simulink",
	"自然语言处理", "电机控制", "电力电子", "机器学习", "深度学习", "机器视觉",
	"文本匹配", "消息队列", "模型部署", "供配电", "单片机", "嵌入式", "传感器",
	"MySQL", "Redis", "React", "Docker", "MATLAB", "STM32", "Linux", "Shell",
	"Python", "Java", "HTML", "CSS", "SQL", "Git", "PLC", "C++", "Go", "C",
}

var extraTerms = []string{
	"课程设计", "统计分析", "电气图纸", "变换器", "调试", "部署", "接口",
	"实习", "项目", "论文", "竞赛", "硕士", "本科", "实验", "图纸", "测试", "电机", "联调",
}

var termsByLen []string

func init() {
	seen := map[string]bool{}
	for _, term := range append(append([]string{}, skillTerms...), extraTerms...) {
		key := normalize(term)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		termsByLen = append(termsByLen, term)
	}
	sort.Slice(termsByLen, func(i, j int) bool {
		return utf8.RuneCountInString(termsByLen[i]) > utf8.RuneCountInString(termsByLen[j])
	})
}

// normalize removes separators so a quote still matches when spaces differ.
func normalize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '#' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// words keeps a boundary where the original text had punctuation or space.
// "Go、Java" stays two words instead of becoming "gojava".
func words(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	space := true
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '#' {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func containsTerm(normText, normTerm string) bool {
	if normTerm == "" || !strings.Contains(normText, normTerm) {
		return false
	}
	if hasHan(normTerm) {
		return true
	}
	rest := normText
	offset := 0
	for {
		i := strings.Index(rest, normTerm)
		if i < 0 {
			return false
		}
		abs := offset + i
		before := abs == 0 || !asciiWord(normText[abs-1])
		end := abs + len(normTerm)
		after := end >= len(normText) || !asciiWord(normText[end])
		if before && after {
			return true
		}
		rest = rest[i+1:]
		offset = abs + 1
	}
}

func asciiWord(b byte) bool {
	return b < 128 && (unicode.IsLetter(rune(b)) || unicode.IsDigit(rune(b)))
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func termsIn(text string) []string {
	n := words(text)
	var hits []string
	seen := map[string]bool{}
	for _, term := range termsByLen {
		key := words(term)
		if seen[key] || !containsTerm(n, key) {
			continue
		}
		seen[key] = true
		hits = append(hits, term)
	}
	if hits == nil {
		return []string{}
	}
	return hits
}

func skillsIn(text string) []string {
	n := words(text)
	var hits []string
	for _, term := range skillTerms {
		if containsTerm(n, words(term)) {
			hits = append(hits, term)
		}
	}
	if hits == nil {
		return []string{}
	}
	return hits
}

func spanInResume(resume, span string) bool {
	span = strings.TrimSpace(span)
	if utf8.RuneCountInString(normalize(span)) < 4 {
		return false
	}
	return strings.Contains(normalize(resume), normalize(span))
}

func features(s string) map[string]float64 {
	s = strings.ToLower(s)
	m := map[string]float64{}
	var ascii strings.Builder
	var hans []rune
	flushASCII := func() {
		if ascii.Len() >= 2 {
			m["w:"+ascii.String()]++
		}
		ascii.Reset()
	}
	flushHan := func() {
		for i := 0; i+1 < len(hans); i++ {
			m["h:"+string(hans[i:i+2])]++
		}
		hans = hans[:0]
	}
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r):
			flushASCII()
			hans = append(hans, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '#':
			flushHan()
			ascii.WriteRune(r)
		default:
			flushASCII()
			flushHan()
		}
	}
	flushASCII()
	flushHan()
	return m
}

func cosine(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var dot, na, nb float64
	for k, av := range a {
		dot += av * b[k]
		na += av * av
	}
	for _, bv := range b {
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return clamp01(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

func cosineVec(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return clamp01(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

func clamp01(v float64) float64 {
	if v < 0 || math.IsNaN(v) {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func sentences(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '。' || r == '！' || r == '？' || r == '\n' || r == '；' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func bestSentence(resume, hints string) string {
	hintTerms := termsIn(hints)
	best := ""
	bestScore := 0
	for _, sentence := range sentences(resume) {
		score := 0
		n := words(sentence)
		for _, term := range hintTerms {
			if containsTerm(n, words(term)) {
				score += 2
			}
		}
		if score == 0 && cosine(features(sentence), features(hints)) >= 0.2 {
			score = 1
		}
		if score > bestScore {
			bestScore = score
			best = sentence
		}
	}
	return clip(best, 80)
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

func roundTo(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

func cleanList(list []string, limit int) []string {
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
