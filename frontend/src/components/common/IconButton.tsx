import React, { type ButtonHTMLAttributes } from "react";
import { Tooltip } from "./Tooltip";
import styles from "./IconButton.module.css";

export type IconButtonVariant = "default" | "ghost" | "danger" | "active";
export type IconButtonSize = "sm" | "md" | "lg";

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** Required: accessible label for screen readers, also used as the tooltip */
  label: string;
  variant?: IconButtonVariant;
  size?: IconButtonSize;
  /** Renders the toggled-on state; pair with aria-pressed for toggles */
  isActive?: boolean;
  /** Colour of the toggled-on state */
  activeTone?: "accent" | "danger";
  /** Tooltip text when it should differ from the label (e.g. a hint on why
   *  the control is disabled). Pass false to suppress the tooltip. */
  tooltip?: React.ReactNode | false;
  tooltipSide?: "top" | "right" | "bottom" | "left";
  ref?: React.Ref<HTMLButtonElement>;
}

/**
 * Icon-only button. Always requires a `label` prop for aria-label.
 * Use this for all icon-only interactive controls (mic toggle, close button, etc.)
 */
export const IconButton: React.FC<IconButtonProps> = ({
  label,
  variant = "default",
  size = "md",
  isActive = false,
  activeTone = "accent",
  tooltip,
  tooltipSide = "top",
  className = "",
  type = "button",
  children,
  ...props
}) => {
  const stateClass = isActive
    ? activeTone === "danger"
      ? styles.activeDanger
      : styles.active
    : styles[variant];

  const cls = [styles.btn, stateClass, styles[size], className]
    .filter(Boolean)
    .join(" ");

  const button = (
    <button type={type} aria-label={label} className={cls} {...props}>
      {children}
    </button>
  );

  if (tooltip === false) return button;

  return (
    <Tooltip
      content={tooltip ?? label}
      side={tooltipSide}
      disabledTrigger={!!props.disabled}
    >
      {button}
    </Tooltip>
  );
};
