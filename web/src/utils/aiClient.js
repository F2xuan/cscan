import { getAIConfig } from '@/api/ai'

// Platform-wide AI call entry: all modules needing AI capability import from here.
// Config is maintained on the /ai-config page.

export const DEFAULT_AI_CONFIG = {
  protocol: 'openai',
  baseUrl: '',
  apiKey: '',
  model: ''
}

export const AI_PROTOCOLS = [
  { value: 'openai', label: 'OpenAI', hint: 'Compatible: /v1/chat/completions, /v1/models' },
  { value: 'anthropic', label: 'Anthropic', hint: 'Anthropic: /v1/messages' }
]

function aiError(code, message) {
  const err = new Error(message)
  err.code = code
  return err
}

export function protocolHint(protocol) {
  const item = AI_PROTOCOLS.find(p => p.value === protocol)
  return item ? item.hint : ''
}

// Load AI config from backend, fallback to defaults on missing fields
export async function loadAIConfig() {
  try {
    const res = await getAIConfig()
    if (res?.code === 0 && res.data) {
      return {
        protocol: res.data.protocol || DEFAULT_AI_CONFIG.protocol,
        baseUrl: res.data.baseUrl || '',
        apiKey: res.data.apiKey || '',
        model: res.data.model || '',
        updateTime: res.data.updateTime || ''
      }
    }
  } catch (e) {
    console.error('[aiClient] Failed to load AI config:', e)
  }
  return { ...DEFAULT_AI_CONFIG, updateTime: '' }
}

// Build request based on protocol
// OpenAI: baseUrl may already include /v1 (e.g. https://host/v1), avoid double /v1/v1/
// Anthropic: baseUrl typically ends with /anthropic, append /v1/messages
function buildRequest(config, prompt, maxTokens) {
  const base = (config.baseUrl || '').replace(/\/+$/, '')
  const messages = [{ role: 'user', content: prompt }]

  if (config.protocol === 'openai') {
    const suffix = /\/v1$/i.test(base) ? '/chat/completions' : '/v1/chat/completions'
    return {
      url: `${base}${suffix}`,
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${config.apiKey}` },
      body: { model: config.model, max_tokens: maxTokens, messages }
    }
  }

  // Anthropic
  return {
    url: `${base}/v1/messages`,
    headers: {
      'Content-Type': 'application/json',
      'x-api-key': config.apiKey,
      'anthropic-version': '2023-06-01'
    },
    body: { model: config.model, max_tokens: maxTokens, messages }
  }
}

function extractText(protocol, data) {
  if (protocol === 'openai') {
    return data?.choices?.[0]?.message?.content || ''
  }
  // Anthropic
  return (data?.content || []).map(b => b.text || '').join('')
}

// Chat completion, returns plain text from model
export async function chat({ config, prompt, maxTokens = 4096, signal }) {
  if (!config?.baseUrl) {
    throw aiError('NO_BASE_URL', 'AI service address not configured')
  }

  const { url, headers, body } = buildRequest(config, prompt, maxTokens)

  let response
  try {
    response = await fetch(url, { method: 'POST', headers, body: JSON.stringify(body), signal })
  } catch (e) {
    if (e.name === 'AbortError') throw e
    throw aiError('NETWORK', e.message)
  }

  if (!response.ok) {
    const detail = await response.text().catch(() => '')
    throw aiError('HTTP', `${response.status} ${detail.substring(0, 200)}`)
  }

  return extractText(config.protocol, await response.json())
}

// Lightweight connectivity probe
export function testConnection(config) {
  return chat({ config, prompt: 'Hi', maxTokens: 10 })
}

// Fetch available models from the service
// OpenAI-compatible: GET /v1/models
// Models endpoint is always at the origin root, not under the chat path
// e.g. baseUrl "https://host/anthropic" → models at "https://host/v1/models"
export async function fetchModels(config) {
  if (!config?.baseUrl) {
    throw aiError('NO_BASE_URL', 'AI service address not configured')
  }

  const origin = new URL(config.baseUrl).origin
  const headers = { 'Content-Type': 'application/json' }

  if (config.protocol === 'openai') {
    headers['Authorization'] = `Bearer ${config.apiKey}`
  } else {
    headers['x-api-key'] = config.apiKey
    headers['anthropic-version'] = '2023-06-01'
  }

  let response
  try {
    response = await fetch(`${origin}/v1/models`, { method: 'GET', headers })
  } catch (e) {
    throw aiError('NETWORK', e.message)
  }

  if (!response.ok) {
    const detail = await response.text().catch(() => '')
    throw aiError('HTTP', `${response.status} ${detail.substring(0, 200)}`)
  }

  const data = await response.json()

  // OpenAI format: { data: [{ id: "model-name", ... }] }
  const models = (data?.data || []).map(m => m.id).filter(Boolean)
  if (models.length === 0) {
    throw aiError('EMPTY', 'No models returned from service')
  }

  return models.sort()
}

// Extract YAML code block from model output
export function extractYamlBlock(text) {
  const content = text || ''
  const matched = content.match(/```ya?ml\n([\s\S]*?)```/)
  if (matched) {
    return matched[1].trim()
  }
  if (content.includes('id:') && content.includes('info:')) {
    return content.replace(/```ya?ml\n?/g, '').replace(/```\n?/g, '').trim()
  }
  return content.trim()
}
