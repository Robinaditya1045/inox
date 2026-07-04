import React, { type ButtonHTMLAttributes } from 'react';
import { Spinner } from './Spinner';

export type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost';
export type ButtonSize = 'sm' | 'md' | 'lg';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  isLoading?: boolean;
  icon?: React.ReactNode;
  fullWidth?: boolean;
}

export const Button: React.FC<ButtonProps> = ({
  children,
  variant = 'primary',
  size = 'md',
  isLoading = false,
  icon,
  fullWidth = false,
  className = '',
  disabled,
  style,
  ...props
}) => {
  const getVariantStyles = (): React.CSSProperties => {
    switch (variant) {
      case 'primary':
        return {
          background: 'linear-gradient(135deg, var(--color-accent-purple) 0%, #8B5CF6 100%)',
          color: 'var(--color-text-on-accent)',
          boxShadow: '0 0 15px var(--color-accent-purple-glow)',
          border: '1px solid rgba(255, 255, 255, 0.2)',
        };
      case 'secondary':
        return {
          background: 'var(--color-bg-glass)',
          color: 'var(--color-text-primary)',
          border: '1px solid var(--color-border-hover)',
          backdropFilter: 'blur(12px)',
        };
      case 'danger':
        return {
          background: 'var(--color-accent-rose)',
          color: 'var(--color-text-on-accent)',
          boxShadow: '0 0 15px rgba(244, 63, 94, 0.3)',
          border: '1px solid rgba(255, 255, 255, 0.2)',
        };
      case 'ghost':
        return {
          background: 'transparent',
          color: 'var(--color-text-secondary)',
          border: '1px solid transparent',
        };
    }
  };

  const getSizeStyles = (): React.CSSProperties => {
    switch (size) {
      case 'sm':
        return { padding: '6px 12px', fontSize: '0.875rem', borderRadius: '6px', gap: '6px' };
      case 'md':
        return { padding: '10px 18px', fontSize: '0.95rem', borderRadius: '8px', gap: '8px' };
      case 'lg':
        return { padding: '14px 24px', fontSize: '1.1rem', borderRadius: '10px', gap: '10px' };
    }
  };

  const baseStyles: React.CSSProperties = {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontFamily: 'var(--font-sans)',
    fontWeight: 600,
    cursor: disabled || isLoading ? 'not-allowed' : 'pointer',
    opacity: disabled || isLoading ? 0.6 : 1,
    transition: 'all var(--transition-fast)',
    width: fullWidth ? '100%' : 'auto',
    position: 'relative',
    ...getSizeStyles(),
    ...getVariantStyles(),
    ...style,
  };

  return (
    <button
      disabled={disabled || isLoading}
      style={baseStyles}
      className={`btn btn-${variant} ${className}`}
      {...props}
    >
      {isLoading && <Spinner size={size === 'sm' ? 16 : 20} color="#FFFFFF" />}
      {!isLoading && icon && <span style={{ display: 'flex', alignItems: 'center' }}>{icon}</span>}
      <span>{children}</span>
    </button>
  );
};
