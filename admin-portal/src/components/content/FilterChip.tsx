import { Check, ChevronDown } from "lucide-react"
import { Menu } from "@/components/ui/menu"
import { cx } from "@/lib/utils"

interface FilterChipProps<T extends string> {
  label: string
  value: T
  options: { value: T; label: string }[]
  /** The value that means "not filtering" */
  neutral: T
  onChange: (value: T) => void
}

/** "Status: Ready ▾" — a filter that reads as its current setting. */
export function FilterChip<T extends string>({ label, value, options, neutral, onChange }: FilterChipProps<T>) {
  const current = options.find((o) => o.value === value) ?? options[0]
  const active = value !== neutral
  return (
    <Menu
      label={label}
      align="start"
      minWidth={180}
      items={options.map((o) => ({
        label: o.label,
        icon: o.value === value ? Check : undefined,
        onSelect: () => onChange(o.value),
      }))}
      trigger={(props) => (
        <button
          {...props}
          type="button"
          className={cx(
            "inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-xs font-medium transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400",
            active
              ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-200 hover:bg-emerald-500/15"
              : "border-zinc-700 bg-zinc-900 text-zinc-300 hover:border-zinc-600 hover:text-white",
          )}
        >
          <span className="text-zinc-500">{label}:</span>
          {current.label}
          <ChevronDown className="h-3.5 w-3.5 opacity-70" aria-hidden="true" />
        </button>
      )}
    />
  )
}
