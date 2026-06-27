import { useEffect, useState } from 'react'
import { API, Overview as OV, Point, ActiveAlert, Unauthorized } from '../api'
import UsageBar from '../components/UsageBar'
import Chart from '../components/Chart'

export default function Overview({ onAuthError }: { onAuthError: () => void }) {
  const [ov, setOv] = useState<OV | null>(null)
  const [alerts, setAlerts] = useState<ActiveAlert[]>([])
  const [cpuSeries, setCpuSeries] = useState<Record<string, [number[], number[]]>>({})
  const [err, setErr] = useState('')

  // 节点集合标识：仅当节点增减时才变，用作曲线轮询的依赖。
  const nodeKey = (ov?.nodes.map((n) => n.name).sort().join(',')) || ''

  // 数字 + 告警：5s 刷新（对象状态变化快，保持响应）；两请求并行。
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

  // CPU 曲线：15s 刷新（指标按采集间隔更新，5s 重拉整段历史是浪费）。
  useEffect(() => {
    if (!nodeKey) return
    const names = nodeKey.split(',')
    let alive = true
    async function loadSeries() {
      const results = await Promise.all(
        names.map((name) =>
          API.query('node', name, 'cpu')
            .then((pts) => [name, pts] as const)
            .catch(() => [name, [] as Point[]] as const),
        ),
      )
      if (!alive) return
      const series: Record<string, [number[], number[]]> = {}
      for (const [name, pts] of results) {
        series[name] = [pts.map((p) => Date.parse(p.TS) / 1000), pts.map((p) => p.Value)]
      }
      setCpuSeries(series)
    }
    loadSeries()
    const id = setInterval(loadSeries, 15000)
    return () => { alive = false; clearInterval(id) }
  }, [nodeKey])

  return (
    <div>
      <h2 className="page-title">总览</h2>
      {err && <div className="err">{err}</div>}
      <div className="summary">
        <div className="stat"><div className="n">{ov?.nodes.length ?? '—'}</div><div className="l">节点</div></div>
        <div className="stat"><div className="n">{ov ? `${ov.workloads.ready}/${ov.workloads.total}` : '—'}</div><div className="l">工作负载就绪</div></div>
        <div className="stat"><div className="n" style={{ color: alerts.length ? 'var(--bad)' : undefined }}>{alerts.length}</div><div className="l">当前告警</div></div>
      </div>

      <div className="cards">
        {ov?.nodes.map((n) => (
          <div className="card" key={n.name}>
            <h3>
              {n.name}{' '}
              <span className={`badge ${n.ready ? 'ok' : 'bad'}`}>{n.ready ? 'Ready' : 'NotReady'}</span>
            </h3>
            <div className="sub">{n.roles.join(', ') || '—'} · {n.kubeletVersion}</div>
            {n.hasData ? (
              <>
                <div className="bar-row"><span>CPU</span><span>{n.cpu.toFixed(2)} 核 · load {n.load1.toFixed(2)}</span></div>
                {cpuSeries[n.name] && <Chart data={cpuSeries[n.name]} label="CPU 核" fmt={(v) => (v == null ? '-' : v.toFixed(2))} />}
                <UsageBar used={n.memUse} total={n.memTot} label="内存" />
                <UsageBar used={n.diskUse} total={n.diskTot} label="磁盘" />
              </>
            ) : (
              <div className="sub">尚无采样数据（采集器未在该节点运行）</div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
