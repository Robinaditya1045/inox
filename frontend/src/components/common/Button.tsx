import React, { type ButtonHTMLAttributes } from "react";
import { Spinner } from "./Spinner";
import styles from "./Button.module.css";

export type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";
export type ButtonSize = "sm" | "md" | "lg";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  isLoading?: boolean;
  /** Leading icon */
  icon?: React.ReactNode;
  /** Trailing icon, for forward actions ("Continue →") */
  trailingIcon?: React.ReactNode;
  fullWidth?: boolean;
  ref?: React.Ref<HTMLButtonElement>;
}

export const Button: React.FC<ButtonProps> = ({
  children,
  variant = "primary",
  size = "md",
  isLoading = false,
  icon,
  trailingIcon,
  fullWidth = false,
  className = "",
  disabled,
  type = "button",
  ...props
}) => {
  const cls = [
    styles.btn,
    styles[variant],
    styles[size],
    fullWidth ? styles.fullWidth : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <button
      type={type}
      disabled={disabled || isLoading}
      aria-busy={isLoading || undefined}
      data-loading={isLoading || undefined}
      className={cls}
      {...props}
    >
      <span className={styles.content}>
        {icon && (
          <span className={styles.icon} aria-hidden="true">
            {icon}
          </span>
        )}
        {children}
        {trailingIcon && (
          <span className={styles.icon} aria-hidden="true">
            {trailingIcon}
          </span>
        )}
      </span>
      {isLoading && (
        <span className={styles.spinner}>
          <Spinner
            size={size === "sm" ? 14 : 16}
            color="currentColor"
            label={null}
          />
        </span>
      )}
    </button>
  );
};
