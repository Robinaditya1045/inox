import React, { type ButtonHTMLAttributes } from 'react';
import styles from './IconButton.module.css';

export type IconButtonVariant = 'default' | 'ghost' | 'danger' | 'active';
export type IconButtonSize = 'sm' | 'md' | 'lg';

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** Required: accessible label for screen readers */
  label: string;
  variant?: IconButtonVariant;
  size?: IconButtonSize;
  isActive?: boolean;
}

/**
 * Icon-only button. Always requires a `label` prop for aria-label.
 * Use this for all icon-only interactive controls (mic toggle, close button, etc.)
 */
export const IconButton: React.FC<IconButtonProps> = ({
  label,
  variant = 'default',
  size = 'md',
  isActive = false,
  className = '',
  children,
  ...props
}) => {
  const cls = [
    styles.btn,
    isActive ? styles.active : styles[variant],
    styles[size],
    className,
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <button
      aria-label={label}
      title={label}
      className={cls}
      {...props}
    >
      {children}
    </button>
  );
};
