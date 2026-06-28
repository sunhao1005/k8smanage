export function fmtBytes(b: number): string {
  if (!b) return '0'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = b
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(1)} ${u[i]}`
}

// formatAge 把 RFC3339 时间转成 "3d"/"5h"/"12m" 这样的运行时长。
export function formatAge(iso: string): string {
  if (!iso) return '—'
  const sec = Math.max(0, (Date.now() - Date.parse(iso)) / 1000)
  if (sec < 60) return `${Math.floor(sec)}s`
  if (sec < 3600) return `${Math.floor(sec / 60)}m`
  if (sec < 86400) return `${Math.floor(sec / 3600)}h`
  return `${Math.floor(sec / 86400)}d`
}

// UsageBar 画使用率条（>70% 转橙、>90% 转红），右侧显示「百分比 · 已用/总量」。
// fmt 自定义数值格式（默认按字节；CPU 等传自己的格式器）。
export default function UsageBar({ used, total, label, fmt = fmtBytes }: {
  used: number; total: number; label: string; fmt?: (n: number) => string
}) {
  const pct = total > 0 ? Math.min(100, (used / total) * 100) : 0
  const cls = pct > 90 ? 'bar bad' : pct > 70 ? 'bar warn' : 'bar'
  return (
    <div style={{ marginBottom: 10 }}>
      <div className="bar-row">
        <span>{label}</span>
        <span>{total > 0 ? `${pct.toFixed(0)}% · ${fmt(used)} / ${fmt(total)}` : '—'}</span>
      </div>
      <div className={cls}><div style={{ width: `${pct}%` }} /></div>
    </div>
  )
}
