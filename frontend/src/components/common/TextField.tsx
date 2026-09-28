import React, { type InputHTMLAttributes, useId, useState } from "react";
import { AlertCircle, Eye, EyeOff } from "lucide-react";
import styles from "./TextField.module.css";

interface TextFieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string | null;
  icon?: React.ReactNode;
  helperText?: string;
  /** For type="password": adds a show/hide toggle inside the field */
  revealable?: boolean;
  ref?: React.Ref<HTMLInputElement>;
}

export const TextField: React.FC<TextFieldProps> = ({
  label,
  error,
  icon,
  helperText,
  revealable = false,
  className = "",
  id,
  type,
  required,
  ref,
  ...props
}) => {
  const generatedId = useId();
  const inputId = id || generatedId;
  const [revealed, setRevealed] = useState(false);

  const canReveal = revealable && type === "password";
  const effectiveType = canReveal && revealed ? "text" : type;

  const describedBy = error
    ? `${inputId}-error`
    : helperText
      ? `${inputId}-help`
      : undefined;

  const inputCls = [
    styles.input,
    icon ? styles.inputWithIcon : "",
    canReveal ? styles.inputWithTrailing : "",
    error ? styles.inputError : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={`${styles.container} ${className}`}>
      {label && (
        <label htmlFor={inputId} className={styles.label}>
          {label}
          {required && (
            <span className={styles.required} aria-hidden="true">
              *
            </span>
          )}
        </label>
      )}

      <div className={styles.inputWrapper}>
        {icon && (
          <span className={styles.icon} aria-hidden="true">
            {icon}
          </span>
        )}
        <input
          ref={ref}
          id={inputId}
          type={effectiveType}
          required={required}
          className={inputCls}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          {...props}
        />
        {canReveal && (
          <span className={styles.trailing}>
            <button
              type="button"
              className={styles.revealBtn}
              onClick={() => setRevealed((r) => !r)}
              aria-label={revealed ? "Hide password" : "Show password"}
              aria-pressed={revealed}
              aria-controls={inputId}
            >
              {revealed ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          </span>
        )}
      </div>

      {error && (
        <span id={`${inputId}-error`} className={styles.errorText} role="alert">
          <AlertCircle size={13} aria-hidden="true" />
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
};

// Export as Input to temporarily satisfy old imports while we replace them
export const Input = TextField;
