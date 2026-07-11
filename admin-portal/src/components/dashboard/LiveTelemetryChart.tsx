import * as React from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import type { TelemetryHistoryPoint } from "@/types/telemetry"

interface LiveTelemetryChartProps {
  title: string
  description?: string
  data: TelemetryHistoryPoint[]
  dataKey: keyof TelemetryHistoryPoint
  color?: string
  unit?: string
  minValue?: number
  maxValue?: number
}

export const LiveTelemetryChart = React.memo(function LiveTelemetryChart({
  title,
  description,
  data,
  dataKey,
  color = "#10b981",
  unit = "",
  minValue,
  maxValue,
}: LiveTelemetryChartProps) {
  const [hoverIndex, setHoverIndex] = React.useState<number | null>(null)
  const svgRef = React.useRef<SVGSVGElement | null>(null)

  if (!data || data.length === 0) {
    return (
      <Card className="border-zinc-800 bg-[#111111]">
        <CardHeader>
          <CardTitle className="text-base text-white">{title}</CardTitle>
          {description && <CardDescription>{description}</CardDescription>}
        </CardHeader>
        <CardContent className="h-56 flex items-center justify-center text-zinc-500 font-mono text-sm">
          Waiting for telemetry stream points...
        </CardContent>
      </Card>
    )
  }

  const values = data.map(d => Number(d[dataKey]) || 0)
  const computedMin = minValue !== undefined ? minValue : Math.min(...values)
  const computedMax = maxValue !== undefined ? maxValue : Math.max(...values)
  const range = (computedMax - computedMin) || 1
  const paddedMin = Math.max(0, computedMin - range * 0.1)
  const paddedMax = computedMax + range * 0.1
  const paddedRange = paddedMax - paddedMin || 1

  const width = 600
  const height = 180
  const paddingX = 40
  const paddingY = 20
  const plotWidth = width - paddingX * 2
  const plotHeight = height - paddingY * 2

  const points = data.map((d, index) => {
    const val = Number(d[dataKey]) || 0
    const x = paddingX + (index / Math.max(1, data.length - 1)) * plotWidth
    const y = paddingY + plotHeight - ((val - paddedMin) / paddedRange) * plotHeight
    return { x, y, val, label: d.timeLabel }
  })

  const polylinePoints = points.map(p => `${p.x},${p.y}`).join(" ")
  const areaPoints = `${paddingX},${paddingY + plotHeight} ${polylinePoints} ${paddingX + plotWidth},${paddingY + plotHeight}`

  const activePoint = hoverIndex !== null && points[hoverIndex] ? points[hoverIndex] : points[points.length - 1]

  const handleMouseMove = (e: React.MouseEvent<SVGSVGElement>) => {
    if (!svgRef.current || points.length === 0) return
    const rect = svgRef.current.getBoundingClientRect()
    const mouseX = e.clientX - rect.left
    const relativeX = (mouseX / rect.width) * width
    
    // Find closest point
    let closestIdx = 0
    let minDist = Infinity
    points.forEach((p, idx) => {
      const dist = Math.abs(p.x - relativeX)
      if (dist < minDist) {
        minDist = dist
        closestIdx = idx
      }
    })
    setHoverIndex(closestIdx)
  }

  const handleMouseLeave = () => {
    setHoverIndex(null)
  }

  const gradientId = `grad-${title.replace(/[^a-zA-Z0-9]/g, "")}`

  return (
    <Card className="border-zinc-800 bg-[#111111] hover:border-zinc-700/80 transition-all duration-200">
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <div>
          <CardTitle className="text-base font-semibold text-white">{title}</CardTitle>
          {description && <CardDescription className="text-xs">{description}</CardDescription>}
        </div>
        <div className="text-right">
          <div className="text-2xl font-bold font-mono text-white tracking-tight flex items-baseline justify-end">
            <span style={{ color }}>{activePoint ? activePoint.val : 0}</span>
            {unit && <span className="text-xs font-mono text-zinc-500 ml-1">{unit}</span>}
          </div>
          <div className="text-[10px] font-mono text-zinc-500">
            {activePoint ? activePoint.label : "LIVE"}
          </div>
        </div>
      </CardHeader>
      <CardContent className="pt-2">
        <div className="w-full aspect-[10/3] min-h-[140px] relative">
          <svg
            ref={svgRef}
            viewBox={`0 0 ${width} ${height}`}
            className="w-full h-full overflow-visible cursor-crosshair"
            onMouseMove={handleMouseMove}
            onMouseLeave={handleMouseLeave}
          >
            <defs>
              <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={color} stopOpacity={0.35} />
                <stop offset="100%" stopColor={color} stopOpacity={0.0} />
              </linearGradient>
            </defs>

            {/* Grid lines */}
            {[0, 0.5, 1].map((ratio, idx) => {
              const yPos = paddingY + plotHeight * ratio
              const valLabel = Math.round(paddedMax - ratio * paddedRange)
              return (
                <g key={idx}>
                  <line
                    x1={paddingX}
                    y1={yPos}
                    x2={paddingX + plotWidth}
                    y2={yPos}
                    stroke="#27272a"
                    strokeDasharray="4 4"
                    strokeWidth="1"
                  />
                  <text
                    x={paddingX - 8}
                    y={yPos + 4}
                    textAnchor="end"
                    fill="#71717a"
                    className="text-[10px] font-mono"
                  >
                    {valLabel}
                  </text>
                </g>
              )
            })}

            {/* Area fill */}
            <polygon points={areaPoints} fill={`url(#${gradientId})`} />

            {/* Line stroke */}
            <polyline
              fill="none"
              stroke={color}
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              points={polylinePoints}
            />

            {/* Active hover dot & guideline */}
            {activePoint && (
              <g>
                <line
                  x1={activePoint.x}
                  y1={paddingY}
                  x2={activePoint.x}
                  y2={paddingY + plotHeight}
                  stroke="#52525b"
                  strokeDasharray="2 2"
                  strokeWidth="1"
                />
                <circle
                  cx={activePoint.x}
                  cy={activePoint.y}
                  r="5"
                  fill="#0a0a0a"
                  stroke={color}
                  strokeWidth="3"
                />
              </g>
            )}
          </svg>
        </div>
      </CardContent>
    </Card>
  )
})
