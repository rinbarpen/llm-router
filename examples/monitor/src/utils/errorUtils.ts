/**
 * 从 API 错误中提取展示用文案（通用）
 */
export function getApiErrorMessage(error: unknown, fallback: string): string {
  const err = error as { response?: { data?: { detail?: unknown } }; message?: string }
  const detail = err?.response?.data?.detail
  if (detail != null && typeof detail === 'string') return detail
  if (err?.message && typeof err.message === 'string') return err.message
  return fallback
}

const UNAUTH_MSG = '未认证，请配置 API Key 或先登录'

/**
 * 定价相关接口错误文案（含 401 特殊提示）
 */
export function getPricingErrorMessage(error: unknown, fallback: string): string {
  const err = error as { response?: { status?: number; data?: { detail?: unknown } }; message?: string }
  const status = err?.response?.status
  const detail = err?.response?.data?.detail
  if (status === 401) {
    return typeof detail === 'string' ? detail : UNAUTH_MSG
  }
  if (detail != null && typeof detail === 'string') return detail
  if (err?.message && typeof err.message === 'string') return err.message
  return fallback
}
