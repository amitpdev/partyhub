import type { PartySnapshot } from '../types'

export type AIProvider = 'openai' | 'anthropic' | 'gemini' | 'ollama' | 'compatible'
export interface AIDraft {
  provider: AIProvider
  model: string
  baseUrl: string
  apiKey: string
  rememberKey: boolean
}
export interface AISettings extends Omit<AIDraft, 'apiKey'> {
  hasKey: boolean
  configured: boolean
  secureStorageAvailable: boolean
}
export interface AIPick { party: PartySnapshot; queueId: string; title: string }

async function request<T>(path: string, method = 'GET', body?: AIDraft, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`/api/ai/${path}`, {
    method, signal,
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!response.ok) {
    const error = await response.json().catch(() => null) as { error?: string } | null
    throw new Error(error?.error || 'The AI service is unavailable. Check the local service and try again.')
  }
  return response.json() as Promise<T>
}
export const fetchAISettings = () => request<AISettings>('settings')
export const saveAISettings = (draft: AIDraft) => request<AISettings>('settings', 'POST', draft)
export const clearAISettings = () => request<AISettings>('settings', 'DELETE')
export const testAIConnection = (draft: AIDraft) => request<{ ok: boolean }>('test', 'POST', draft)
export const fetchAIModels = (draft: AIDraft) => request<{ models: string[] }>('models', 'POST', draft)
export const pickAISong = (signal?: AbortSignal) => request<AIPick>('pick', 'POST', undefined, signal)
