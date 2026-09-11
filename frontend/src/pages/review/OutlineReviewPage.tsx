// 提纲审核页：档案员/管理员审核采访员提交的提纲版本；通过后版本锁定，退回必须填写原因。
import { useCallback, useEffect, useState } from 'react'
import EmptyState from '../../components/EmptyState'
import StatusBadge from '../../components/StatusBadge'
import {
  OUTLINE_STATUS_APPROVED,
  OUTLINE_STATUS_REJECTED,
  ROLE_ADMIN,
  ROLE_ARCHIVIST,
} from '../../constants'
import { useAuthStore } from '../../stores/authStore'
import { useOutlineStore } from '../../stores/outlineStore'
import { useProjectStore } from '../../stores/projectStore'
import { formatDateTime } from '../../utils/format'
import type { OutlineVersion } from '../../api/types'

export default function OutlineReviewPage() {
  const { user } = useAuthStore()
  const { pending, pendingTotal, loading, fetchPending, fetchVersion, review } = useOutlineStore()
  const { projects, fetchList } = useProjectStore()
  const [detail, setDetail] = useState<OutlineVersion | null>(null)
  const [reason, setReason] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const canReview = user?.role === ROLE_ARCHIVIST || user?.role === ROLE_ADMIN

  const reload = useCallback(() => {
    fetchPending(1, 50)
  }, [fetchPending])

  useEffect(() => {
    fetchList({ page: 1, page_size: 100 })
    reload()
  }, [fetchList, reload])

  if (!canReview) {
    return (
      <div className="page">
        <EmptyState title="无访问权限" description="仅档案员或管理员可以审核访谈提纲" />
      </div>
    )
  }

  const projectTitle = (projectId: number) => projects.find((p) => p.id === projectId)?.title || `项目 #${projectId}`

  const openDetail = async (version: OutlineVersion) => {
    setReason('')
    setError('')
    const full = await fetchVersion(version.project_id, version.id)
    setDetail(full)
  }

  const doReview = async (action: typeof OUTLINE_STATUS_APPROVED | typeof OUTLINE_STATUS_REJECTED) => {
    if (!detail) return
    const trimmed = reason.trim()
    if (action === OUTLINE_STATUS_REJECTED && !trimmed) {
      setError('退回提纲必须填写退回原因')
      return
    }
    setBusy(true)
    setError('')
    try {
      await review(detail.id, action === OUTLINE_STATUS_APPROVED ? 'approve' : 'reject', trimmed || undefined)
      setMessage(action === OUTLINE_STATUS_APPROVED ? '已审核通过，版本已锁定' : '已退回采访员')
      setTimeout(() => setMessage(''), 3000)
      setDetail(null)
      reload()
    } catch (e) {
      setError(e instanceof Error ? e.message : '审核失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page">
      {message && <div className="toast success">{message}</div>}
      {error && <div className="toast error">{error}</div>}
      <div className="page-header">
        <h2>提纲审核</h2>
        <span className="muted">待审核 {pendingTotal} 条</span>
      </div>

      {pending.length === 0 && !loading ? (
        <EmptyState title="暂无待审核提纲" description="采访员提交后将出现在这里" />
      ) : (
        <section className="card">
          <div className="version-list">
            {pending.map((v) => (
              <div key={v.id} className="version-row">
                <StatusBadge status={v.status} type="outline" />
                <span>{projectTitle(v.project_id)}</span>
                <span className="muted">v{v.version_number}</span>
                <span className="muted">提交于 {v.submitted_at ? formatDateTime(v.submitted_at) : '-'}</span>
                <button className="btn btn-primary btn-small" onClick={() => openDetail(v)}>
                  查看并审核
                </button>
              </div>
            ))}
          </div>
        </section>
      )}

      {detail && (
        <section className="card" style={{ marginTop: 16 }}>
          <div className="card-title">
            {projectTitle(detail.project_id)} · v{detail.version_number} 提纲内容
          </div>
          <ul className="question-list">
            {(detail.questions || []).map((q, idx) => (
              <li key={q.id} className="question-item">
                <span className="question-index">{idx + 1}</span>
                <span className="question-content">{q.content}</span>
              </li>
            ))}
            {(detail.questions || []).length === 0 && <EmptyState title="该版本没有问题" description="" />}
          </ul>
          <div className="form-row" style={{ marginTop: 12 }}>
            <label>退回原因（退回时必填，通过时忽略）</label>
            <textarea
              rows={3}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="如：问题顺序需要调整，请补充受访者早年经历相关问题"
            />
          </div>
          <div className="row-actions" style={{ marginTop: 12 }}>
            <button className="btn btn-primary btn-small" disabled={busy} onClick={() => doReview(OUTLINE_STATUS_APPROVED)}>
              审核通过（锁定版本）
            </button>
            <button className="btn btn-danger btn-small" disabled={busy} onClick={() => doReview(OUTLINE_STATUS_REJECTED)}>
              退回（需原因）
            </button>
            <button className="btn btn-plain btn-small" disabled={busy} onClick={() => setDetail(null)}>
              取消
            </button>
          </div>
        </section>
      )}
    </div>
  )
}
