import { useEffect, useRef } from 'react'
import uPlot from 'uplot'

// Chart 用 uPlot 画单条时序线：浅蓝渐变填充 + 细线 + 淡网格，顶部显示标题与当前值。
export default function Chart({
  data, label, title, color = '#165dff', fmt,
}: {
  data: [number[], number[]]
  label: string
  title?: string
  color?: string
  fmt?: (v: number | null) => string
}) {
  const ref = useRef<HTMLDivElement>(null)
  const plot = useRef<uPlot | null>(null)

  const ys = data[1] || []
  const cur = ys.length ? ys[ys.length - 1] : null
  const curLabel = cur == null ? '—' : fmt ? fmt(cur) : String(cur)

  useEffect(() => {
    if (!ref.current) return
    const valFmt = (_: any, v: number | null) => (v == null ? '-' : fmt ? fmt(v) : String(v))
    const opts: uPlot.Options = {
      width: ref.current.clientWidth || 300,
      height: 110,
      legend: { show: false },
      cursor: { points: { size: 5 } },
      scales: { x: { time: true } },
      series: [
        {},
        {
          label,
          stroke: color,
          width: 2,
          // 渐变面积填充，自上而下淡出
          fill: (u: uPlot) => {
            const g = u.ctx.createLinearGradient(0, u.bbox.top, 0, u.bbox.top + u.bbox.height)
            g.addColorStop(0, 'rgba(22,93,255,0.18)')
            g.addColorStop(1, 'rgba(22,93,255,0.01)')
            return g
          },
          points: { show: false },
          value: valFmt,
        },
      ],
      axes: [
        { stroke: '#c9cdd4', grid: { show: false }, ticks: { show: false }, size: 28 },
        {
          stroke: '#c9cdd4', size: 46,
          grid: { stroke: '#f0f1f3', width: 1 },
          ticks: { show: false },
          values: (_: any, vals: number[]) => vals.map((v) => (fmt ? fmt(v) : String(v))),
        },
      ],
    }
    plot.current = new uPlot(opts, data, ref.current)
    const onResize = () => plot.current?.setSize({ width: ref.current!.clientWidth, height: 110 })
    window.addEventListener('resize', onResize)
    return () => { window.removeEventListener('resize', onResize); plot.current?.destroy() }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => { plot.current?.setData(data) }, [data])

  return (
    <div className="chart-box">
      <div className="chart-head">
        <span className="chart-title">{title || label}</span>
        <span className="chart-cur">{curLabel}</span>
      </div>
      <div ref={ref} />
    </div>
  )
}
