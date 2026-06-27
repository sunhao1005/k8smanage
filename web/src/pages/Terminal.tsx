import { useEffect, useRef, useState } from 'react'
import { Terminal as XTerm } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { wsURL } from '../api'
import PodPicker from '../components/PodPicker'

export default function Terminal({ onAuthError }: { onAuthError: () => void }) {
  const [status, setStatus] = useState('')
  const elRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const termRef = useRef<XTerm | null>(null)
  const fitRef = useRef<FitAddon | null>(null)

  useEffect(() => () => { wsRef.current?.close(); termRef.current?.dispose() }, [])

  function connect(ns: string, pod: string, container: string) {
    wsRef.current?.close()
    termRef.current?.dispose()
    if (!elRef.current) return

    const term = new XTerm({ convertEol: true, fontSize: 13, cursorBlink: true, theme: { background: '#0b1220' } })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(elRef.current)
    fit.fit()
    termRef.current = term
    fitRef.current = fit

    setStatus('连接中…')
    const ws = new WebSocket(wsURL(`/exec?ns=${ns}&pod=${pod}&container=${container}`))
    ws.binaryType = 'arraybuffer'
    const enc = new TextEncoder()

    ws.onopen = () => {
      setStatus(`已进入 ${ns}/${pod}`)
      sendResize()
    }
    ws.onmessage = (e) => term.write(new Uint8Array(e.data as ArrayBuffer))
    ws.onclose = () => setStatus((s) => s + ' · 已退出')
    ws.onerror = () => setStatus('连接出错')

    // 键入 → stdin（二进制）
    term.onData((d) => { if (ws.readyState === ws.OPEN) ws.send(enc.encode(d)) })

    function sendResize() {
      fit.fit()
      if (ws.readyState === ws.OPEN) ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
    const onResize = () => sendResize()
    window.addEventListener('resize', onResize)
    ws.addEventListener('close', () => window.removeEventListener('resize', onResize))
    wsRef.current = ws
  }

  return (
    <div>
      <h2 className="page-title">终端</h2>
      <PodPicker onPick={connect} onAuthError={onAuthError} actionLabel="打开终端" />
      <div style={{ color: 'var(--muted)', fontSize: 12, marginBottom: 6 }}>{status}</div>
      <div className="term" ref={elRef} />
    </div>
  )
}
