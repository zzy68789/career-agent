export type Degree = "bachelor" | "master"

export type Profile = {
  degree: Degree
  name: string
  college: string
  major: string
  grade: string
  skills: string[]
  experiences: string[]
  research: string
  achievements: string[]
  intentCities: string[]
  intentRoles: string[]
  summary: string
}

export type Job = {
  id: string
  title: string
  org: string
  city: string
  minDegree: Degree | string
  families: string[]
  jd: string
  requirements: string[]
}

export type EvidenceStatus = "covered" | "partial" | "missing"

export type Evidence = {
  requirement: string
  resumeSpan: string
  status: EvidenceStatus
}

export type Match = {
  job: Job
  score: number
  semantic: number
  coverage: number
  covered: number
  total: number
  evidence: Evidence[]
  explanation: string
}

export type Diagnosis = {
  summary: string
  structureIssues: string[]
  keywordGaps: string[]
  rewrites: string[]
}

export type Turn = {
  role: "assistant" | "user"
  content: string
}

export type Score = {
  overall: number
  strengths: string[]
  improvements: string[]
  summary: string
}

export type PipelineMode = "llm" | "offline" | "degraded"

export type ParseResponse = {
  profile: Profile
  mode: PipelineMode
  warning?: string
}

export type MatchResponse = {
  matches: Match[]
  mode: PipelineMode
  warning?: string
  considered: number
  recall: "lexical" | "embedding" | string
}

export type DiagnoseResponse = {
  diagnosis: Diagnosis
  job: Job
  mode: PipelineMode
  warning?: string
}

export type InterviewResponse = {
  message: Turn
  finished: boolean
  score?: Score
  mode: PipelineMode
  warning?: string
}

export type Health = {
  ok: boolean
  mode: PipelineMode
  model?: string
  embedding: boolean
}
