import React, { type ReactElement, type ReactNode } from "react";
import * as RadixTooltip from "@radix-ui/react-tooltip";
import styles from "./Tooltip.module.css";

interface TooltipProps {
  content: ReactNode;
  children: ReactElement;
  side?: "top" | "right" | "bottom" | "left";
  align?: "start" | "center" | "end";
  sideOffset?: number;
  /** Wrap the trigger so the tooltip still works when the child is disabled. */
  disabledTrigger?: boolean;
  /** Skip the portal when the trigger lives inside a fullscreen element,
   *  where content portalled to <body> would be invisible. */
  inline?: boolean;
}

/**
 * Hover/focus label. Requires <TooltipProvider> near the root (App.tsx).
 * Touch devices do not open tooltips, so never put information here that is
 * not also reachable another way — for icon buttons it mirrors aria-label.
 */
export const Tooltip: React.FC<TooltipProps> = ({
  content,
  children,
  side = "top",
  align = "center",
  sideOffset = 6,
  disabledTrigger = false,
  inline = false,
}) => {
  if (content === null || content === undefined || content === "") {
    return children;
  }

  const body = (
    <RadixTooltip.Content
      side={side}
      align={align}
      sideOffset={sideOffset}
      collisionPadding={8}
      className={styles.content}
    >
      {content}
      <RadixTooltip.Arrow className={styles.arrow} width={10} height={5} />
    </RadixTooltip.Content>
  );

  return (
    <RadixTooltip.Root>
      <RadixTooltip.Trigger asChild>
        {disabledTrigger ? (
          <span className={styles.disabledTrigger}>{children}</span>
        ) : (
          children
        )}
      </RadixTooltip.Trigger>
      {inline ? body : <RadixTooltip.Portal>{body}</RadixTooltip.Portal>}
    </RadixTooltip.Root>
  );
};

export const TooltipProvider: React.FC<RadixTooltip.TooltipProviderProps> = (
  props,
) => <RadixTooltip.Provider {...props} />;
