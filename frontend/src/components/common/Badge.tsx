import React from 'react';

export type BadgeVariant = 'default' | 'accent' | 'success' | 'warning' | 'danger' | 'live';

interface BadgeProps {
  children: React.ReactNode;
  variant?: BadgeVariant;
  className?: string;
}

const variantStyles: Record<BadgeVariant, React.CSSProperties> = {
  default: {
    background: 'var(--color-surface-3)',
    color: 'var(--color-text-secondary)',
    border: '1px solid var(--color-border-default)',
  },
  accent: {
    background: 'var(--color-accent-subtle)',
    color: 'var(--color-accent)',
    border: '1px solid var(--color-accent-border)',
  },
  success: {
    background: 'var(--color-success-subtle)',
    color: 'var(--color-success)',
    border: '1px solid var(--color-success-border)',
  },
  warning: {
    background: 'var(--color-warning-subtle)',
    color: 'var(--color-warning)',
    border: '1px solid var(--color-warning-border)',
  },
  danger: {
    background: 'var(--color-danger-subtle)',
    color: 'var(--color-danger)',
    border: '1px solid var(--color-danger-border)',
  },
  live: {
    background: 'var(--color-success-subtle)',
    color: 'var(--color-live)',
    border: '1px solid var(--color-success-border)',
  },
};

export const Badge: React.FC<BadgeProps> = ({
  children,
  variant = 'default',
  className = '',
}) => {
  return (
    <span
      className={className}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '3px',
        padding: '2px 7px',
        borderRadius: 'var(--radius-full)',
        fontSize: 'var(--text-label)',
        fontWeight: 600,
        letterSpacing: '0.03em',
        lineHeight: 1.5,
        whiteSpace: 'nowrap',
        ...variantStyles[variant],
      }}
    >
      {children}
    </span>
  );
};
