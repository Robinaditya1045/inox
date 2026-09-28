import React, { type ReactNode } from "react";
import { AlertCircle, CheckCircle2, Info } from "lucide-react";
import styles from "./Alert.module.css";

interface AlertProps {
  tone?: "danger" | "success" | "info";
  children: ReactNode;
  className?: string;
}

const ICONS = { danger: AlertCircle, success: CheckCircle2, info: Info };

/** Inline message block. Errors are announced assertively, the rest politely. */
export const Alert: React.FC<AlertProps> = ({
  tone = "danger",
  children,
  className = "",
}) => {
  const Icon = ICONS[tone];
  return (
    <div
      className={`${styles.alert} ${styles[tone]} ${className}`}
      role={tone === "danger" ? "alert" : "status"}
    >
      <span className={styles.icon} aria-hidden="true">
        <Icon size={16} />
      </span>
      <div className={styles.body}>{children}</div>
    </div>
  );
};
