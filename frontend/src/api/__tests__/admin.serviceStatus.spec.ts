import { afterEach, describe, expect, it } from 'vitest'
import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { AxiosError } from 'axios'
import { apiClient } from '../client'
import { getConfig, getSnapshot, updateConfig, type ServiceStatusConfig } from '../admin/serviceStatus'

const originalAdapter = apiClient.defaults.adapter
const config: ServiceStatusConfig = {
  version: 4, enabled: false, platforms: ['openai'], minimum_samples: 5,
  warning_error_rate: 0.05, outage_error_rate: 0.9, abnormal_windows: 2, recovery_windows: 3
}
afterEach(() => { apiClient.defaults.adapter = originalAdapter })
function installAdapter(handler: (request: InternalAxiosRequestConfig) => unknown) {
  apiClient.defaults.adapter = (async request => ({
    status: 200, statusText: 'OK', headers: {}, config: request,
    data: { code: 0, data: await handler(request) }
  })) as AxiosAdapter
}

describe('独立服务状态 API 的实际 Axios 合同', () => {
  it('默认与单平台快照仅发送 range/platform，不注入 timezone 或主体范围', async () => {
    const urls: string[] = []
    installAdapter(request => { urls.push(apiClient.getUri(request)); return { health: 'unknown' } })
    await getSnapshot()
    await getSnapshot({ range: '7d', platform: 'openai' })
    expect(urls[0]).toMatch(/\/admin\/service-status\/snapshot\?range=24h$/)
    expect(urls[1]).toMatch(/\/admin\/service-status\/snapshot\?range=7d&platform=openai$/)
  })

  it('disabled 配置可读，完整 PUT 保留版本及全部可编辑字段', async () => {
    let written: unknown
    installAdapter(request => {
      if (request.method === 'put') written = JSON.parse(request.data)
      else expect(apiClient.getUri(request)).toMatch(/\/admin\/service-status\/config$/)
      return config
    })
    expect(await getConfig()).toEqual(config)
    expect(await updateConfig(config)).toEqual(config)
    expect(written).toEqual(config)
  })

  it('取消已发出的快照时，实际 Axios 拒绝返回晚到数据', async () => {
    let finish!: () => void
    installAdapter(async () => { await new Promise<void>(resolve => { finish = resolve }); return { health: 'operational' } })
    const controller = new AbortController()
    const pending = getSnapshot({}, controller.signal)
    // 等待请求拦截器及 adapter 启动。
    await Promise.resolve(); await Promise.resolve()
    controller.abort()
    finish()
    await expect(pending).rejects.toMatchObject({ code: 'ERR_CANCELED' })
  })

  it('保留服务器 503 数值 code 与固定 reason，供页面辨识停用', async () => {
    apiClient.defaults.adapter = async request => {
      throw new AxiosError('disabled', undefined, request, undefined, {
        status: 503, statusText: 'Service Unavailable', config: request, headers: {},
        data: { code: 503, reason: 'SERVICE_STATUS_DISABLED', message: 'disabled' }
      })
    }
    await expect(getSnapshot()).rejects.toMatchObject({ status: 503, code: 503, reason: 'SERVICE_STATUS_DISABLED' })
  })
})
