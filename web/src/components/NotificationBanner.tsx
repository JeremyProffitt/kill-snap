import React, { useEffect } from 'react';
import './NotificationBanner.css';

export interface Notification {
  id: string;
  message: string;
  type: 'success' | 'error' | 'info';
}

interface NotificationBannerProps {
  notification: Notification | null;
  onDismiss: () => void;
  autoHideMs?: number;
}

export const NotificationBanner: React.FC<NotificationBannerProps> = ({
  notification,
  onDismiss,
  autoHideMs = 15000,
}) => {
  useEffect(() => {
    if (notification && autoHideMs > 0) {
      const timer = setTimeout(() => {
        onDismiss();
      }, autoHideMs);
      return () => clearTimeout(timer);
    }
  }, [notification, autoHideMs, onDismiss]);

  if (!notification) return null;

  return (
    <div className={`notification-banner ${notification.type}`}>
      <div className="notification-banner-content">
        <span className="notification-message">{notification.message}</span>
        <button className="notification-dismiss-btn" onClick={onDismiss}>
          &times;
        </button>
      </div>
    </div>
  );
};
