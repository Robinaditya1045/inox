import React from 'react';

interface SpinnerProps {
  size?: number;
  /** Defaults to --color-accent. Pass any CSS color value. */
  color?: string;
  className?: string;
}

export const Spinner: React.FC<SpinnerProps> = ({
  size = 24,
  color = 'var(--color-accent)',
  className = '',
}) => {
  return (
    <div
      className={className}
      style={{
        width: `${size}px`,
        height: `${size}px`,
        border: `2px solid var(--color-border-default)`,
        borderTopColor: color,
        borderRadius: '50%',
        animation: 'spin 0.8s linear infinite',
        flexShrink: 0,
      }}
      role="status"
      aria-label="Loading"
    />
  );
};
