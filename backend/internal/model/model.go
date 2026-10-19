package model

// Profile is the structured reading of one resume.
// Degree is "bachelor" or "master" and comes from the user's choice.
type Profile struct {
	Degree       string   `json:"degree"`
	Name         string   `json:"name"`
	College      string   `json:"college"`
	Major        string   `json:"major"`
	Grade        string   `json:"grade"`
	Skills       []string `json:"skills"`
	Experiences  []string `json:"experiences"`
	Research     string   `json:"research"`
	Achievements []string `json:"achievements"`
	IntentCities []string `json:"intentCities"`
	IntentRoles  []string `json:"intentRoles"`
	Summary      string   `json:"summary"`
}

// Job is one opening in the local corpus.
// MinDegree "bachelor" accepts both degrees; "master" accepts only master.
// Families are "cs" and/or "ee".
type Job struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Org          string   `json:"org"`
	City         string   `json:"city"`
	MinDegree    string   `json:"minDegree"`
	Families     []string `json:"families"`
	JD           string   `json:"jd"`
	Requirements []string `json:"requirements"`
}

// Evidence is one job requirement checked against the resume.
// Status is covered, partial, or missing.
// ResumeSpan is a verbatim excerpt, empty when nothing can be quoted.
type Evidence struct {
	Requirement string `json:"requirement"`
	ResumeSpan  string `json:"resumeSpan"`
	Status      string `json:"status"`
}

// Match is one ranked opening.
type Match struct {
	Job         Job        `json:"job"`
	Score       float64    `json:"score"`
	Semantic    float64    `json:"semantic"`
	Coverage    float64    `json:"coverage"`
	Covered     int        `json:"covered"`
	Total       int        `json:"total"`
	Evidence    []Evidence `json:"evidence"`
	Explanation string     `json:"explanation"`
}

// Diagnosis is the one-page resume review for a chosen job.
type Diagnosis struct {
	Summary         string   `json:"summary"`
	StructureIssues []string `json:"structureIssues"`
	KeywordGaps     []string `json:"keywordGaps"`
	Rewrites        []string `json:"rewrites"`
}

// Turn is one interview utterance.
type Turn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Score is the closing review after three answers.
type Score struct {
	Overall      int      `json:"overall"`
	Strengths    []string `json:"strengths"`
	Improvements []string `json:"improvements"`
	Summary      string   `json:"summary"`
}

// FillStrings replaces nil slices so JSON clients can iterate.
func (p *Profile) FillStrings() {
	p.Skills = strs(p.Skills)
	p.Experiences = strs(p.Experiences)
	p.Achievements = strs(p.Achievements)
	p.IntentCities = strs(p.IntentCities)
	p.IntentRoles = strs(p.IntentRoles)
}

func strs(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
