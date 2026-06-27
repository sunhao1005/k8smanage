export function fmtBytes(b: number): string {
  if (!b) return '0'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = b
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(1)} ${u[i]}`
}

// UsageBar 画使用率条，>70% 转橙、>90% 转红。
export default function UsageBar({ used, total, label }: { used: number; total: number; label: string }) {
  const pct = total > 0 ? Math.min(100, (used / total) * 100) : 0
  const cls = pct > 90 ? 'bar bad' : pct > 70 ? 'bar warn' : 'bar'
  return (
    <div style={{ marginBottom: 8 }}>
      <div className="bar-row">
        <span>{label}</span>
        <span>{total > 0 ? `${fmtBytes(used)} / ${fmtBytes(total)} (${pct.toFixed(0)}%)` : '—'}</span>
      </div>
      <div className={cls}><div style={{ width: `${pct}%` }} /></div>
    </div>
  )
}
