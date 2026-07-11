import * as React from "react"
import { cn } from "@/lib/utils"

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 
    | "default" 
    | "emerald" 
    | "cyan" 
    | "outline" 
    | "outline-emerald" 
    | "outline-cyan" 
    | "ghost" 
    | "destructive" 
    | "destructive-outline" 
    | "secondary" 
    | "warning"
  size?: "default" | "sm" | "lg" | "icon"
  ref?: React.Ref<HTMLButtonElement>
}

export function Button({ 
  className, 
  variant = "default", 
  size = "default", 
  ref,
  ...props 
}: ButtonProps) {
  const baseStyles = "inline-flex items-center justify-center rounded-md text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400 focus-visible:ring-offset-2 focus-visible:ring-offset-[#0a0a0a] disabled:pointer-events-none disabled:opacity-50 cursor-pointer select-none active:scale-[0.98]"
  
  const variantStyles = {
    default: "bg-white text-black hover:bg-zinc-200 shadow-md font-bold",
    emerald: "bg-emerald-600 hover:bg-emerald-500 text-white shadow-lg shadow-emerald-950/40 font-bold border border-emerald-500/30",
    cyan: "bg-cyan-600 hover:bg-cyan-500 text-white shadow-lg shadow-cyan-950/40 font-bold border border-cyan-500/30",
    outline: "border border-zinc-800 bg-zinc-900/60 hover:bg-zinc-800 hover:border-zinc-700 text-zinc-100 font-semibold",
    "outline-emerald": "border border-emerald-500/30 bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 font-semibold",
    "outline-cyan": "border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-400 font-semibold",
    ghost: "hover:bg-zinc-800/60 hover:text-white text-zinc-300 font-medium",
    destructive: "bg-rose-600 hover:bg-rose-500 text-white shadow-lg shadow-rose-950/40 font-bold border border-rose-500/30",
    "destructive-outline": "bg-rose-500/10 text-rose-400 border border-rose-500/30 hover:bg-rose-500/20 font-semibold",
    secondary: "bg-zinc-800 text-zinc-100 hover:bg-zinc-700 font-semibold border border-zinc-700/50",
    warning: "bg-amber-600 hover:bg-amber-500 text-black font-bold shadow-lg shadow-amber-950/40 border border-amber-500/30",
  }[variant]

  const sizeStyles = {
    default: "h-9 px-4 py-2",
    sm: "h-8 rounded-md px-3 text-xs",
    lg: "h-10 rounded-md px-8 text-base",
    icon: "h-9 w-9",
  }[size]

  return (
    <button
      ref={ref}
      className={cn(baseStyles, variantStyles, sizeStyles, className)}
      {...props}
    />
  )
}
