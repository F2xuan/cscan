import { describe, test, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/ai', () => ({
  getAIConfig: vi.fn()
}))

import { getAIConfig } from '@/api/ai'
import { chat, loadAIConfig, fetchModels, extractYamlBlock, DEFAULT_AI_CONFIG } from '@/utils/aiClient'

function mockFetch(payload, { ok = true, status = 200 } = {}) {
  const fetchMock = vi.fn().mockResolvedValue({
    ok,
    status,
    json: async () => payload,
    text: async () => (typeof payload === 'string' ? payload : JSON.stringify(payload))
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

const baseConfig = { baseUrl: 'http://ai.local', apiKey: 'k-123', model: 'm-1' }

describe('chat request construction', () => {
  beforeEach(() => vi.unstubAllGlobals())

  test('anthropic uses /v1/messages with x-api-key header', async () => {
    const fetchMock = mockFetch({ content: [{ text: 'hello' }] })
    const text = await chat({ config: { ...baseConfig, protocol: 'anthropic' }, prompt: 'p' })
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('http://ai.local/v1/messages')
    expect(init.headers['x-api-key']).toBe('k-123')
    expect(init.headers['anthropic-version']).toBe('2023-06-01')
    expect(JSON.parse(init.body).messages).toEqual([{ role: 'user', content: 'p' }])
    expect(text).toBe('hello')
  })

  test('openai uses /v1/chat/completions with Bearer header', async () => {
    const fetchMock = mockFetch({ choices: [{ message: { content: 'hi' } }] })
    const text = await chat({ config: { ...baseConfig, protocol: 'openai' }, prompt: 'p' })
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('http://ai.local/v1/chat/completions')
    expect(init.headers.Authorization).toBe('Bearer k-123')
    expect(text).toBe('hi')
  })

  test('openai baseUrl ending with /v1 avoids double /v1/v1/', async () => {
    const fetchMock = mockFetch({ choices: [{ message: { content: 'ok' } }] })
    await chat({ config: { baseUrl: 'https://host.example.com/v1', apiKey: 'k', protocol: 'openai' }, prompt: 'p' })
    expect(fetchMock.mock.calls[0][0]).toBe('https://host.example.com/v1/chat/completions')
  })

  test('trailing slash in baseUrl is normalized', async () => {
    const fetchMock = mockFetch({ content: [{ text: 'x' }] })
    await chat({ config: { ...baseConfig, baseUrl: 'http://ai.local/', protocol: 'anthropic' }, prompt: 'p' })
    expect(fetchMock.mock.calls[0][0]).toBe('http://ai.local/v1/messages')
  })
})

describe('chat error handling', () => {
  beforeEach(() => vi.unstubAllGlobals())

  test('throws NO_BASE_URL when baseUrl is empty', async () => {
    await expect(chat({ config: { protocol: 'anthropic' }, prompt: 'p' }))
      .rejects.toMatchObject({ code: 'NO_BASE_URL' })
  })

  test('throws NETWORK when fetch fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    await expect(chat({ config: { ...baseConfig, protocol: 'anthropic' }, prompt: 'p' }))
      .rejects.toMatchObject({ code: 'NETWORK' })
  })

  test('throws HTTP with status code on non-2xx', async () => {
    mockFetch('unauthorized', { ok: false, status: 401 })
    await expect(chat({ config: { ...baseConfig, protocol: 'anthropic' }, prompt: 'p' }))
      .rejects.toMatchObject({ code: 'HTTP', message: expect.stringContaining('401') })
  })

  test('AbortError is re-thrown as-is', async () => {
    const abortErr = new Error('aborted')
    abortErr.name = 'AbortError'
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(abortErr))
    await expect(chat({ config: { ...baseConfig, protocol: 'anthropic' }, prompt: 'p' }))
      .rejects.toMatchObject({ name: 'AbortError' })
  })
})

describe('fetchModels', () => {
  beforeEach(() => vi.unstubAllGlobals())

  test('fetches and sorts model list from OpenAI-compatible endpoint', async () => {
    mockFetch({ data: [{ id: 'gpt-4o' }, { id: 'gpt-3.5-turbo' }, { id: 'claude-3' }] })
    const models = await fetchModels({ ...baseConfig, protocol: 'openai' })
    expect(models).toEqual(['claude-3', 'gpt-3.5-turbo', 'gpt-4o'])
  })

  test('uses origin for models URL, stripping path from baseUrl', async () => {
    const fetchMock = mockFetch({ data: [{ id: 'm1' }] })
    await fetchModels({ baseUrl: 'https://host.example.com/anthropic', apiKey: 'k', protocol: 'openai' })
    expect(fetchMock.mock.calls[0][0]).toBe('https://host.example.com/v1/models')
  })

  test('uses Bearer auth for openai protocol', async () => {
    const fetchMock = mockFetch({ data: [{ id: 'm1' }] })
    await fetchModels({ ...baseConfig, protocol: 'openai' })
    const [, init] = fetchMock.mock.calls[0]
    expect(init.headers.Authorization).toBe('Bearer k-123')
  })

  test('uses x-api-key for anthropic protocol', async () => {
    const fetchMock = mockFetch({ data: [{ id: 'm1' }] })
    await fetchModels({ ...baseConfig, protocol: 'anthropic' })
    const [, init] = fetchMock.mock.calls[0]
    expect(init.headers['x-api-key']).toBe('k-123')
  })

  test('throws EMPTY when service returns no models', async () => {
    mockFetch({ data: [] })
    await expect(fetchModels(baseConfig)).rejects.toMatchObject({ code: 'EMPTY' })
  })

  test('throws NO_BASE_URL when baseUrl is missing', async () => {
    await expect(fetchModels({ protocol: 'openai' })).rejects.toMatchObject({ code: 'NO_BASE_URL' })
  })

  test('throws NETWORK on fetch failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('fail')))
    await expect(fetchModels(baseConfig)).rejects.toMatchObject({ code: 'NETWORK' })
  })
})

describe('loadAIConfig', () => {
  test('returns config from backend with field fallback', async () => {
    getAIConfig.mockResolvedValue({
      code: 0,
      data: { protocol: 'openai', baseUrl: 'http://x', apiKey: 'k', model: 'gpt-4o', updateTime: '2026-07-27 10:00:00' }
    })
    const config = await loadAIConfig()
    expect(config).toEqual({
      protocol: 'openai',
      baseUrl: 'http://x',
      apiKey: 'k',
      model: 'gpt-4o',
      updateTime: '2026-07-27 10:00:00'
    })
  })

  test('falls back to defaults on error', async () => {
    getAIConfig.mockRejectedValue(new Error('boom'))
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const config = await loadAIConfig()
    expect(config).toEqual({ ...DEFAULT_AI_CONFIG, updateTime: '' })
  })
})

describe('extractYamlBlock', () => {
  test('extracts content from ```yaml block', () => {
    expect(extractYamlBlock('desc\n```yaml\nid: demo\ninfo:\n```\nend')).toBe('id: demo\ninfo:')
  })

  test('strips residual markdown when content looks like YAML', () => {
    expect(extractYamlBlock('id: demo\ninfo:\n  name: x\n```')).toBe('id: demo\ninfo:\n  name: x')
  })

  test('returns empty string for null/empty input', () => {
    expect(extractYamlBlock('')).toBe('')
    expect(extractYamlBlock(null)).toBe('')
  })
})
