import * as React from "react"
import { cn } from "@/lib/utils"

export interface LabelProps extends React.LabelHTMLAttributes<HTMLLabelElement> {
  ref?: React.Ref<HTMLLabelElement>
}

export function Label({ className, ref, ...props }: LabelProps) {
  return (
    <label
      ref={ref}
      className={cn(
        "text-sm font-medium leading-none text-zinc-300 peer-disabled:cursor-not-allowed peer-disabled:opacity-70 select-none",
        className
      )}
      {...props}
    />
  )
}
