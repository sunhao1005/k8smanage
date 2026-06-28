import { useEffect, useMemo, useState } from 'react'
import { API, Workload, Unauthorized } from '../api'
import { formatAge } from '../components/UsageBar'
import Modal, { ModalActions } from '../components/Modal'

type Dialog =
  | { kind: 'scale'; w: Workload }
  | { kind: 'pause'; w: Workload }
  | null

export default function Workloads({ onAuthError }: { onAuthError: () => void }) {
  const [wls, setWls] = useState<Workload[]>([])
  const [err, setErr] = useState('')
  const [msg, setMsg] = useState('')
  const [dialog, setDialog] = useState<Dialog>(null)
  const [scaleVal, setScaleVal] = useState(0)
  // 分类筛选
  const [nsFilter, setNsFilter] = useState('')
  const [kindFilter, setKindFilter] = useState('')
  const [search, setSearch] = useState('')

  const namespaces = useMemo(() => Array.from(new Set(wls.map((w) => w.namespace))).sort(), [wls])
  const filtered = useMemo(() => wls.filter((w) =>
    (!nsFilter || w.namespace === nsFilter) &&
    (!kindFilter || w.kind === kindFilter) &&
    (!search || w.name.includes(search) || (w.image || '').includes(search)),
  ), [wls, nsFilter, kindFilter, search])

  async function load() {
    try {
      setWls(await API.workloads())
      setErr('')
    } catch (e: any) {
      if (e instanceof Unauthorized) return onAuthError()
      setErr(e.message)
    }
  }
  // 自动刷新：消除写操作后 informer 缓存的短暂延迟，并让就绪数实时更新。
  useEffect(() => {
    load()
    const id = setInterval(load, 5000)
    return () => clearInterval(id)
  }, [])

  function flash(s: string) {
    setMsg(s)
    setTimeout(() => setMsg(''), 2500)
  }

  async function run(fn: () => Promise<any>, ok: string) {
    try {
      await fn()
      flash(ok)
      load()
    } catch (e: any) {
      if (e instanceof Unauthorized) return onAuthError()
      setErr(e.message)
    }
  }

  function openScale(w: Workload) {
    setScaleVal(w.desired)
    setDialog({ kind: 'scale', w })
  }

  const canScale = (k: string) => k === 'Deployment' || k === 'StatefulSet'

  return (
    <div>
      <h2 className="page-title">工作负载</h2>
      {err && <div className="err">{err}</div>}
      {msg && <div style={{ color: 'var(--ok)', marginBottom: 10 }}>{msg}</div>}
      <div className="toolbar">
        <select value={nsFilter} onChange={(e) => setNsFilter(e.target.value)}>
          <option value="">全部命名空间</option>
          {namespaces.map((ns) => <option key={ns} value={ns}>{ns}</option>)}
        </select>
        <select value={kindFilter} onChange={(e) => setKindFilter(e.target.value)}>
          <option value="">全部类型</option>
          <option value="Deployment">Deployment</option>
          <option value="StatefulSet">StatefulSet</option>
          <option value="DaemonSet">DaemonSet</option>
        </select>
        <input placeholder="搜索名称 / 镜像" value={search} onChange={(e) => setSearch(e.target.value)} style={{ minWidth: 180 }} />
        <span style={{ color: 'var(--muted)', fontSize: 12 }}>共 {filtered.length} / {wls.length}</span>
        <button className="btn" onClick={load} style={{ marginLeft: 'auto' }}>刷新</button>
      </div>
      <table>
        <thead>
          <tr><th>命名空间</th><th>类型</th><th>名称</th><th>镜像</th><th>状态</th><th>运行时长</th><th>操作</th></tr>
        </thead>
        <tbody>
          {filtered.map((w) => {
            const ready = w.desired > 0 && w.ready === w.desired
            return (
              <tr key={`${w.namespace}/${w.kind}/${w.name}`}>
                <td>{w.namespace}</td>
                <td>{w.kind}</td>
                <td>{w.name}</td>
                <td title={w.image} className="cell-image">{w.image || '—'}</td>
                <td>
                  {w.paused
                    ? <span className="badge warn">已暂停</span>
                    : <span className={`badge ${ready ? 'ok' : 'warn'}`}>{w.ready}/{w.desired}</span>}
                </td>
                <td>{formatAge(w.createdAt)}</td>
                <td className="cell-actions">
                  {canScale(w.kind) && <button className="btn" onClick={() => openScale(w)}>扩缩</button>}
                  <button className="btn" onClick={() => run(() => API.restart(w.namespace, w.kind, w.name), `已重启 ${w.name}`)}>重启</button>
                  {w.pausable && (w.paused
                    ? <button className="btn primary" onClick={() => run(() => API.resume(w.namespace, w.kind, w.name), `已启用 ${w.name}`)}>启用</button>
                    : <button className="btn" onClick={() => setDialog({ kind: 'pause', w })}>暂停</button>)}
                </td>
              </tr>
            )
          })}
          {filtered.length === 0 && <tr><td colSpan={7} style={{ color: 'var(--muted)' }}>无匹配的工作负载</td></tr>}
        </tbody>
      </table>

      {dialog?.kind === 'scale' && (
        <Modal title={`扩缩 ${dialog.w.name}`} onClose={() => setDialog(null)}>
          <div style={{ color: 'var(--muted)', fontSize: 12, marginBottom: 10 }}>
            {dialog.w.kind} · 当前 {dialog.w.desired} 副本
          </div>
          <div className="stepper">
            <button className="btn" onClick={() => setScaleVal((v) => Math.max(0, v - 1))}>−</button>
            <input
              type="number" min={0} value={scaleVal}
              onChange={(e) => setScaleVal(Math.max(0, parseInt(e.target.value, 10) || 0))}
            />
            <button className="btn" onClick={() => setScaleVal((v) => v + 1)}>+</button>
            <span style={{ color: 'var(--muted)', fontSize: 12 }}>副本</span>
          </div>
          <ModalActions>
            <button className="btn" onClick={() => setDialog(null)}>取消</button>
            <button className="btn primary" onClick={() => {
              const w = dialog.w
              setDialog(null)
              run(() => API.scale(w.namespace, w.kind, w.name, scaleVal), `已扩缩 ${w.name} → ${scaleVal}`)
            }}>确定</button>
          </ModalActions>
        </Modal>
      )}

      {dialog?.kind === 'pause' && (
        <Modal title={`暂停 ${dialog.w.name}？`} onClose={() => setDialog(null)}>
          <div style={{ marginBottom: 14 }}>
            将把副本缩到 0，<b>Pod 会被移除</b>、释放资源。原副本数（{dialog.w.desired}）会被记住，点「启用」即可恢复。
          </div>
          <ModalActions>
            <button className="btn" onClick={() => setDialog(null)}>取消</button>
            <button className="btn danger" onClick={() => {
              const w = dialog.w
              setDialog(null)
              run(() => API.pause(w.namespace, w.kind, w.name), `已暂停 ${w.name}`)
            }}>确定暂停</button>
          </ModalActions>
        </Modal>
      )}
    </div>
  )
}
