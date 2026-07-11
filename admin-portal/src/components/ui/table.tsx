import * as React from "react"
import { cn } from "@/lib/utils"

export function Table({ className, ref, ...props }: React.HTMLAttributes<HTMLTableElement> & { ref?: React.Ref<HTMLTableElement> }) {
  return (
    <div className="relative w-full overflow-auto rounded-lg border border-zinc-800">
      <table ref={ref} className={cn("w-full caption-bottom text-sm text-left", className)} {...props} />
    </div>
  )
}

export function TableHeader({ className, ref, ...props }: React.HTMLAttributes<HTMLTableSectionElement> & { ref?: React.Ref<HTMLTableSectionElement> }) {
  return (
    <thead ref={ref} className={cn("[&_tr]:border-b border-zinc-800 bg-zinc-900/60", className)} {...props} />
  )
}

export function TableBody({ className, ref, ...props }: React.HTMLAttributes<HTMLTableSectionElement> & { ref?: React.Ref<HTMLTableSectionElement> }) {
  return (
    <tbody ref={ref} className={cn("[&_tr:last-child]:border-0", className)} {...props} />
  )
}

export function TableRow({ className, ref, ...props }: React.HTMLAttributes<HTMLTableRowElement> & { ref?: React.Ref<HTMLTableRowElement> }) {
  return (
    <tr
      ref={ref}
      className={cn("border-b border-zinc-800 transition-colors hover:bg-zinc-900/40 data-[state=selected]:bg-zinc-800", className)}
      {...props}
    />
  )
}

export function TableHead({ className, ref, ...props }: React.ThHTMLAttributes<HTMLTableCellElement> & { ref?: React.Ref<HTMLTableCellElement> }) {
  return (
    <th
      ref={ref}
      className={cn("h-10 px-4 text-left align-middle font-semibold text-zinc-400 text-xs uppercase tracking-wider select-none", className)}
      {...props}
    />
  )
}

export function TableCell({ className, ref, ...props }: React.TdHTMLAttributes<HTMLTableCellElement> & { ref?: React.Ref<HTMLTableCellElement> }) {
  return (
    <td ref={ref} className={cn("p-4 align-middle text-zinc-200", className)} {...props} />
  )
}
