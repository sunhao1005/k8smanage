import { useEffect, useRef } from 'react'
import uPlot from 'uplot'

// Chart 用 uPlot 画单条时序线（高性能）。data=[xs(unix秒), ys]。
export default function Chart({
  data, label, color = '#2563eb', fmt,
}: {
  data: [number[], number[]]
  label: string
  color?: string
  fmt?: (v: number | null) => string
}) {
  const ref = useRef<HTMLDivElement>(null)
  const plot = useRef<uPlot | null>(null)

  useEffect(() => {
    if (!ref.current) return
    const valFmt = (_: any, v: number | null) => (v == null ? '-' : fmt ? fmt(v) : String(v))
    const opts: uPlot.Options = {
      width: ref.current.clientWidth || 300,
      height: 130,
      legend: { show: false },
      scales: { x: { time: true } },
      series: [
        {},
        { label, stroke: color, width: 1.5, fill: color + '22', value: valFmt },
      ],
      axes: [
        { stroke: '#94a3b8', grid: { stroke: '#f1f5f9' } },
        { stroke: '#94a3b8', grid: { stroke: '#f1f5f9' }, size: 52, values: (_: any, vals: number[]) => vals.map((v) => (fmt ? fmt(v) : String(v))) },
      ],
    }
    plot.current = new uPlot(opts, data, ref.current)
    const onResize = () => plot.current?.setSize({ width: ref.current!.clientWidth, height: 130 })
    window.addEventListener('resize', onResize)
    return () => { window.removeEventListener('resize', onResize); plot.current?.destroy() }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => { plot.current?.setData(data) }, [data])

  return <div ref={ref} />
}
