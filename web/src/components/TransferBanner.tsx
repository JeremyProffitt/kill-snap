import React from 'react';
import './TransferBanner.css';

export interface TransferProgress {
  isActive: boolean;
  currentFile: string;
  currentIndex: number;
  totalCount: number;
  projectName: string;
  status: 'transferring' | 'complete' | 'error';
  errorMessage?: string;
}

interface TransferBannerProps {
  progress: TransferProgress;
  onDismiss?: () => void;
}

export const TransferBanner: React.FC<TransferBannerProps> = ({ progress, onDismiss }) => {
  if (!progress.isActive) return null;

  const percentage = progress.totalCount > 0
    ? Math.round((progress.currentIndex / progress.totalCount) * 100)
    : 0;

  const getStatusText = () => {
    if (progress.status === 'complete') {
      return `Successfully added ${progress.totalCount} image${progress.totalCount !== 1 ? 's' : ''} to "${progress.projectName}"`;
    }
    if (progress.status === 'error') {
      return progress.errorMessage || 'Transfer failed';
    }
    return `Adding to "${progress.projectName}": ${progress.currentIndex} of ${progress.totalCount} (${percentage}%)`;
  };

  const getFileName = (path: string): string => {
    const parts = path.split('/');
    return parts[parts.length - 1];
  };

  return (
    <div className={`transfer-banner ${progress.status}`}>
      <div className="transfer-banner-content">
        <div className="transfer-banner-text">
          <span className="transfer-status">{getStatusText()}</span>
          {progress.status === 'transferring' && progress.currentFile && (
            <span className="transfer-filename">{getFileName(progress.currentFile)}</span>
          )}
        </div>

        {progress.status === 'transferring' && (
          <div className="transfer-progress-bar">
            <div
              className="transfer-progress-fill"
              style={{ width: `${percentage}%` }}
            />
          </div>
        )}

        {(progress.status === 'complete' || progress.status === 'error') && onDismiss && (
          <button className="transfer-dismiss-btn" onClick={onDismiss}>
            &times;
          </button>
        )}
      </div>
    </div>
  );
};
