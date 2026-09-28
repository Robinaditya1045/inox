import React, { type CSSProperties, type ReactNode } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import styles from "./Modal.module.css";

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  /** Optional supporting line under the title, announced with the dialog */
  description?: ReactNode;
  children: ReactNode;
  maxWidth?: string;
  /** Drop the body padding for layouts that manage their own (e.g. settings) */
  flush?: boolean;
}

/**
 * Dialog built on Radix: focus is trapped and restored, the page behind stops
 * scrolling, Escape and the backdrop close it, and it animates both in and out.
 * On phones it becomes a bottom sheet.
 */
export const Modal: React.FC<ModalProps> = ({
  isOpen,
  onClose,
  title,
  description,
  children,
  maxWidth = "480px",
  flush = false,
}) => {
  return (
    <Dialog.Root
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className={styles.overlay} />
        <Dialog.Content
          className={styles.content}
          style={{ "--modal-max-width": maxWidth } as CSSProperties}
          {...(description ? {} : { "aria-describedby": undefined })}
        >
          <div className={styles.header}>
            <div className={styles.titles}>
              <Dialog.Title className={styles.title}>{title}</Dialog.Title>
              {description && (
                <Dialog.Description className={styles.description}>
                  {description}
                </Dialog.Description>
              )}
            </div>
            <Dialog.Close className={styles.closeBtn} aria-label="Close">
              <X size={18} aria-hidden="true" />
            </Dialog.Close>
          </div>

          <div className={flush ? styles.bodyFlush : styles.body}>
            {children}
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
};
