import * as React from "react"
import { cn } from "@/lib/utils"

export function Card({ className, ref, ...props }: React.HTMLAttributes<HTMLDivElement> & { ref?: React.Ref<HTMLDivElement> }) {
  return (
    <div
      ref={ref}
      className={cn("surface-card p-6 text-zinc-100 shadow-xl", className)}
      {...props}
    />
  )
}

export function CardHeader({ className, ref, ...props }: React.HTMLAttributes<HTMLDivElement> & { ref?: React.Ref<HTMLDivElement> }) {
  return (
    <div
      ref={ref}
      className={cn("flex flex-col space-y-1.5 pb-4", className)}
      {...props}
    />
  )
}

export function CardTitle({ className, ref, ...props }: React.HTMLAttributes<HTMLHeadingElement> & { ref?: React.Ref<HTMLHeadingElement> }) {
  return (
    <h3
      ref={ref}
      className={cn("text-lg font-semibold leading-none tracking-tight text-white", className)}
      {...props}
    />
  )
}

export function CardDescription({ className, ref, ...props }: React.HTMLAttributes<HTMLParagraphElement> & { ref?: React.Ref<HTMLParagraphElement> }) {
  return (
    <p
      ref={ref}
      className={cn("text-sm text-zinc-400", className)}
      {...props}
    />
  )
}

export function CardContent({ className, ref, ...props }: React.HTMLAttributes<HTMLDivElement> & { ref?: React.Ref<HTMLDivElement> }) {
  return (
    <div ref={ref} className={cn("pt-0", className)} {...props} />
  )
}

export function CardFooter({ className, ref, ...props }: React.HTMLAttributes<HTMLDivElement> & { ref?: React.Ref<HTMLDivElement> }) {
  return (
    <div
      ref={ref}
      className={cn("flex items-center pt-4 border-t border-zinc-800", className)}
      {...props}
    />
  )
}
