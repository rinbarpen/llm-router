import initSqlJs from 'sql.js'
import type { Database } from 'sql.js'
import type {
  InvocationRead,
  InvocationQuery,
  StatisticsResponse,
  TimeSeriesResponse,
  GroupedTimeSeriesResponse,
  TimeRangeStatistics,
  ModelStatistics,
  TimeSeriesDataPoint,
  GroupedTimeSeriesDataPoint,
} from './types'

// 从环境变量获取API基础URL
const getApiBaseUrl = () => {
  if (import.meta.env.DEV) {
    return '/api'
  }
  return import.meta.env.VITE_API_BASE_URL || '/api'
}

let db: Database | null = null
let dbLoadPromise: Promise<Database> | null = null

/**
 * 加载数据库文件
 */
async function loadDatabase(): Promise<Database> {
  if (db) {
    return db
  }

  if (dbLoadPromise) {
    return dbLoadPromise
  }

  dbLoadPromise = (async () => {
    try {
      // 初始化SQL.js
      const SQL = await initSqlJs({
        locateFile: (file: string) => {
          // 使用CDN或本地文件
          return `https://sql.js.org/dist/${file}`
        },
      })

      // 下载数据库文件
      const response = await fetch(`${getApiBaseUrl()}/monitor/database`)
      if (!response.ok) {
        throw new Error(`Failed to download database: ${response.statusText}`)
      }

      const arrayBuffer = await response.arrayBuffer()
      const uint8Array = new Uint8Array(arrayBuffer)

      // 打开数据库
      db = new SQL.Database(uint8Array)
      return db
    } catch (error) {
      console.error('Failed to load database:', error)
      dbLoadPromise = null
      throw error
    }
  })()

  return dbLoadPromise
}

/**
 * 重新加载数据库（用于刷新数据）
 */
export async function reloadDatabase(): Promise<void> {
  if (db) {
    db.close()
    db = null
  }
  dbLoadPromise = null
  await loadDatabase()
}

/**
 * 解析调用查询参数为SQL WHERE子句
 */
function buildWhereClause(query: Partial<InvocationQuery>): { sql: string; params: any[] } {
  const conditions: string[] = []
  const params: any[] = []

  if (query.model_id) {
    conditions.push('mi.model_id = ?')
    params.push(query.model_id)
  }
  if (query.provider_id) {
    conditions.push('mi.provider_id = ?')
    params.push(query.provider_id)
  }
  if (query.model_name) {
    conditions.push('mi.model_name = ?')
    params.push(query.model_name)
  }
  if (query.provider_name) {
    conditions.push('mi.provider_name = ?')
    params.push(query.provider_name)
  }
  if (query.status) {
    conditions.push('mi.status = ?')
    params.push(query.status)
  }
  if (query.start_time) {
    conditions.push('mi.started_at >= ?')
    params.push(query.start_time.toISOString())
  }
  if (query.end_time) {
    conditions.push('mi.started_at <= ?')
    params.push(query.end_time.toISOString())
  }

  const sql = conditions.length > 0 ? `WHERE ${conditions.join(' AND ')}` : ''
  return { sql, params }
}

export const dbService = {
  /**
   * 获取调用历史列表
   */
  async getInvocations(query: Partial<InvocationQuery> = {}): Promise<{
    items: InvocationRead[]
    total: number
    limit: number
    offset: number
  }> {
    const database = await loadDatabase()
    const { sql: whereClause, params } = buildWhereClause(query)

    // 获取总数
    const countSql = `
      SELECT COUNT(*) as total
      FROM monitor_invocations mi
      ${whereClause}
    `
    const countResult = database.exec(countSql, params)
    const total = countResult[0]?.values[0]?.[0] || 0

    // 获取分页数据
    const limit = query.limit || 100
    const offset = query.offset || 0
    const orderBy = query.order_by || 'started_at'
    const orderDesc = query.order_desc !== false

    const dataSql = `
      SELECT 
        mi.id,
        mi.model_id,
        mi.provider_id,
        mi.model_name,
        mi.provider_name,
        mi.started_at,
        mi.completed_at,
        mi.duration_ms,
        mi.status,
        mi.error_message,
        mi.request_prompt,
        mi.request_messages,
        mi.request_parameters,
        mi.response_text,
        mi.response_text_length,
        mi.prompt_tokens,
        mi.completion_tokens,
        mi.total_tokens,
        mi.cost,
        mi.raw_response,
        mi.created_at
      FROM monitor_invocations mi
      ${whereClause}
      ORDER BY mi.${orderBy} ${orderDesc ? 'DESC' : 'ASC'}
      LIMIT ? OFFSET ?
    `

    const result = database.exec(dataSql, [...params, limit, offset])
    if (!result[0]) {
      return { items: [], total: Number(total), limit, offset }
    }

    const columns = result[0].columns
    const values = result[0].values

    const items: InvocationRead[] = values.map((row) => {
      const item: any = {}
      columns.forEach((col, idx) => {
        const value = row[idx]
        if (col === 'request_messages' || col === 'request_parameters' || col === 'raw_response') {
          item[col] = value ? JSON.parse(value as string) : null
        } else {
          item[col] = value
        }
      })
      return item as InvocationRead
    })

    return { items, total: Number(total), limit, offset }
  },

  /**
   * 获取单次调用详情
   */
  async getInvocationById(id: number): Promise<InvocationRead | null> {
    const database = await loadDatabase()

    const sql = `
      SELECT 
        mi.id,
        mi.model_id,
        mi.provider_id,
        mi.model_name,
        mi.provider_name,
        mi.started_at,
        mi.completed_at,
        mi.duration_ms,
        mi.status,
        mi.error_message,
        mi.request_prompt,
        mi.request_messages,
        mi.request_parameters,
        mi.response_text,
        mi.response_text_length,
        mi.prompt_tokens,
        mi.completion_tokens,
        mi.total_tokens,
        mi.cost,
        mi.raw_response,
        mi.created_at
      FROM monitor_invocations mi
      WHERE mi.id = ?
    `

    const result = database.exec(sql, [id])
    if (!result[0] || result[0].values.length === 0) {
      return null
    }

    const columns = result[0].columns
    const row = result[0].values[0]

    const item: any = {}
    columns.forEach((col, idx) => {
      const value = row[idx]
      if (col === 'request_messages' || col === 'request_parameters' || col === 'raw_response') {
        item[col] = value ? JSON.parse(value as string) : null
      } else {
        item[col] = value
      }
    })

    return item as InvocationRead
  },

  /**
   * 获取统计信息
   */
  async getStatistics(timeRangeHours: number = 24, limit: number = 10): Promise<StatisticsResponse> {
    const database = await loadDatabase()

    const startTime = new Date(Date.now() - timeRangeHours * 60 * 60 * 1000).toISOString()

    // 总体统计
    const overallSql = `
      SELECT 
        COUNT(*) as total_calls,
        SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_calls,
        SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as error_calls,
        SUM(total_tokens) as total_tokens,
        AVG(duration_ms) as avg_duration_ms,
        SUM(cost) as total_cost
      FROM monitor_invocations
      WHERE started_at >= ?
    `

    const overallResult = database.exec(overallSql, [startTime])
    const overallRow = overallResult[0]?.values[0] || [0, 0, 0, 0, null, null]

    const totalCalls = Number(overallRow[0]) || 0
    const successCalls = Number(overallRow[1]) || 0
    const errorCalls = Number(overallRow[2]) || 0
    const successRate = totalCalls > 0 ? (successCalls / totalCalls) * 100 : 0

    const overall: TimeRangeStatistics = {
      time_range: `${timeRangeHours}h`,
      total_calls: totalCalls,
      success_calls: successCalls,
      error_calls: errorCalls,
      success_rate: Math.round(successRate * 100) / 100,
      total_tokens: Number(overallRow[3]) || 0,
      avg_duration_ms: overallRow[4] ? Math.round(Number(overallRow[4]) * 100) / 100 : null,
      total_cost: overallRow[5] ? Math.round(Number(overallRow[5]) * 1000000) / 1000000 : null,
    }

    // 按模型统计
    const modelStatsSql = `
      SELECT 
        mi.model_id,
        mi.model_name,
        mi.provider_name,
        COUNT(*) as total_calls,
        SUM(CASE WHEN mi.status = 'success' THEN 1 ELSE 0 END) as success_calls,
        SUM(CASE WHEN mi.status = 'error' THEN 1 ELSE 0 END) as error_calls,
        SUM(mi.total_tokens) as total_tokens,
        SUM(mi.prompt_tokens) as prompt_tokens,
        SUM(mi.completion_tokens) as completion_tokens,
        AVG(mi.duration_ms) as avg_duration_ms,
        SUM(mi.duration_ms) as total_duration_ms,
        SUM(mi.cost) as total_cost
      FROM monitor_invocations mi
      WHERE mi.started_at >= ?
      GROUP BY mi.model_id, mi.model_name, mi.provider_name
      ORDER BY total_calls DESC
      LIMIT ?
    `

    const modelStatsResult = database.exec(modelStatsSql, [startTime, limit])
    const byModel: ModelStatistics[] = []

    if (modelStatsResult[0]) {
      const columns = modelStatsResult[0].columns
      modelStatsResult[0].values.forEach((row) => {
        const modelTotal = Number(row[3]) || 0
        const modelSuccess = Number(row[4]) || 0
        const modelSuccessRate = modelTotal > 0 ? (modelSuccess / modelTotal) * 100 : 0

        byModel.push({
          model_id: Number(row[0]),
          model_name: row[1] as string,
          provider_name: row[2] as string,
          total_calls: modelTotal,
          success_calls: modelSuccess,
          error_calls: Number(row[5]) || 0,
          success_rate: Math.round(modelSuccessRate * 100) / 100,
          total_tokens: Number(row[6]) || 0,
          prompt_tokens: Number(row[7]) || 0,
          completion_tokens: Number(row[8]) || 0,
          avg_duration_ms: row[9] ? Math.round(Number(row[9]) * 100) / 100 : null,
          total_duration_ms: Number(row[10]) || 0,
          total_cost: row[11] ? Math.round(Number(row[11]) * 1000000) / 1000000 : null,
        })
      })
    }

    // 最近的错误
    const errorSql = `
      SELECT 
        mi.id,
        mi.model_id,
        mi.provider_id,
        mi.model_name,
        mi.provider_name,
        mi.started_at,
        mi.completed_at,
        mi.duration_ms,
        mi.status,
        mi.error_message,
        mi.request_prompt,
        mi.request_messages,
        mi.request_parameters,
        mi.response_text,
        mi.response_text_length,
        mi.prompt_tokens,
        mi.completion_tokens,
        mi.total_tokens,
        mi.cost,
        mi.raw_response,
        mi.created_at
      FROM monitor_invocations mi
      WHERE mi.status = 'error' AND mi.started_at >= ?
      ORDER BY mi.started_at DESC
      LIMIT 5
    `

    const errorResult = database.exec(errorSql, [startTime])
    const recentErrors: InvocationRead[] = []

    if (errorResult[0]) {
      const columns = errorResult[0].columns
      errorResult[0].values.forEach((row) => {
        const item: any = {}
        columns.forEach((col, idx) => {
          const value = row[idx]
          if (col === 'request_messages' || col === 'request_parameters' || col === 'raw_response') {
            item[col] = value ? JSON.parse(value as string) : null
          } else {
            item[col] = value
          }
        })
        recentErrors.push(item as InvocationRead)
      })
    }

    return {
      overall,
      by_model: byModel,
      recent_errors: recentErrors,
    }
  },

  /**
   * 获取时间序列数据
   */
  async getTimeSeries(
    granularity: 'hour' | 'day' | 'week' | 'month' = 'day',
    timeRangeHours: number = 168
  ): Promise<TimeSeriesResponse> {
    const database = await loadDatabase()

    const startTime = new Date(Date.now() - timeRangeHours * 60 * 60 * 1000).toISOString()

    let timeFormat = ''
    switch (granularity) {
      case 'hour':
        timeFormat = "strftime('%Y-%m-%d %H:00:00', started_at)"
        break
      case 'day':
        timeFormat = "strftime('%Y-%m-%d 00:00:00', started_at)"
        break
      case 'week':
        timeFormat = "strftime('%Y-W%W', started_at)"
        break
      case 'month':
        timeFormat = "strftime('%Y-%m-01 00:00:00', started_at)"
        break
    }

    const sql = `
      SELECT 
        ${timeFormat} as time_bucket,
        COUNT(*) as total_calls,
        SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_calls,
        SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as error_calls,
        SUM(total_tokens) as total_tokens,
        SUM(prompt_tokens) as prompt_tokens,
        SUM(completion_tokens) as completion_tokens
      FROM monitor_invocations
      WHERE started_at >= ?
      GROUP BY time_bucket
      ORDER BY time_bucket
    `

    const result = database.exec(sql, [startTime])
    const dataPoints: TimeSeriesDataPoint[] = []

    if (result[0]) {
      result[0].values.forEach((row) => {
        const timeStr = row[0] as string
        let timestamp: Date

        if (granularity === 'week') {
          // 处理周格式
          const [year, week] = timeStr.split('-W')
          const jan1 = new Date(Number(year), 0, 1)
          const daysOffset = (Number(week) - 1) * 7
          const jan1Weekday = jan1.getDay()
          const daysToMonday = (7 - jan1Weekday) % 7
          timestamp = new Date(jan1.getTime() + (daysOffset + daysToMonday) * 24 * 60 * 60 * 1000)
        } else {
          timestamp = new Date(timeStr)
        }

        dataPoints.push({
          timestamp,
          total_calls: Number(row[1]) || 0,
          success_calls: Number(row[2]) || 0,
          error_calls: Number(row[3]) || 0,
          total_tokens: Number(row[4]) || 0,
          prompt_tokens: Number(row[5]) || 0,
          completion_tokens: Number(row[6]) || 0,
        })
      })
    }

    return {
      granularity,
      data: dataPoints,
    }
  },

  /**
   * 获取分组时间序列数据
   */
  async getGroupedTimeSeries(
    groupBy: 'model' | 'provider',
    granularity: 'hour' | 'day' | 'week' | 'month' = 'day',
    timeRangeHours: number = 168
  ): Promise<GroupedTimeSeriesResponse> {
    const database = await loadDatabase()

    const startTime = new Date(Date.now() - timeRangeHours * 60 * 60 * 1000).toISOString()

    let timeFormat = ''
    switch (granularity) {
      case 'hour':
        timeFormat = "strftime('%Y-%m-%d %H:00:00', mi.started_at)"
        break
      case 'day':
        timeFormat = "strftime('%Y-%m-%d 00:00:00', mi.started_at)"
        break
      case 'week':
        timeFormat = "strftime('%Y-W%W', mi.started_at)"
        break
      case 'month':
        timeFormat = "strftime('%Y-%m-01 00:00:00', mi.started_at)"
        break
    }

    const groupColumn = groupBy === 'model' ? 'mi.model_name' : 'mi.provider_name'

    const sql = `
      SELECT 
        ${timeFormat} as time_bucket,
        ${groupColumn} as group_name,
        COUNT(*) as total_calls,
        SUM(CASE WHEN mi.status = 'success' THEN 1 ELSE 0 END) as success_calls,
        SUM(CASE WHEN mi.status = 'error' THEN 1 ELSE 0 END) as error_calls,
        SUM(mi.total_tokens) as total_tokens,
        SUM(mi.prompt_tokens) as prompt_tokens,
        SUM(mi.completion_tokens) as completion_tokens
      FROM monitor_invocations mi
      WHERE mi.started_at >= ?
      GROUP BY time_bucket, group_name
      ORDER BY time_bucket, group_name
    `

    const result = database.exec(sql, [startTime])
    const dataPoints: GroupedTimeSeriesDataPoint[] = []

    if (result[0]) {
      result[0].values.forEach((row) => {
        const timeStr = row[0] as string
        let timestamp: Date

        if (granularity === 'week') {
          const [year, week] = timeStr.split('-W')
          const jan1 = new Date(Number(year), 0, 1)
          const daysOffset = (Number(week) - 1) * 7
          const jan1Weekday = jan1.getDay()
          const daysToMonday = (7 - jan1Weekday) % 7
          timestamp = new Date(jan1.getTime() + (daysOffset + daysToMonday) * 24 * 60 * 60 * 1000)
        } else {
          timestamp = new Date(timeStr)
        }

        dataPoints.push({
          timestamp,
          group_name: row[1] as string,
          total_calls: Number(row[2]) || 0,
          success_calls: Number(row[3]) || 0,
          error_calls: Number(row[4]) || 0,
          total_tokens: Number(row[5]) || 0,
          prompt_tokens: Number(row[6]) || 0,
          completion_tokens: Number(row[7]) || 0,
        })
      })
    }

    return {
      granularity,
      group_by: groupBy,
      data: dataPoints,
    }
  },
}

