import React, { type InputHTMLAttributes, forwardRef } from 'react';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  icon?: React.ReactNode;
  helperText?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, icon, helperText, className = '', style, id, ...props }, ref) => {
    const inputId = id || (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined);

    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', width: '100%' }}>
        {label && (
          <label
            htmlFor={inputId}
            style={{
              fontSize: '0.875rem',
              fontWeight: 500,
              color: 'var(--color-text-secondary)',
              fontFamily: 'var(--font-sans)',
            }}
          >
            {label}
          </label>
        )}

        <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
          {icon && (
            <span
              style={{
                position: 'absolute',
                left: '12px',
                color: 'var(--color-text-muted)',
                display: 'flex',
                alignItems: 'center',
                pointerEvents: 'none',
              }}
            >
              {icon}
            </span>
          )}

          <input
            ref={ref}
            id={inputId}
            style={{
              width: '100%',
              padding: icon ? '12px 14px 12px 40px' : '12px 14px',
              background: 'var(--color-bg-surface)',
              border: `1px solid ${error ? 'var(--color-accent-rose)' : 'var(--color-border-glass)'}`,
              borderRadius: '8px',
              color: 'var(--color-text-primary)',
              fontSize: '0.95rem',
              fontFamily: 'var(--font-sans)',
              transition: 'all var(--transition-fast)',
              outline: 'none',
              ...style,
            }}
            onFocus={(e) => {
              e.target.style.borderColor = error ? 'var(--color-accent-rose)' : 'var(--color-accent-purple)';
              e.target.style.boxShadow = error
                ? '0 0 0 3px rgba(244, 63, 94, 0.2)'
                : '0 0 0 3px var(--color-accent-purple-glow)';
              props.onFocus?.(e);
            }}
            onBlur={(e) => {
              e.target.style.borderColor = error ? 'var(--color-accent-rose)' : 'var(--color-border-glass)';
              e.target.style.boxShadow = 'none';
              props.onBlur?.(e);
            }}
            className={`input-glass ${className}`}
            {...props}
          />
        </div>

        {error && (
          <span style={{ fontSize: '0.8rem', color: 'var(--color-accent-rose)', fontWeight: 500 }}>
            {error}
          </span>
        )}
        {!error && helperText && (
          <span style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>{helperText}</span>
        )}
      </div>
    );
  }
);

Input.displayName = 'Input';
