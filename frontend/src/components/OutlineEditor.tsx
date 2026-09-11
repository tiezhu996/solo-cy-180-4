// 提纲编辑器：采访员在项目详情页维护草稿问题、提交审核，并查看历史版本。
// 已通过版本只读；调整已通过提纲时基于它创建新草稿版本，不覆盖历史。
import { useCallback, useEffect, useState } from 'react'
import ConfirmDialog from './ConfirmDialog'
import EmptyState from './EmptyState'
import StatusBadge from './StatusBadge'
import {
  OUTLINE_STATUS_APPROVED,
  OUTLINE_STATUS_DRAFT,
  OUTLINE_STATUS_REJECTED,
  OUTLINE_STATUS_SUBMITTED,
  ROLE_INTERVIEWER,
} from '../constants'
import { useAuthStore } from '../stores/authStore'
import { useOutlineStore } from '../stores/outlineStore'
import { formatDateTime } from '../utils/format'
import type { OutlineVersion } from '../api/types'

interface OutlineEditorProps {
  projectId: number
  projectArchived: boolean
  onBusy?: (busy: boolean) => void
}

interface DraftLine {
  question_id: number
  content: string
  sort_order: number
}

export default function OutlineEditor({ projectId, projectArchived, onBusy }: OutlineEditorProps) {
  const { user } = useAuthStore()
  const { versions, fetchByProject, ensureDraft, saveQuestions, removeQuestion, submit } = useOutlineStore()
  const [activeId, setActiveId] = useState<number>(0)
  const [activeDetail, setActiveDetail] = useState<OutlineVersion | null>(null)
  const [lines, setLines] = useState<DraftLine[]>([])
  const [newContent, setNewContent] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const isInterviewer = user?.role === ROLE_INTERVIEWER || user?.role === 'admin'

  useEffect(() => {
    fetchByProject(projectId).catch(() => undefined)
  }, [projectId, fetchByProject])

  // 默认选中最新版本（列表已按版本号倒序）。
  useEffect(() => {
    if (!activeId && versions.length > 0) {
      setActiveId(versions[0].id)
    }
  }, [versions, activeId])

  const active: OutlineVersion | undefined = versions.find((v) => v.id === activeId)
  const editable =
    !!active &&
    (active.status === OUTLINE_STATUS_DRAFT || active.status === OUTLINE_STATUS_REJECTED) &&
    isInterviewer

  const flash = (text: string, isError = false) => {
    setMessage(isError ? '' : text)
    setError(isError ? text : '')
    setTimeout(() => {
      setMessage('')
      setError('')
    }, 4000)
  }

  const run = useCallback(
    async (fn: () => Promise<void>) => {
      setBusy(true)
      onBusy?.(true)
      try {
        await fn()
      } catch (e) {
        flash(e instanceof Error ? e.message : '操作失败', true)
      } finally {
        setBusy(false)
        onBusy?.(false)
      }
    },
    [onBusy],
  )

  const openVersion = async (version: OutlineVersion) => {
    setActiveId(version.id)
    setNewContent('')
    const detail = await useOutlineStore.getState().fetchVersion(projectId, version.id)
    setActiveDetail(detail)
    setLines(
      (detail.questions || []).map((q) => ({ question_id: q.id, content: q.content, sort_order: q.sort_order })),
    )
  }

  // 切换版本时加载该版本的问题内容。
  useEffect(() => {
    if (active) {
      openVersion(active)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeId])

  const handleStartDraft = (baseVersionId = 0) =>
    run(async () => {
      const draft = await ensureDraft(projectId, baseVersionId)
      const latest = await fetchByProject(projectId)
      setActiveId(draft.id)
      const fresh = latest.find((v) => v.id === draft.id) || draft
      setActiveDetail(draft)
      setLines((fresh.questions || []).map((q) => ({ question_id: q.id, content: q.content, sort_order: q.sort_order })))
      flash(baseVersionId ? '已基于已通过版本创建新草稿版本' : '草稿版本已创建')
    })

  const addLine = () => {
    const text = newContent.trim()
    if (!text) return
    setLines((prev) => [...prev, { question_id: 0, content: text, sort_order: prev.length }])
    setNewContent('')
  }

  const handleSave = () =>
    run(async () => {
      if (lines.length === 0) {
        flash('请至少保留一个问题再保存', true)
        return
      }
      const updated = await saveQuestions(projectId, activeId, lines)
      setActiveDetail(updated)
      setLines((updated.questions || []).map((q) => ({ question_id: q.id, content: q.content, sort_order: q.sort_order })))
      await fetchByProject(projectId)
      flash('提纲草稿已保存')
    })

  const handleDeleteLine = (questionId: number) =>
    run(async () => {
      await removeQuestion(projectId, activeId, questionId)
      setLines((prev) => prev.filter((l) => l.question_id !== questionId))
      flash('问题已从草稿删除')
    })

  const handleSubmit = () =>
    run(async () => {
      // 提交前先保存，确保草稿内的改动一并送审。
      if (lines.length > 0) {
        const updated = await saveQuestions(projectId, activeId, lines)
        setActiveDetail(updated)
        setLines((updated.questions || []).map((q) => ({ question_id: q.id, content: q.content, sort_order: q.sort_order })))
      }
      await submit(projectId, activeId)
      await fetchByProject(projectId)
      flash('提纲已提交，等待档案员或管理员审核')
    })

  const readOnlyQuestions = activeDetail?.questions || []

  return (
    <div className="outline-editor">
      {message && <div className="toast success">{message}</div>}
      {error && <div className="toast error">{error}</div>}

      <div className="version-list">
        {versions.length === 0 && (
          <EmptyState title="还没有提纲版本" description="采访员创建草稿并添加问题后提交审核" />
        )}
        {versions.map((v) => (
          <div key={v.id} className={`version-row ${v.id === activeId ? 'active' : ''}`}>
            <button className="btn btn-plain btn-small" onClick={() => openVersion(v)}>
              v{v.version_number}
            </button>
            <StatusBadge status={v.status} type="outline" />
            <span className="muted">
              {v.status === OUTLINE_STATUS_SUBMITTED && v.submitted_at ? `提交于 ${formatDateTime(v.submitted_at)}` : `更新于 ${formatDateTime(v.updated_at)}`}
            </span>
            {v.status === OUTLINE_STATUS_APPROVED && v.reviewed_at && (
              <span className="muted">审核人：{v.reviewer_name || '-'}</span>
            )}
          </div>
        ))}
      </div>

      {active && (
        <>
          {active.status === OUTLINE_STATUS_REJECTED && active.reject_reason && (
            <div className="reject-reason">退回原因：{active.reject_reason}</div>
          )}

          {editable ? (
            <>
              <ul className="question-list">
                {lines.map((line, idx) => (
                  <li key={line.question_id || `new-${idx}`} className="question-item">
                    <span className="question-index">{idx + 1}</span>
                    <input
                      value={line.content}
                      onChange={(e) =>
                        setLines((prev) =>
                          prev.map((l, i) => (i === idx ? { ...l, content: e.target.value } : l)),
                        )
                      }
                      placeholder="采访问题内容"
                      style={{ flex: 1 }}
                    />
                    {line.question_id > 0 && (
                      <ConfirmDialog
                        title="删除问题"
                        message="从草稿中删除该问题？"
                        danger
                        confirmText="删除"
                        onConfirm={() => handleDeleteLine(line.question_id)}
                      >
                        <button className="btn btn-plain btn-small" disabled={busy}>
                          删除
                        </button>
                      </ConfirmDialog>
                    )}
                  </li>
                ))}
              </ul>
              <div className="inline-form">
                <input
                  value={newContent}
                  onChange={(e) => setNewContent(e.target.value)}
                  placeholder="输入新的采访问题"
                  onKeyDown={(e) => e.key === 'Enter' && addLine()}
                />
                <button className="btn btn-plain" onClick={addLine} disabled={!newContent.trim() || busy}>
                  ＋ 添加
                </button>
              </div>
              <div className="row-actions" style={{ marginTop: 12 }}>
                <button className="btn btn-primary btn-small" onClick={handleSave} disabled={busy}>
                  保存草稿
                </button>
                <button className="btn btn-primary btn-small" onClick={handleSubmit} disabled={busy}>
                  提交审核
                </button>
              </div>
            </>
          ) : (
            <>
              <ul className="question-list">
                {readOnlyQuestions.map((q, idx) => (
                  <li key={q.id} className="question-item">
                    <span className="question-index">{idx + 1}</span>
                    <span className="question-content">{q.content}</span>
                  </li>
                ))}
                {readOnlyQuestions.length === 0 && (
                  <EmptyState title="版本问题加载中或为空" description="" />
                )}
              </ul>
              {active.status === OUTLINE_STATUS_APPROVED && !projectArchived && isInterviewer && (
                <div className="row-actions" style={{ marginTop: 12 }}>
                  <ConfirmDialog
                    title="调整已通过提纲"
                    message="将基于该已通过版本创建一个新的草稿版本，历史版本保持锁定不变。继续？"
                    confirmText="创建新版本"
                    onConfirm={() => handleStartDraft(active.id)}
                  >
                    <button className="btn btn-primary btn-small" disabled={busy}>
                      调整提纲（新建版本）
                    </button>
                  </ConfirmDialog>
                </div>
              )}
              {active.status === OUTLINE_STATUS_SUBMITTED && (
                <p className="muted" style={{ marginTop: 8 }}>
                  该版本正在等待档案员或管理员审核，审核期间锁定不可编辑。
                </p>
              )}
            </>
          )}
        </>
      )}

      {versions.length === 0 && !projectArchived && isInterviewer && (
        <div className="row-actions">
          <button className="btn btn-primary btn-small" disabled={busy} onClick={() => handleStartDraft(0)}>
            创建草稿版本
          </button>
        </div>
      )}
    </div>
  )
}
