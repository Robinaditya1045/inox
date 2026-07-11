import * as React from "react"
import { cn } from "@/lib/utils"

interface TabsProps extends React.HTMLAttributes<HTMLDivElement> {
  defaultValue: string
  value?: string
  onValueChange?: (value: string) => void
  children: React.ReactNode
}

const TabsContext = React.createContext<{
  value: string
  onValueChange: (value: string) => void
}>({ value: "", onValueChange: () => {} })

export function Tabs({ defaultValue, value: controlledValue, onValueChange, children, className, ...props }: TabsProps) {
  const [internalValue, setInternalValue] = React.useState(defaultValue)
  const value = controlledValue !== undefined ? controlledValue : internalValue
  
  const handleValueChange = (newVal: string) => {
    if (controlledValue === undefined) {
      setInternalValue(newVal)
    }
    onValueChange?.(newVal)
  }

  return (
    <TabsContext.Provider value={{ value, onValueChange: handleValueChange }}>
      <div className={cn("space-y-4", className)} {...props}>
        {children}
      </div>
    </TabsContext.Provider>
  )
}

export function TabsList({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "inline-flex h-10 items-center justify-center rounded-lg bg-zinc-900/80 p-1 text-zinc-400 border border-zinc-800/80",
        className
      )}
      {...props}
    />
  )
}

interface TabsTriggerProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  value: string
}

export function TabsTrigger({ className, value, children, ...props }: TabsTriggerProps) {
  const { value: activeValue, onValueChange } = React.useContext(TabsContext)
  const isActive = activeValue === value

  return (
    <button
      type="button"
      role="tab"
      aria-selected={isActive}
      onClick={() => onValueChange(value)}
      className={cn(
        "inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1.5 text-sm font-medium transition-all cursor-pointer",
        isActive
          ? "bg-zinc-800 text-white shadow-sm border border-zinc-700/50"
          : "hover:text-zinc-200 hover:bg-zinc-800/30",
        className
      )}
      {...props}
    >
      {children}
    </button>
  )
}

interface TabsContentProps extends React.HTMLAttributes<HTMLDivElement> {
  value: string
}

export function TabsContent({ className, value, children, ...props }: TabsContentProps) {
  const { value: activeValue } = React.useContext(TabsContext)
  if (activeValue !== value) return null

  return (
    <div
      role="tabpanel"
      className={cn("mt-2 ring-offset-background focus-visible:outline-none animate-pulse-subtle", className)}
      style={{ animationDuration: "0.3s", animationIterationCount: 1 }}
      {...props}
    >
      {children}
    </div>
  )
}
