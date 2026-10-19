import { useEffect, useRef, useState } from "react"
import {
  diagnoseResume,
  getHealth,
  interviewTurn,
  matchJobs,
  parseResume,
} from "./api.ts"
import { samples } from "./samples.ts"
import type {
  Diagnosis,
  Evidence,
  Health,
  Match,
  MatchResponse,
  Profile,
  Score,
  Turn,
  Degree,
  PipelineMode,
} from "./types.ts"

const degreeText: Record<Degree, string> = { bachelor: "本科", master: "硕士" }
const statusText = { covered: "已覆盖", partial: "部分覆盖", missing: "未覆盖" }
const modeText: Record<PipelineMode, string> = {
  llm: "大模型",
  offline: "本地规则",
  degraded: "已回退本地规则",
}

export default function App() {
  const [health, setHealth] = useState<Health | null>(null)
  const [offlineBanner, setOfflineBanner] = useState("")
  const [degree, setDegree] = useState<Degree>("bachelor")
  const [resume, setResume] = useState(samples[0].resume)
  const [busy, setBusy] = useState("")
  const [error, setError] = useState("")
  const [warning, setWarning] = useState("")
  const [profile, setProfile] = useState<Profile | null>(null)
  const [profileMode, setProfileMode] = useState<PipelineMode>("offline")
  const [match, setMatch] = useState<MatchResponse | null>(null)
  const [selectedId, setSelectedId] = useState("")
  const [diagnosis, setDiagnosis] = useState<Diagnosis | null>(null)
  const [messages, setMessages] = useState<Turn[]>([])
  const [finished, setFinished] = useState(false)
  const [score, setScore] = useState<Score | null>(null)
  const [draft, setDraft] = useState("")
  const ticket = useRef(0)

  useEffect(() => {
    let alive = true
    getHealth()
      .then((data) => {
        if (alive) setHealth(data)
      })
      .catch(() => {
        if (alive) setOfflineBanner("后端没有连上。在 backend 目录启动服务后再试。")
      })
    return () => {
      alive = false
    }
  }, [])

  const selected = match?.matches.find((item) => item.job.id === selectedId) ?? null

  async function analyze() {
    const current = ++ticket.current
    setBusy("analyze")
    setError("")
    setWarning("")
    setProfile(null)
    setMatch(null)
    setSelectedId("")
    setDiagnosis(null)
    setMessages([])
    setFinished(false)
    setScore(null)
    try {
      const parsed = await parseResume(degree, resume)
      if (current !== ticket.current) return
      setProfile(parsed.profile)
      setProfileMode(parsed.mode)
      if (parsed.warning) setWarning(parsed.warning)
      const ranked = await matchJobs(resume, parsed.profile)
      if (current !== ticket.current) return
      setMatch(ranked)
      if (ranked.warning) setWarning(ranked.warning)
    } catch (err) {
      if (current === ticket.current) setError(messageOf(err))
    } finally {
      if (current === ticket.current) setBusy("")
    }
  }

  async function openJob(item: Match) {
    if (!profile) return
    const current = ++ticket.current
    setSelectedId(item.job.id)
    setDiagnosis(null)
    setMessages([])
    setFinished(false)
    setScore(null)
    setError("")
    setBusy("diagnose")
    try {
      const reviewed = await diagnoseResume(resume, profile, item.job.id, item.evidence)
      if (current !== ticket.current) return
      setDiagnosis(reviewed.diagnosis)
      if (reviewed.warning) setWarning(reviewed.warning)
      setBusy("interview")
      const opening = await interviewTurn(resume, profile, item.job.id, item.evidence, [])
      if (current !== ticket.current) return
      setMessages([opening.message])
      setFinished(opening.finished)
      setScore(opening.score ?? null)
      if (opening.warning) setWarning(opening.warning)
    } catch (err) {
      if (current === ticket.current) setError(messageOf(err))
    } finally {
      if (current === ticket.current) setBusy("")
    }
  }

  async function sendAnswer() {
    if (!profile || !selected || finished) return
    const content = draft.trim()
    if (!content) return
    const current = ++ticket.current
    const history = [...messages, { role: "user" as const, content }]
    setMessages(history)
    setDraft("")
    setBusy("interview")
    setError("")
    try {
      const turn = await interviewTurn(resume, profile, selected.job.id, selected.evidence, history)
      if (current !== ticket.current) return
      setMessages([...history, turn.message])
      setFinished(turn.finished)
      setScore(turn.score ?? null)
      if (turn.warning) setWarning(turn.warning)
    } catch (err) {
      if (current === ticket.current) setError(messageOf(err))
    } finally {
      if (current === ticket.current) setBusy("")
    }
  }

  return (
    <div className="page">
      <header className="top">
        <div>
          <p className="eyebrow">郑州大学 · 求职智能体 MVP</p>
          <h1>简历到模拟面试</h1>
          <p className="lede">能力画像、岗位排序、简历诊断、三轮模拟面试。岗位库是计算机和电气两个方向的演示数据。</p>
        </div>
        <HealthBadge health={health} banner={offlineBanner} />
      </header>

      <main className="layout">
        <section className="panel input-panel">
          <div className="segmented" role="group" aria-label="学历">
            {(["bachelor", "master"] as const).map((item) => (
              <button
                key={item}
                type="button"
                className={degree === item ? "active" : ""}
                onClick={() => setDegree(item)}
              >
                {degreeText[item]}
              </button>
            ))}
          </div>
          <label className="field">
            <span>简历</span>
            <textarea
              value={resume}
              onChange={(event) => setResume(event.target.value)}
              rows={14}
            />
          </label>
          <div className="samples">
            {samples.map((sample) => (
              <button
                key={sample.id}
                type="button"
                className="ghost"
                onClick={() => {
                  setDegree(sample.degree)
                  setResume(sample.resume)
                }}
              >
                {sample.label}
              </button>
            ))}
          </div>
          <button type="button" className="primary" disabled={busy !== "" || resume.trim().length < 20} onClick={() => void analyze()}>
            {busy === "analyze" ? "正在生成画像并匹配…" : "生成画像并匹配"}
          </button>
        </section>

        <section className="results">
          {error ? <p className="banner bad">{error}</p> : null}
          {warning ? <p className="banner warn">{warning}</p> : null}
          {!profile && busy === "" ? <Intro /> : null}
          {profile ? <ProfileCard profile={profile} mode={profileMode} /> : null}
          {match ? (
            <MatchList
              match={match}
              selectedId={selectedId}
              busy={busy === "diagnose" || busy === "interview"}
              onOpen={(item) => void openJob(item)}
            />
          ) : null}
          {diagnosis && selected ? (
            <DiagnosisCard diagnosis={diagnosis} title={selected.job.title} />
          ) : null}
          {messages.length > 0 && selected ? (
            <Interview
              messages={messages}
              finished={finished}
              score={score}
              draft={draft}
              busy={busy === "interview"}
              onDraft={setDraft}
              onSend={() => void sendAnswer()}
            />
          ) : null}
        </section>
      </main>
    </div>
  )
}

function HealthBadge({ health, banner }: { health: Health | null; banner: string }) {
  if (banner) return <p className="badge bad">{banner}</p>
  if (!health) return <p className="badge">正在连接后端…</p>
  if (health.mode === "llm") {
    return (
      <p className="badge ok">
        大模型{health.model ? ` · ${health.model}` : ""}
        {health.embedding ? " · 向量召回" : " · 字面召回"}
      </p>
    )
  }
  return <p className="badge">本地规则。配置 backend/.env 后重启即改走大模型。</p>
}

function Intro() {
  return (
    <ol className="intro">
      <li>选择学历，粘贴简历，或先用一份示例。</li>
      <li>系统抽出能力画像，并给出通过硬性条件的岗位排序。</li>
      <li>点开一个岗位，查看引用了简历原文的诊断。</li>
      <li>按缺口回答三轮问题，结束时给出评分。</li>
    </ol>
  )
}

function ProfileCard({ profile, mode }: { profile: Profile; mode: PipelineMode }) {
  return (
    <article className="panel">
      <header className="row">
        <h2>能力画像</h2>
        <span className="meta">{modeText[mode]}</span>
      </header>
      <p className="summary">{profile.summary || "未抽出摘要"}</p>
      <dl className="facts">
        <Fact label="身份" value={[profile.name, degreeText[profile.degree], profile.grade].filter(Boolean).join(" · ")} />
        <Fact label="院系" value={[profile.college, profile.major].filter(Boolean).join(" · ")} />
        <Fact label="意向" value={[...profile.intentRoles, ...profile.intentCities].join(" · ")} />
      </dl>
      <TagList label="技能" items={profile.skills} />
      {profile.research ? <p className="note">研究方向：{profile.research}</p> : null}
      <BulletList label="经历" items={profile.experiences} />
      <BulletList label="成果" items={profile.achievements} />
    </article>
  )
}

function MatchList({
  match,
  selectedId,
  busy,
  onOpen,
}: {
  match: MatchResponse
  selectedId: string
  busy: boolean
  onOpen: (item: Match) => void
}) {
  const recall = match.recall === "embedding" ? "向量相似度" : "字面相似度"
  return (
    <article className="panel">
      <header className="row">
        <h2>岗位排序</h2>
        <span className="meta">
          {modeText[match.mode]} · {recall} · 硬性条件通过 {match.considered} 个
        </span>
      </header>
      {match.matches.length === 0 ? (
        <p className="note">没有同时满足学历和专业方向的岗位。</p>
      ) : (
        <div className="matches">
          {match.matches.map((item) => (
            <button
              key={item.job.id}
              type="button"
              className={item.job.id === selectedId ? "match active" : "match"}
              onClick={() => onOpen(item)}
              disabled={busy}
            >
              <div className="match-top">
                <strong>{item.score.toFixed(1)}</strong>
                <div>
                  <p className="job-title">{item.job.title}</p>
                  <p className="meta">
                    {item.job.org} · {item.job.city} · {item.job.minDegree === "master" ? "硕士及以上" : "本科及以上"} ·{" "}
                    {item.job.families.map(familyName).join(" / ")}
                  </p>
                </div>
              </div>
              <p className="note">{item.explanation}</p>
              <EvidenceList evidence={item.evidence} />
            </button>
          ))}
        </div>
      )}
    </article>
  )
}

function DiagnosisCard({ diagnosis, title }: { diagnosis: Diagnosis; title: string }) {
  return (
    <article className="panel">
      <header className="row">
        <h2>简历诊断</h2>
        <span className="meta">{title}</span>
      </header>
      <p className="summary">{diagnosis.summary}</p>
      <BulletList label="结构" items={diagnosis.structureIssues} />
      <TagList label="缺口" items={diagnosis.keywordGaps} />
      <BulletList label="可写进简历的三句" items={diagnosis.rewrites} />
    </article>
  )
}

function Interview({
  messages,
  finished,
  score,
  draft,
  busy,
  onDraft,
  onSend,
}: {
  messages: Turn[]
  finished: boolean
  score: Score | null
  draft: string
  busy: boolean
  onDraft: (value: string) => void
  onSend: () => void
}) {
  return (
    <article className="panel">
      <header className="row">
        <h2>模拟面试</h2>
        <span className="meta">{finished ? "已结束" : "回答三轮后评分"}</span>
      </header>
      <div className="thread">
        {messages.map((turn, index) => (
          <p key={`${turn.role}-${index}`} className={turn.role === "user" ? "bubble user" : "bubble"}>
            {turn.content}
          </p>
        ))}
      </div>
      {score ? <ScoreCard score={score} /> : null}
      {finished ? null : (
        <form
          className="reply"
          onSubmit={(event) => {
            event.preventDefault()
            onSend()
          }}
        >
          <input
            value={draft}
            onChange={(event) => onDraft(event.target.value)}
            placeholder="写清你的职责、工具和结果"
            disabled={busy}
          />
          <button type="submit" className="primary" disabled={busy || draft.trim() === ""}>
            {busy ? "发送中…" : "发送"}
          </button>
        </form>
      )}
    </article>
  )
}

function ScoreCard({ score }: { score: Score }) {
  return (
    <div className="score">
      <strong>{score.overall}</strong>
      <div>
        <p>{score.summary}</p>
        <TagList label="优势" items={score.strengths} />
        <TagList label="改进" items={score.improvements} />
      </div>
    </div>
  )
}

function EvidenceList({ evidence }: { evidence: Evidence[] }) {
  return (
    <ul className="evidence">
      {evidence.map((item) => (
        <li key={item.requirement} className={item.status}>
          <span>{statusText[item.status]}</span>
          <div>
            <p>{item.requirement}</p>
            {item.resumeSpan ? <q>{item.resumeSpan}</q> : null}
          </div>
        </li>
      ))}
    </ul>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  if (!value) return null
  return (
    <>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </>
  )
}

function TagList({ label, items }: { label: string; items: string[] }) {
  if (items.length === 0) return null
  return (
    <div className="tags">
      <span>{label}</span>
      {items.map((item) => (
        <em key={item}>{item}</em>
      ))}
    </div>
  )
}

function BulletList({ label, items }: { label: string; items: string[] }) {
  if (items.length === 0) return null
  return (
    <div className="bullets">
      <span>{label}</span>
      <ul>
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </div>
  )
}

function familyName(family: string) {
  if (family === "cs") return "计算机"
  if (family === "ee") return "电气"
  return family
}

function messageOf(err: unknown) {
  return err instanceof Error ? err.message : "请求失败"
}
