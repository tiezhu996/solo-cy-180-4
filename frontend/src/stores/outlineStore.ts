// 访谈提纲版本与审核状态管理。
import { create } from 'zustand'
import {
  createOutlineDraft,
  deleteOutlineQuestion,
  getOutlineVersion,
  listOutlineVersions,
  listPendingOutlines,
  reviewOutline,
  saveOutlineQuestions,
  submitOutlineVersion,
  type SaveOutlineQuestion,
} from '../api/outline'
import type { OutlineVersion } from '../api/types'

interface OutlineState {
  versions: OutlineVersion[]
  pending: OutlineVersion[]
  pendingTotal: number
  loading: boolean
  fetchByProject: (projectId: number) => Promise<OutlineVersion[]>
  fetchVersion: (projectId: number, versionId: number) => Promise<OutlineVersion>
  fetchPending: (page?: number, pageSize?: number) => Promise<void>
  ensureDraft: (projectId: number, baseVersionId?: number) => Promise<OutlineVersion>
  saveQuestions: (projectId: number, versionId: number, questions: SaveOutlineQuestion[]) => Promise<OutlineVersion>
  removeQuestion: (projectId: number, versionId: number, questionId: number) => Promise<void>
  submit: (projectId: number, versionId: number) => Promise<OutlineVersion>
  review: (versionId: number, action: 'approve' | 'reject', reason?: string) => Promise<OutlineVersion>
}

export const useOutlineStore = create<OutlineState>((set) => ({
  versions: [],
  pending: [],
  pendingTotal: 0,
  loading: false,

  async fetchByProject(projectId) {
    set({ loading: true })
    try {
      const res = await listOutlineVersions(projectId)
      set({ versions: res.list, loading: false })
      return res.list
    } catch (e) {
      console.error('fetch outline versions failed', e)
      set({ loading: false })
      throw e
    }
  },

  async fetchVersion(projectId, versionId) {
    return getOutlineVersion(projectId, versionId)
  },

  async fetchPending(page = 1, pageSize = 20) {
    set({ loading: true })
    try {
      const res = await listPendingOutlines({ page, page_size: pageSize })
      set({ pending: res.list, pendingTotal: res.total, loading: false })
    } catch (e) {
      console.error('fetch pending outlines failed', e)
      set({ loading: false })
    }
  },

  // 采访员保存改动时生成草稿：后端在已有 draft/rejected 草稿时复用，否则创建新版本。
  async ensureDraft(projectId, baseVersionId = 0) {
    return createOutlineDraft(projectId, baseVersionId)
  },

  async saveQuestions(projectId, versionId, questions) {
    const version = await saveOutlineQuestions(projectId, versionId, questions)
    set((s) => ({
      versions: s.versions.map((v) => (v.id === versionId ? { ...v, ...version } : v)),
    }))
    return version
  },

  async removeQuestion(projectId, versionId, questionId) {
    await deleteOutlineQuestion(projectId, versionId, questionId)
  },

  async submit(projectId, versionId) {
    const version = await submitOutlineVersion(projectId, versionId)
    set((s) => ({
      versions: s.versions.map((v) => (v.id === versionId ? { ...v, ...version } : v)),
    }))
    return version
  },

  async review(versionId, action, reason) {
    const version = await reviewOutline(versionId, { action, reason })
    set((s) => ({
      pending: s.pending.filter((v) => v.id !== versionId),
      pendingTotal: Math.max(0, s.pendingTotal - 1),
      versions: s.versions.map((v) => (v.id === versionId ? { ...v, ...version } : v)),
    }))
    return version
  },
}))
