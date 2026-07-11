import * as React from "react"
import { cn } from "@/lib/utils"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "outline" | "success" | "warning" | "destructive" | "cyan" | "purple"
}

export function Badge({ className, variant = "default", ...props }: BadgeProps) {
  const variantStyles = {
    default: "bg-zinc-800 text-zinc-100 border border-zinc-700",
    outline: "border border-zinc-700 text-zinc-300 bg-transparent",
    success: "bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 badge-glow-emerald font-bold",
    warning: "bg-amber-500/10 text-amber-400 border border-amber-500/30 badge-glow-amber font-bold",
    destructive: "bg-rose-500/10 text-rose-400 border border-rose-500/30 badge-glow-rose font-bold",
    cyan: "bg-cyan-500/10 text-cyan-400 border border-cyan-500/30 badge-glow-cyan font-bold",
    purple: "bg-purple-500/10 text-purple-400 border border-purple-500/30 font-bold",
  }[variant]

  return (
    <div
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none select-none",
        variantStyles,
        className
      )}
      {...props}
    />
  )
}
