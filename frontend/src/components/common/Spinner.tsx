import React from 'react';

interface SpinnerProps {
  size?: number;
  color?: string;
  className?: string;
}

export const Spinner: React.FC<SpinnerProps> = ({
  size = 24,
  color = 'var(--color-accent-purple)',
  className = '',
}) => {
  return (
    <div
      className={`spinner ${className}`}
      style={{
        width: `${size}px`,
        height: `${size}px`,
        border: `3px solid rgba(255, 255, 255, 0.1)`,
        borderTopColor: color,
        borderRadius: '50%',
        animation: 'spin 0.8s linear infinite',
      }}
      role="status"
      aria-label="Loading"
    >
      <style>{`
        @keyframes spin {
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
};
