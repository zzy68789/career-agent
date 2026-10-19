import type {
  DiagnoseResponse,
  Health,
  InterviewResponse,
  MatchResponse,
  ParseResponse,
  Profile,
  Turn,
  Evidence,
  Degree,
} from "./types.ts"

export class ApiError extends Error {
  constructor(message: string) {
    super(message)
    this.name = "ApiError"
  }
}

async function request<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method: body === undefined ? "GET" : "POST",
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const data: unknown = await res.json().catch(() => null)
  if (!res.ok) {
    const message =
      isRecord(data) && typeof data.error === "string" ? data.error : `请求失败（${res.status}）`
    throw new ApiError(message)
  }
  return data as T
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

export function getHealth() {
  return request<Health>("/api/health")
}

export function parseResume(degree: Degree, resume: string) {
  return request<ParseResponse>("/api/profile", { degree, resume })
}

export function matchJobs(resume: string, profile: Profile) {
  return request<MatchResponse>("/api/match", { resume, profile })
}

export function diagnoseResume(
  resume: string,
  profile: Profile,
  jobId: string,
  evidence: Evidence[],
) {
  return request<DiagnoseResponse>("/api/diagnose", { resume, profile, jobId, evidence })
}

export function interviewTurn(
  resume: string,
  profile: Profile,
  jobId: string,
  evidence: Evidence[],
  messages: Turn[],
) {
  return request<InterviewResponse>("/api/interview", {
    resume,
    profile,
    jobId,
    evidence,
    messages,
  })
}
