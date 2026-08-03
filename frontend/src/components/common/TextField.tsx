import React, { type InputHTMLAttributes, forwardRef, useId } from 'react';
import styles from './TextField.module.css';

interface TextFieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  icon?: React.ReactNode;
  helperText?: string;
}

export const TextField = forwardRef<HTMLInputElement, TextFieldProps>(
  ({ label, error, icon, helperText, className = '', id, ...props }, ref) => {
    const generatedId = useId();
    const inputId = id || generatedId;

    return (
      <div className={`${styles.container} ${className}`}>
        {label && (
          <label htmlFor={inputId} className={styles.label}>
            {label}
          </label>
        )}

        <div className={styles.inputWrapper}>
          {icon && <span className={styles.icon}>{icon}</span>}
          <input
            ref={ref}
            id={inputId}
            className={`${styles.input} ${icon ? styles.inputWithIcon : ''} ${error ? styles.inputError : ''}`}
            aria-invalid={!!error}
            aria-describedby={
              error ? `${inputId}-error` : helperText ? `${inputId}-help` : undefined
            }
            {...props}
          />
        </div>

        {error && (
          <span id={`${inputId}-error`} className={styles.errorText}>
            {error}
          </span>
        )}
        {!error && helperText && (
          <span id={`${inputId}-help`} className={styles.helperText}>
            {helperText}
          </span>
        )}
      </div>
    );
  }
);

TextField.displayName = 'TextField';

// Export as Input to temporarily satisfy old imports while we replace them
export const Input = TextField;
