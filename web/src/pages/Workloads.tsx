import { useEffect, useState } from 'react'
import { API, Workload, Unauthorized } from '../api'

export default function Workloads({ onAuthError }: { onAuthError: () => void }) {
  const [wls, setWls] = useState<Workload[]>([])
  const [err, setErr] = useState('')
  const [msg, setMsg] = useState('')

  async function load() {
    try {
      setWls(await API.workloads())
      setErr('')
    } catch (e: any) {
      if (e instanceof Unauthorized) return onAuthError()
      setErr(e.message)
    }
  }
  useEffect(() => { load() }, [])

  async function act(fn: () => Promise<any>, ok: string) {
    try {
      await fn()
      setMsg(ok)
      setTimeout(() => setMsg(''), 2500)
      load()
    } catch (e: any) {
      if (e instanceof Unauthorized) return onAuthError()
      setErr(e.message)
    }
  }

  function scale(w: Workload) {
    const v = prompt(`将 ${w.name} 扩缩到几个副本？`, String(w.desired))
    if (v == null) return
    const n = parseInt(v, 10)
    if (isNaN(n) || n < 0) { setErr('副本数非法'); return }
    act(() => API.scale(w.namespace, w.kind, w.name, n), `已扩缩 ${w.name} → ${n}`)
  }

  const canScale = (k: string) => k === 'Deployment' || k === 'StatefulSet'

  return (
    <div>
      <h2 className="page-title">工作负载</h2>
      {err && <div className="err">{err}</div>}
      {msg && <div style={{ color: 'var(--ok)', marginBottom: 10 }}>{msg}</div>}
      <div className="toolbar">
        <button className="btn" onClick={load}>刷新</button>
      </div>
      <table>
        <thead>
          <tr><th>命名空间</th><th>类型</th><th>名称</th><th>就绪</th><th>操作</th></tr>
        </thead>
        <tbody>
          {wls.map((w) => {
            const ready = w.desired > 0 && w.ready === w.desired
            return (
              <tr key={`${w.namespace}/${w.kind}/${w.name}`}>
                <td>{w.namespace}</td>
                <td>{w.kind}</td>
                <td>{w.name}</td>
                <td><span className={`badge ${ready ? 'ok' : 'warn'}`}>{w.ready}/{w.desired}</span></td>
                <td>
                  {canScale(w.kind) && <button className="btn" onClick={() => scale(w)}>扩缩</button>}
                  <button className="btn" onClick={() => act(() => API.restart(w.namespace, w.kind, w.name), `已重启 ${w.name}`)}>重启</button>
                </td>
              </tr>
            )
          })}
          {wls.length === 0 && <tr><td colSpan={5} style={{ color: 'var(--muted)' }}>无工作负载（或未连接集群）</td></tr>}
        </tbody>
      </table>
    </div>
  )
}
