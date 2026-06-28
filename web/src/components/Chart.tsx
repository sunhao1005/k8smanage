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
  const peak = ys.length ? Math.max(...ys) : null
  const avg = ys.length ? ys.reduce((a, b) => a + b, 0) / ys.length : null
  const f = (v: number | null) => (v == null ? '—' : fmt ? fmt(v) : String(v))

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
        // X 轴：只显示「时:分」，去掉 uPlot 默认会在底部多出的日期行
        {
          stroke: '#c9cdd4', grid: { show: false }, ticks: { show: false }, size: 26,
          values: (_u: any, splits: number[]) => splits.map((ts) => {
            const d = new Date(ts * 1000)
            return `${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}`
          }),
        },
        // Y 轴：仅保留淡横向网格，不显示刻度标签（当前值已在右上角，避免小数值都显示成 0.00）
        {
          size: 6, stroke: 'transparent',
          grid: { stroke: '#f2f3f5', width: 1 },
          ticks: { show: false },
          values: (_u: any, splits: number[]) => splits.map(() => ''),
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
      </div>
      <div ref={ref} />
      <div className="chart-stats">
        <span>当前 <b>{f(cur)}</b></span>
        <span>均值 <b>{f(avg)}</b></span>
        <span>峰值 <b>{f(peak)}</b></span>
      </div>
    </div>
  )
}
