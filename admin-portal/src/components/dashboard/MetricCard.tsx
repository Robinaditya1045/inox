import * as React from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Progress } from "@/components/ui/progress"
import type { LucideIcon } from "lucide-react"

interface MetricCardProps {
  title: string
  value: string | number
  unit?: string
  icon: LucideIcon
  iconColor?: string
  category: "RED" | "USE"
  categoryLabel?: string
  trend?: string
  trendPositive?: boolean
  progressValue?: number
  footerText?: string
}

export const MetricCard = React.memo(function MetricCard({
  title,
  value,
  unit = "",
  icon: Icon,
  iconColor = "text-emerald-400",
  category,
  categoryLabel,
  trend,
  trendPositive = true,
  progressValue,
  footerText,
}: MetricCardProps) {
  return (
    <Card className="border-zinc-800 bg-gradient-to-br from-[#111111] to-[#16161a] hover:border-zinc-700 transition-all duration-200 shadow-lg">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <div className="flex items-center space-x-2">
          <CardTitle className="text-xs font-semibold uppercase tracking-wider text-zinc-400">
            {title}
          </CardTitle>
          <Badge variant={category === "RED" ? "warning" : "cyan"} className="text-[10px] px-1.5 py-0">
            {categoryLabel || category}
          </Badge>
        </div>
        <div className={`p-2 rounded-md bg-black/40 border border-zinc-800 ${iconColor}`}>
          <Icon className="h-4 w-4" />
        </div>
      </CardHeader>
      <CardContent>
        <div className="flex items-baseline space-x-1">
          <span className="text-3xl font-bold font-mono text-white tracking-tight">{value}</span>
          {unit && <span className="text-sm font-mono text-zinc-500">{unit}</span>}
        </div>

        {(trend || footerText) && (
          <p className="text-xs text-zinc-400 mt-2 flex items-center justify-between">
            {trend && (
              <span className={`font-mono font-medium ${trendPositive ? "text-emerald-400" : "text-rose-400"}`}>
                {trend}
              </span>
            )}
            {footerText && <span className="text-zinc-500 ml-auto">{footerText}</span>}
          </p>
        )}

        {progressValue !== undefined && (
          <Progress value={Math.min(100, Math.max(0, progressValue))} className="mt-3 h-1" />
        )}
      </CardContent>
    </Card>
  )
})
