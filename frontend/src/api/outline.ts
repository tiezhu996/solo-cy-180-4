import { del, get, post, put } from '../utils/request'
import type { OutlineVersion, Paged } from './types'

export interface SaveOutlineQuestion {
  question_id?: number
  content: string
  sort_order?: number
}

// 项目下的提纲版本列表（版本查看）。
export function listOutlineVersions(projectId: number) {
  return get<{ list: OutlineVersion[] }>(`/projects/${projectId}/outline-versions`)
}

// 查看单个提纲版本详情（含问题内容）。
export function getOutlineVersion(projectId: number, versionId: number) {
  return get<OutlineVersion>(`/projects/${projectId}/outline-versions/${versionId}`)
}

// 采访员保存改动前创建（或复用）草稿版本；baseVersionId 传已通过版本 ID 可复制其问题。
export function createOutlineDraft(projectId: number, baseVersionId = 0) {
  return post<OutlineVersion>(`/projects/${projectId}/outline-versions`, {
    base_version_id: baseVersionId,
  })
}

// 保存草稿改动（新增/更新问题，question_id 省略或为 0 表示新增）。
export function saveOutlineQuestions(projectId: number, versionId: number, questions: SaveOutlineQuestion[]) {
  return put<OutlineVersion>(`/projects/${projectId}/outline-versions/${versionId}/questions`, { questions })
}

// 删除草稿版本中的单个问题。
export function deleteOutlineQuestion(projectId: number, versionId: number, questionId: number) {
  return del<null>(`/projects/${projectId}/outline-versions/${versionId}/questions/${questionId}`)
}

// 提交提纲版本审核；重复提交会被后端拒绝。
export function submitOutlineVersion(projectId: number, versionId: number) {
  return post<OutlineVersion>(`/projects/${projectId}/outline-versions/${versionId}/submit`)
}

// 档案员/管理员：待审核提纲列表。
export function listPendingOutlines(params: { page?: number; page_size?: number; project_id?: number } = {}) {
  return get<Paged<OutlineVersion>>('/outlines/pending', params as Record<string, unknown>)
}

// 档案员/管理员审核：通过或退回（退回必须填写原因）。
export function reviewOutline(
  versionId: number,
  payload: { action: 'approve' | 'reject'; reason?: string },
) {
  return post<OutlineVersion>(`/outlines/${versionId}/review`, payload)
}
