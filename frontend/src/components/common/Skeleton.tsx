import React, { type CSSProperties } from "react";
import styles from "./Skeleton.module.css";

interface SkeletonProps {
  width?: number | string;
  height?: number | string;
  circle?: boolean;
  radius?: string;
  className?: string;
  style?: CSSProperties;
}

/**
 * Placeholder block shaped like the content it stands in for. Always decorative:
 * wrap a group of them in an element with aria-busy and a label instead.
 */
export const Skeleton: React.FC<SkeletonProps> = ({
  width = "100%",
  height = 12,
  circle = false,
  radius,
  className = "",
  style,
}) => (
  <span
    aria-hidden="true"
    className={`${styles.skeleton} ${circle ? styles.circle : ""} ${className}`}
    style={{ width, height, borderRadius: radius, ...style }}
  />
);
