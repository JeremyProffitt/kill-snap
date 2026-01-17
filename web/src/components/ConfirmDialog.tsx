import React, { useState, useEffect, useCallback } from 'react';
import './ConfirmDialog.css';

interface ConfirmDialogProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  confirmVariant?: 'danger' | 'primary' | 'warning';
  onConfirm: () => void;
  onCancel: () => void;
  showDontAskAgain?: boolean;
  onDontAskAgain?: (checked: boolean) => void;
}

export const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  isOpen,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  confirmVariant = 'danger',
  onConfirm,
  onCancel,
  showDontAskAgain = false,
  onDontAskAgain,
}) => {
  const [dontAsk, setDontAsk] = useState(false);

  // Handle keyboard events
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (!isOpen) return;
    
    if (e.key === 'Escape') {
      e.preventDefault();
      onCancel();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (dontAsk && onDontAskAgain) {
        onDontAskAgain(true);
      }
      onConfirm();
    }
  }, [isOpen, dontAsk, onDontAskAgain, onConfirm, onCancel]);

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);

  // Reset dontAsk when dialog opens
  useEffect(() => {
    if (isOpen) {
      setDontAsk(false);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleConfirm = () => {
    if (dontAsk && onDontAskAgain) {
      onDontAskAgain(true);
    }
    onConfirm();
  };

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onCancel();
    }
  };

  return (
    <div className="confirm-dialog-overlay" onClick={handleBackdropClick}>
      <div className="confirm-dialog">
        <h3 className="confirm-dialog-title">{title}</h3>
        <p className="confirm-dialog-message">{message}</p>
        
        {showDontAskAgain && (
          <label className="confirm-dialog-checkbox">
            <input
              type="checkbox"
              checked={dontAsk}
              onChange={(e) => setDontAsk(e.target.checked)}
            />
            <span>Don't ask again</span>
          </label>
        )}
        
        <div className="confirm-dialog-buttons">
          <button
            className="confirm-dialog-btn cancel"
            onClick={onCancel}
          >
            {cancelLabel}
          </button>
          <button
            className={`confirm-dialog-btn confirm ${confirmVariant}`}
            onClick={handleConfirm}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
};
