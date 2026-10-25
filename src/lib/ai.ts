import type { PartySnapshot } from '../types'

export type AIProvider = 'chatgpt' | 'openai' | 'anthropic' | 'gemini' | 'ollama' | 'compatible'
export interface AIDraft {
  accountId?: string
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

async function request<T>(path: string, method = 'GET', body?: object, signal?: AbortSignal): Promise<T> {
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
export const fetchAIModels = (draft: AIDraft, signal?: AbortSignal) => request<{ models: string[] }>('models', 'POST', draft, signal)
export const pickAISong = (signal?: AbortSignal) => request<AIPick>('pick', 'POST', undefined, signal)

export interface ChatGPTAccount { id: string; label: string; connected: boolean; remembered: boolean; welcomed: boolean }
export interface ChatGPTStatus { accounts: ChatGPTAccount[]; login: { status: 'idle' | 'pending' | 'processing' | 'success' | 'error'; message?: string; accountId?: string } }
export interface ModelChoice { id: string; name: string }
export const fetchChatGPTStatus = () => request<ChatGPTStatus>('chatgpt')
export const startChatGPTLogin = (accountId: string, remember: boolean) => request<{ url: string }>('chatgpt/login', 'POST', { accountId, remember })
export const cancelChatGPTLogin = () => request<{ ok: boolean }>('chatgpt/login', 'DELETE')
export const welcomeChatGPT = (accountId: string) => request<{ ok: boolean }>('chatgpt/welcome', 'POST', { accountId })
export const signOutChatGPT = (accountId: string) => request<{ warning: string }>('chatgpt/signout', 'POST', { accountId })
export const fetchChatGPTModels = (accountId: string, signal?: AbortSignal) => request<{ models: ModelChoice[] }>('chatgpt/models', 'POST', { accountId }, signal)
