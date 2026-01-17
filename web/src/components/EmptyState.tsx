import React from 'react';
import './EmptyState.css';

interface EmptyStateProps {
  filter: string;
  selectedDate?: string;
  onAction?: () => void;
}

interface EmptyStateContent {
  icon: string;
  title: string;
  message: string;
  actionLabel?: string;
}

const getEmptyStateContent = (filter: string, selectedDate?: string): EmptyStateContent => {
  if (selectedDate) {
    return {
      icon: 'calendar',
      title: 'No images for this date',
      message: 'There are no images matching your filters for the selected date.',
    };
  }

  switch (filter) {
    case 'unreviewed':
      return {
        icon: 'check-circle',
        title: 'All caught up!',
        message: 'You have reviewed all images. Great job!',
        actionLabel: 'View approved images',
      };
    case 'approved':
      return {
        icon: 'thumbs-up',
        title: 'No approved images',
        message: 'Images you approve will appear here. Start reviewing to add some!',
      };
    case 'rejected':
      return {
        icon: 'thumbs-down',
        title: 'No rejected images',
        message: 'Images you reject will appear here.',
      };
    case 'deleted':
      return {
        icon: 'trash',
        title: 'Trash is empty',
        message: 'Deleted images will appear here for 90 days before permanent removal.',
      };
    default:
      return {
        icon: 'image',
        title: 'No images found',
        message: 'Upload images to your S3 bucket to get started with reviewing.',
      };
  }
};

const IconComponent: React.FC<{ name: string }> = ({ name }) => {
  const icons: Record<string, React.ReactElement> = {
    'check-circle': (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <circle cx="12" cy="12" r="10" />
        <path d="M9 12l2 2 4-4" />
      </svg>
    ),
    'thumbs-up': (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M14 9V5a3 3 0 0 0-3-3l-4 9v11h11.28a2 2 0 0 0 2-1.7l1.38-9a2 2 0 0 0-2-2.3zM7 22H4a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2h3" />
      </svg>
    ),
    'thumbs-down': (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M10 15v4a3 3 0 0 0 3 3l4-9V2H5.72a2 2 0 0 0-2 1.7l-1.38 9a2 2 0 0 0 2 2.3zm7-13h2.67A2.31 2.31 0 0 1 22 4v7a2.31 2.31 0 0 1-2.33 2H17" />
      </svg>
    ),
    'trash': (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
      </svg>
    ),
    'image': (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
        <circle cx="8.5" cy="8.5" r="1.5" />
        <path d="M21 15l-5-5L5 21" />
      </svg>
    ),
    'calendar': (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
        <line x1="16" y1="2" x2="16" y2="6" />
        <line x1="8" y1="2" x2="8" y2="6" />
        <line x1="3" y1="10" x2="21" y2="10" />
      </svg>
    ),
  };

  return <div className="empty-state-icon">{icons[name] || icons['image']}</div>;
};

export const EmptyState: React.FC<EmptyStateProps> = ({
  filter,
  selectedDate,
  onAction,
}) => {
  const content = getEmptyStateContent(filter, selectedDate);

  return (
    <div className="empty-state">
      <IconComponent name={content.icon} />
      <h3 className="empty-state-title">{content.title}</h3>
      <p className="empty-state-message">{content.message}</p>
      {content.actionLabel && onAction && (
        <button className="empty-state-action" onClick={onAction}>
          {content.actionLabel}
        </button>
      )}
    </div>
  );
};
