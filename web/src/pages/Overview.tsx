import { useEffect, useState } from 'react'
import { API, Overview as OV, Point, ActiveAlert, Unauthorized } from '../api'
import UsageBar, { fmtBytes } from '../components/UsageBar'
import Chart from '../components/Chart'

type Period = 'today' | '7d' | '30d'
const PERIODS: { key: Period; label: string }[] = [
  { key: 'today', label: '今日' },
  { key: '7d', label: '近 7 天' },
  { key: '30d', label: '近 30 天' },
]

// periodFrom 返回所选周期的起始 unix 秒。
function periodFrom(p: Period): number {
  if (p === 'today') {
    const d = new Date()
    d.setHours(0, 0, 0, 0)
    return Math.floor(d.getTime() / 1000)
  }
  const days = p === '7d' ? 7 : 30
  return Math.floor(Date.now() / 1000) - days * 86400
}

type XY = [number[], number[]]
type NodeSeries = { cpu: XY; load: XY; net: XY }
const toXY = (pts: Point[]): XY => [pts.map((p) => Date.parse(p.TS) / 1000), pts.map((p) => p.Value)]

type MetricKey = 'cpu' | 'net' | 'load'
const CARD_METRICS: { key: MetricKey; label: string; fmt: (v: number | null) => string }[] = [
  { key: 'cpu', label: 'CPU', fmt: (v) => (v == null ? '—' : v.toFixed(2) + ' 核') },
  { key: 'net', label: '网络', fmt: (v) => (v == null ? '—' : fmtBytes(v) + '/s') },
  { key: 'load', label: '负载', fmt: (v) => (v == null ? '—' : v.toFixed(2)) },
]

export default function Overview({ onAuthError }: { onAuthError: () => void }) {
  const [ov, setOv] = useState<OV | null>(null)
  const [alerts, setAlerts] = useState<ActiveAlert[]>([])
  const [series, setSeries] = useState<Record<string, NodeSeries>>({})
  const [traffic, setTraffic] = useState<Record<string, { rx: number; tx: number }>>({})
  const [period, setPeriod] = useState<Period>('30d')
  const [chartTab, setChartTab] = useState<Record<string, MetricKey>>({})
  const [err, setErr] = useState('')

  const nodeKey = (ov?.nodes.map((n) => n.name).sort().join(',')) || ''
  const periodLabel = PERIODS.find((p) => p.key === period)?.label ?? ''

  // 数字 + 告警：5s 刷新（对象状态变化快，保持响应）。
  useEffect(() => {
    let alive = true
    async function tick() {
      try {
        const [o, a] = await Promise.all([API.overview(), API.active()])
        if (!alive) return
        setOv(o)
        setAlerts(a)
        setErr('')
      } catch (e: any) {
        if (e instanceof Unauthorized) { onAuthError(); return }
        setErr(e.message)
      }
    }
    tick()
    const id = setInterval(tick, 5000)
    return () => { alive = false; clearInterval(id) }
  }, [])

  // 时序图（CPU/负载/网络）+ 流量累计：15s 刷新（对齐采集间隔）。
  useEffect(() => {
    if (!nodeKey) return
    const names = nodeKey.split(',')
    let alive = true

    async function loadSeries() {
      const results = await Promise.all(names.map(async (name) => {
        const [cpu, load, rx, tx] = await Promise.all([
          API.query('node', name, 'cpu').catch(() => [] as Point[]),
          API.query('node', name, 'load1').catch(() => [] as Point[]),
          API.query('node', name, 'net_rx').catch(() => [] as Point[]),
          API.query('node', name, 'net_tx').catch(() => [] as Point[]),
        ])
        // 上下行合并为总吞吐（同一批样本、时间戳对齐）
        const n = Math.min(rx.length, tx.length)
        const net: Point[] = []
        for (let i = 0; i < n; i++) net.push({ TS: rx[i].TS, Value: rx[i].Value + tx[i].Value })
        return { name, s: { cpu: toXY(cpu), load: toXY(load), net: toXY(net) } as NodeSeries }
      }))
      if (!alive) return
      const m: Record<string, NodeSeries> = {}
      for (const r of results) m[r.name] = r.s
      setSeries(m)
    }

    async function loadTraffic() {
      const from = periodFrom(period)
      const results = await Promise.all(names.map((name) =>
        API.traffic('node', name, from).then((t) => [name, t] as const).catch(() => [name, { rx: 0, tx: 0 }] as const),
      ))
      if (!alive) return
      const m: Record<string, { rx: number; tx: number }> = {}
      for (const [name, t] of results) m[name] = t
      setTraffic(m)
    }

    loadSeries()
    loadTraffic()
    const id = setInterval(() => { loadSeries(); loadTraffic() }, 15000)
    return () => { alive = false; clearInterval(id) }
  }, [nodeKey, period])

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <h2 className="page-title">总览</h2>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: 'var(--muted)' }}>
          <span>流量统计</span>
          <select value={period} onChange={(e) => setPeriod(e.target.value as Period)}>
            {PERIODS.map((p) => <option key={p.key} value={p.key}>{p.label}</option>)}
          </select>
        </div>
      </div>
      {err && <div className="err">{err}</div>}
      <div className="summary">
        <div className="stat"><div className="n">{ov?.nodes.length ?? '—'}</div><div className="l">节点</div></div>
        <div className="stat"><div className="n">{ov ? `${ov.workloads.ready}/${ov.workloads.total}` : '—'}</div><div className="l">工作负载就绪</div></div>
        <div className="stat"><div className="n" style={{ color: alerts.length ? 'var(--bad)' : undefined }}>{alerts.length}</div><div className="l">当前告警</div></div>
      </div>

      <div className="cards">
        {ov?.nodes.map((n) => {
          const s = series[n.name]
          const tab = chartTab[n.name] || 'cpu'
          const metric = CARD_METRICS.find((m) => m.key === tab)!
          return (
            <div className="card" key={n.name}>
              <h3>
                {n.name}{' '}
                <span className={`badge ${n.ready ? 'ok' : 'bad'}`}>{n.ready ? 'Ready' : 'NotReady'}</span>
              </h3>
              <div className="sub">{n.roles.join(', ') || '—'} · {n.kubeletVersion}</div>
              {n.hasData ? (
                <>
                  {/* 仪表盘：水位一眼看 */}
                  {n.cpuCores > 0
                    ? <UsageBar used={n.cpu} total={n.cpuCores} label="CPU" fmt={(v) => `${v.toFixed(2)} 核`} />
                    : <div className="bar-row"><span>CPU</span><span>{n.cpu.toFixed(2)} 核</span></div>}
                  <UsageBar used={n.memUse} total={n.memTot} label="内存" />
                  <UsageBar used={n.diskUse} total={n.diskTot} label="磁盘" />
                  <div className="info-row"><span>负载 (1m)</span><span>{n.load1.toFixed(2)}</span></div>
                  <div className="info-row">
                    <span>流量 · {periodLabel}</span>
                    <span>↑ {fmtBytes(traffic[n.name]?.tx ?? 0)} · ↓ {fmtBytes(traffic[n.name]?.rx ?? 0)}</span>
                  </div>

                  {/* 趋势图：标签页切换，全卡只一条曲线 */}
                  <div className="chart-tabs">
                    {CARD_METRICS.map((m) => (
                      <button
                        key={m.key}
                        className={tab === m.key ? 'tab active' : 'tab'}
                        onClick={() => setChartTab((t) => ({ ...t, [n.name]: m.key }))}
                      >{m.label}</button>
                    ))}
                  </div>
                  {s && <Chart data={s[tab]} label={metric.label} title={`${metric.label} · 近 1 小时`} fmt={metric.fmt} />}
                </>
              ) : (
                <div className="sub">尚无采样数据（采集器未在该节点运行）</div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
