"use client";

import type { ReactNode } from "react";

export function ModalShell({
  open,
  title,
  children,
  onClose,
  wide,
  closeOnBackdrop = true,
}: {
  open: boolean;
  title: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
  closeOnBackdrop?: boolean;
}) {
  if (!open) return null;

  return (
    <div
      className="modal-backdrop"
      onClick={closeOnBackdrop ? onClose : undefined}
      role="presentation"
    >
      <div
        className={`modal card${wide ? " modal-wide" : ""}`}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-labelledby="modal-title"
      >
        <h2 id="modal-title" style={{ marginBottom: "1rem" }}>
          {title}
        </h2>
        {children}
      </div>
    </div>
  );
}
