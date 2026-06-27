import { useEffect, useState } from 'react'
import { API, Pod, Unauthorized } from '../api'

// PodPicker 拉取 Pod 列表，提供 ns/pod/container 三级选择 + 操作按钮。
export default function PodPicker({
  onPick, onAuthError, actionLabel,
}: {
  onPick: (ns: string, pod: string, container: string) => void
  onAuthError: () => void
  actionLabel: string
}) {
  const [pods, setPods] = useState<Pod[]>([])
  const [sel, setSel] = useState('')
  const [container, setContainer] = useState('')
  const [err, setErr] = useState('')

  useEffect(() => {
    (async () => {
      try {
        const p = await API.pods()
        setPods(p)
      } catch (e: any) {
        if (e instanceof Unauthorized) return onAuthError()
        setErr(e.message)
      }
    })()
  }, [])

  const cur = pods.find((p) => `${p.namespace}/${p.name}` === sel)

  function pickPod(v: string) {
    setSel(v)
    const p = pods.find((x) => `${x.namespace}/${x.name}` === v)
    setContainer(p?.containers[0] || '')
  }

  return (
    <div>
      {err && <div className="err">{err}</div>}
      <div className="toolbar">
        <select value={sel} onChange={(e) => pickPod(e.target.value)}>
          <option value="">选择 Pod…</option>
          {pods.map((p) => (
            <option key={`${p.namespace}/${p.name}`} value={`${p.namespace}/${p.name}`}>
              {p.namespace} / {p.name} ({p.phase})
            </option>
          ))}
        </select>
        <select value={container} onChange={(e) => setContainer(e.target.value)} disabled={!cur}>
          {cur?.containers.map((c) => <option key={c} value={c}>{c}</option>)}
        </select>
        <button
          className="btn primary"
          disabled={!cur || !container}
          onClick={() => cur && onPick(cur.namespace, cur.name, container)}
        >
          {actionLabel}
        </button>
      </div>
    </div>
  )
}
