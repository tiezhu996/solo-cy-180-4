// 项目详情页：基本信息、访谈提纲版本（编辑/提交/版本查看）、时间线（录音片段 + 关键节点 + 一句话摘要）。
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import AudioPlayer from '../../components/AudioPlayer'
import ConfirmDialog from '../../components/ConfirmDialog'
import EmptyState from '../../components/EmptyState'
import OutlineEditor from '../../components/OutlineEditor'
import StatusBadge from '../../components/StatusBadge'
import {
  PROJECT_STATUS_ARCHIVED,
  PROJECT_STATUS_COMPLETED,
  PROJECT_STATUS_IN_PROGRESS,
} from '../../constants'
import { useProjectStore } from '../../stores/projectStore'
import { useRecordingStore } from '../../stores/recordingStore'
import { useTimelineStore } from '../../stores/timelineStore'
import { formatDateTime, formatDuration } from '../../utils/format'
import type { Recording, TimelineMarker } from '../../api/types'

export default function ProjectDetailPage() {
  const { id } = useParams()
  const projectId = Number(id)
  const navigate = useNavigate()
  const { detail, fetchDetail, transitionStatus, remove } = useProjectStore()
  const { recordings, fetchByProject: fetchRecordings } = useRecordingStore()
  const { markers, fetchByProject: fetchMarkers, create: createMarker } = useTimelineStore()
  const [message, setMessage] = useState('')

  useEffect(() => {
    if (projectId) {
      fetchDetail(projectId)
      fetchRecordings(projectId)
      fetchMarkers(projectId)
    }
  }, [projectId, fetchDetail, fetchRecordings, fetchMarkers])

  const markersOf = (recordingId: number) => markers.filter((m) => m.recording_id === recordingId)

  if (!detail) {
    return <div className="page">加载中…</div>
  }

  return (
    <div className="page">
      {message && <div className="toast success">{message}</div>}
      <div className="page-header">
        <button className="btn btn-plain" onClick={() => navigate('/')}>
          ← 返回列表
        </button>
        <h2>{detail.title}</h2>
        <StatusBadge status={detail.status} type="project" />
      </div>

      <section className="card">
        <div className="card-title">项目信息</div>
        <div className="detail-grid">
          <div>
            <div className="detail-label">受访者</div>
            <div className="detail-value">{detail.interviewee_name}</div>
          </div>
          <div>
            <div className="detail-label">出生年份</div>
            <div className="detail-value">{detail.birth_year}</div>
          </div>
          <div>
            <div className="detail-label">创建时间</div>
            <div className="detail-value">{formatDateTime(detail.created_at)}</div>
          </div>
          <div>
            <div className="detail-label">背景简介</div>
            <div className="detail-value">{detail.background || '-'}</div>
          </div>
        </div>
        <div className="row-actions" style={{ marginTop: 12 }}>
          {detail.status !== PROJECT_STATUS_ARCHIVED && (
            <button
              className="btn btn-primary btn-small"
              onClick={async () => {
                const next =
                  detail.status === PROJECT_STATUS_IN_PROGRESS ? PROJECT_STATUS_COMPLETED : PROJECT_STATUS_IN_PROGRESS
                await transitionStatus(projectId, next)
                setMessage('项目状态已更新')
                setTimeout(() => setMessage(''), 3000)
              }}
            >
              {detail.status === PROJECT_STATUS_IN_PROGRESS ? '标记为已完成' : '开始采访'}
            </button>
          )}
          <ConfirmDialog
            title="删除采访项目"
            message="确定删除该项目吗？此操作不可恢复。"
            danger
            confirmText="删除"
            onConfirm={async () => {
              await remove(projectId)
              navigate('/')
            }}
          >
            <button className="btn btn-danger btn-small">删除项目</button>
          </ConfirmDialog>
          <LinkToInterview projectId={projectId} />
        </div>
      </section>

      <section className="card">
        <div className="card-title">访谈提纲（版本与审核）</div>
        <OutlineEditor projectId={projectId} projectArchived={detail.status === PROJECT_STATUS_ARCHIVED} />
      </section>

      <section className="card">
        <div className="card-title">时间线 · 采访片段</div>
        {recordings.length === 0 ? (
          <EmptyState title="还没有录音片段" description="前往采访工作台开始录音，片段将按时间线展示" />
        ) : (
          <div className="timeline">
            {recordings.map((r) => (
              <TimelineItem key={r.id} recording={r} markers={markersOf(r.id)} onCreateMarker={createMarker} />
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

function LinkToInterview({ projectId }: { projectId: number }) {
  return (
    <a className="btn btn-plain btn-small" href={`#/interview?project_id=${projectId}`}>
      前往采访工作台
    </a>
  )
}

function TimelineItem({
  recording,
  markers,
  onCreateMarker,
}: {
  recording: Recording
  markers: TimelineMarker[]
  onCreateMarker: (payload: {
    project_id: number
    recording_id: number
    timestamp_second: number
    label: string
    note?: string
  }) => Promise<void>
}) {
  const [label, setLabel] = useState('')
  // 录音保留了当时的问题内容快照，提纲更新版本后历史展示不变。
  const questionText = recording.question_snapshot || `问题 #${recording.question_id}`

  return (
    <div className="timeline-item">
      <div className="timeline-dot" />
      <div className="timeline-content">
        <div className="timeline-head">
          <span className="timeline-q">问题：{questionText}</span>
          <StatusBadge status={recording.status} type="recording" />
          <span className="timeline-duration">{formatDuration(recording.duration_seconds)}</span>
        </div>
        <AudioPlayer recordingId={recording.id} durationSeconds={recording.duration_seconds} />
        <div className="timeline-summary">
          <span className="summary-label">一句话摘要：</span>
          {recording.summary || <span className="muted">暂无摘要</span>}
        </div>
        {markers.length > 0 && (
          <div className="marker-list">
            {markers.map((m) => (
              <span key={m.id} className="marker-chip">
                ⏱ {formatDuration(m.timestamp_second)} · {m.label}
                {m.note ? `（${m.note}）` : ''}
              </span>
            ))}
          </div>
        )}
        <div className="inline-form">
          <input
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="标注关键节点，如：回忆童年故居"
          />
          <button
            className="btn btn-plain btn-small"
            disabled={!label.trim()}
            onClick={async () => {
              await onCreateMarker({
                project_id: recording.project_id,
                recording_id: recording.id,
                timestamp_second: recording.duration_seconds > 0 ? Math.floor(recording.duration_seconds / 2) : 0,
                label: label.trim(),
              })
              setLabel('')
            }}
          >
            ＋ 标注节点
          </button>
        </div>
      </div>
    </div>
  )
}
