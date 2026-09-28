import React, { type ReactNode } from "react";
import { Logo } from "../common/Logo";
import styles from "./AuthLayout.module.css";

interface AuthLayoutProps {
  title: string;
  subtitle?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
}

/** Shared frame for sign in, sign up and password recovery. */
export const AuthLayout: React.FC<AuthLayoutProps> = ({
  title,
  subtitle,
  children,
  footer,
}) => (
  <main className={styles.page}>
    <div className={styles.brand}>
      <Logo size={36} withWordmark />
    </div>
    <div className={styles.card}>
      <header className={styles.header}>
        <h1 className={styles.title}>{title}</h1>
        {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
      </header>
      {children}
      {footer && <div className={styles.footer}>{footer}</div>}
    </div>
  </main>
);
