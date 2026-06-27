import { useEffect, useRef, useState } from 'react'
import { wsURL } from '../api'
import PodPicker from '../components/PodPicker'

export default function Logs({ onAuthError }: { onAuthError: () => void }) {
  const [lines, setLines] = useState('')
  const [status, setStatus] = useState('')
  const wsRef = useRef<WebSocket | null>(null)
  const preRef = useRef<HTMLDivElement>(null)

  useEffect(() => () => wsRef.current?.close(), [])
  useEffect(() => {
    if (preRef.current) preRef.current.scrollTop = preRef.current.scrollHeight
  }, [lines])

  function connect(ns: string, pod: string, container: string) {
    wsRef.current?.close()
    setLines('')
    setStatus('连接中…')
    const ws = new WebSocket(wsURL(`/logs?ns=${ns}&pod=${pod}&container=${container}&follow=1`))
    ws.onopen = () => setStatus(`正在跟随 ${ns}/${pod}`)
    ws.onmessage = (e) => setLines((p) => p + e.data)
    ws.onclose = () => setStatus((s) => s + ' · 已断开')
    ws.onerror = () => setStatus('连接出错')
    wsRef.current = ws
  }

  return (
    <div>
      <h2 className="page-title">日志</h2>
      <PodPicker onPick={connect} onAuthError={onAuthError} actionLabel="查看日志" />
      <div style={{ color: 'var(--muted)', fontSize: 12, marginBottom: 6 }}>{status}</div>
      <div className="logs" ref={preRef}>{lines || '选择一个 Pod 开始跟随日志…'}</div>
    </div>
  )
}
