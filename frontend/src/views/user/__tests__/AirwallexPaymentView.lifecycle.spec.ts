import { describe, expect, it } from 'vitest'
import router from '@/router'

// Airwallex SDK 已停用；旧入口不得重新启动支付，历史结果页继续可达。
describe('Airwallex 停用后的路由合同', () => {
  it('旧支付地址进入未找到页面，保留原有订单查询参数', () => {
    const resolved = router.resolve('/payment/airwallex?order_id=101&resume_token=resume-awx')

    expect(resolved.name).toBe('NotFound')
    expect(resolved.query).toEqual({ order_id: '101', resume_token: 'resume-awx' })
  })

  it('旧命名入口无法解析为可启动的支付页面', () => {
    expect(() => router.resolve({ name: 'AirwallexPayment', query: { order_id: '101' } })).toThrow()
  })

  it('历史订单结果地址仍可解析并保留查询凭据', () => {
    const resolved = router.resolve('/payment/result?out_trade_no=sub2_awx_101&resume_token=resume-awx')

    expect(resolved.name).toBe('PaymentResult')
    expect(resolved.query).toEqual({ out_trade_no: 'sub2_awx_101', resume_token: 'resume-awx' })
  })
})
