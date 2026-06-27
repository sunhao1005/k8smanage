import { useEffect, useState } from 'react'
import { API, Rule, ActiveAlert, Unauthorized } from '../api'

const EMPTY: Rule = { id: '', name: '', kind: 'node', target: '', metric: 'cpu', cmp: '>', threshold: 0, forSec: 60, enabled: true }

export default function Alerts({ onAuthError }: { onAuthError: () => void }) {
  const [rules, setRules] = useState<Rule[]>([])
  const [active, setActive] = useState<ActiveAlert[]>([])
  const [form, setForm] = useState<Rule>(EMPTY)
  const [err, setErr] = useState('')

  async function load() {
    try {
      setRules(await API.rules())
      setActive(await API.active())
      setErr('')
    } catch (e: any) {
      if (e instanceof Unauthorized) return onAuthError()
      setErr(e.message)
    }
  }
  useEffect(() => {
    load()
    const id = setInterval(() => API.active().then(setActive).catch(() => {}), 5000)
    return () => clearInterval(id)
  }, [])

  async function save() {
    if (!form.name) { setErr('请填写规则名'); return }
    try {
      await API.saveRule(form)
      setForm(EMPTY)
      load()
    } catch (e: any) {
      if (e instanceof Unauthorized) return onAuthError()
      setErr(e.message)
    }
  }
  async function del(id: string) {
    try { await API.delRule(id); load() } catch (e: any) { setErr(e.message) }
  }

  const up = (k: keyof Rule, v: any) => setForm((f) => ({ ...f, [k]: v }))

  return (
    <div>
      <h2 className="page-title">告警</h2>
      {err && <div className="err">{err}</div>}

      {active.length > 0 && (
        <>
          <h3>当前告警</h3>
          <table style={{ marginBottom: 20 }}>
            <thead><tr><th>规则</th><th>对象</th><th>指标</th><th>状态</th><th>当前值</th><th>阈值</th></tr></thead>
            <tbody>
              {active.map((a, i) => (
                <tr key={i}>
                  <td>{a.ruleName}</td><td>{a.kind}/{a.target}</td><td>{a.metric}</td>
                  <td><span className={`badge ${a.state === 'firing' ? 'bad' : 'warn'}`}>{a.state}</span></td>
                  <td>{a.value.toFixed(3)}</td><td>{a.threshold}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}

      <h3>规则</h3>
      <table style={{ marginBottom: 20 }}>
        <thead><tr><th>名称</th><th>对象</th><th>条件</th><th>持续</th><th>启用</th><th></th></tr></thead>
        <tbody>
          {rules.map((r) => (
            <tr key={r.id}>
              <td>{r.name}</td>
              <td>{r.kind}{r.target ? `/${r.target}` : '（全部）'}</td>
              <td>{r.metric} {r.cmp} {r.threshold}</td>
              <td>{r.forSec}s</td>
              <td><span className={`badge ${r.enabled ? 'ok' : 'warn'}`}>{r.enabled ? '是' : '否'}</span></td>
              <td>
                <button className="btn" onClick={() => setForm(r)}>编辑</button>
                <button className="btn danger" onClick={() => del(r.id)}>删除</button>
              </td>
            </tr>
          ))}
          {rules.length === 0 && <tr><td colSpan={6} style={{ color: 'var(--muted)' }}>暂无规则</td></tr>}
        </tbody>
      </table>

      <h3>{form.id ? '编辑规则' : '新增规则'}</h3>
      <div className="card" style={{ maxWidth: 560 }}>
        <div className="grid-form">
          <label className="field">名称<input value={form.name} onChange={(e) => up('name', e.target.value)} /></label>
          <label className="field">对象类型
            <select value={form.kind} onChange={(e) => up('kind', e.target.value)}>
              <option value="node">node</option><option value="pod">pod</option>
            </select>
          </label>
          <label className="field">目标（空=全部）<input value={form.target} onChange={(e) => up('target', e.target.value)} placeholder="节点名 或 ns/pod" /></label>
          <label className="field">指标
            <select value={form.metric} onChange={(e) => up('metric', e.target.value)}>
              <option value="cpu">cpu（核）</option><option value="mem">mem（使用率）</option>
              <option value="disk">disk（使用率）</option><option value="load1">load1</option>
            </select>
          </label>
          <label className="field">比较
            <select value={form.cmp} onChange={(e) => up('cmp', e.target.value)}>
              <option value=">">&gt;</option><option value="<">&lt;</option>
            </select>
          </label>
          <label className="field">阈值<input type="number" step="any" value={form.threshold} onChange={(e) => up('threshold', parseFloat(e.target.value))} /></label>
          <label className="field">持续秒数<input type="number" value={form.forSec} onChange={(e) => up('forSec', parseInt(e.target.value, 10) || 0)} /></label>
          <label className="field">启用
            <select value={form.enabled ? '1' : '0'} onChange={(e) => up('enabled', e.target.value === '1')}>
              <option value="1">是</option><option value="0">否</option>
            </select>
          </label>
        </div>
        <div style={{ marginTop: 12 }}>
          <button className="btn primary" onClick={save}>保存</button>
          {form.id && <button className="btn" onClick={() => setForm(EMPTY)}>取消编辑</button>}
        </div>
      </div>
    </div>
  )
}
