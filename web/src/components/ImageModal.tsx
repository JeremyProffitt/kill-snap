import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../services/api';
import { Image } from '../types';
import './ImageModal.css';

interface ImageModalProps {
  image: Image;
  onClose: () => void;
  onUpdate: () => void;
  onNavigate: (direction: 'prev' | 'next') => void;
  hasPrev: boolean;
  hasNext: boolean;
  currentIndex: number;
  totalImages: number;
}

const GROUP_COLORS = [
  { number: 0, color: '#ffffff', name: 'None', textColor: '#333' },
  { number: 1, color: '#e74c3c', name: 'Red', textColor: '#fff' },
  { number: 2, color: '#3498db', name: 'Blue', textColor: '#fff' },
  { number: 3, color: '#2ecc71', name: 'Green', textColor: '#fff' },
  { number: 4, color: '#f1c40f', name: 'Yellow', textColor: '#333' },
  { number: 5, color: '#9b59b6', name: 'Purple', textColor: '#fff' },
  { number: 6, color: '#e67e22', name: 'Orange', textColor: '#fff' },
  { number: 7, color: '#e91e63', name: 'Pink', textColor: '#fff' },
  { number: 8, color: '#795548', name: 'Brown', textColor: '#fff' },
];

const formatFileSize = (bytes: number): string => {
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)}M`;
  }
  return `${(bytes / 1024).toFixed(1)}K`;
};

const getFilename = (path: string): string => {
  const parts = path.split('/');
  return parts[parts.length - 1];
};

export const ImageModal: React.FC<ImageModalProps> = ({
  image,
  onClose,
  onUpdate,
  onNavigate,
  hasPrev,
  hasNext,
  currentIndex,
  totalImages,
}) => {
  const [groupNumber, setGroupNumber] = useState<number>(0);
  const [loading, setLoading] = useState(false);

  // Reset group selection when image changes
  useEffect(() => {
    setGroupNumber(0);
  }, [image.imageGUID]);

  const handleApprove = useCallback(async () => {
    setLoading(true);
    try {
      const group = GROUP_COLORS.find(g => g.number === groupNumber);
      await api.updateImage(image.imageGUID, {
        groupNumber,
        colorCode: group?.name.toLowerCase() || 'none',
        promoted: false,
        reviewed: 'true',
      });
      onUpdate();
    } catch (err) {
      console.error('Failed to approve image:', err);
      alert('Failed to approve image');
    } finally {
      setLoading(false);
    }
  }, [groupNumber, image.imageGUID, onUpdate]);

  const handleReject = useCallback(async () => {
    setLoading(true);
    try {
      await api.updateImage(image.imageGUID, {
        groupNumber: 0,
        colorCode: 'none',
        reviewed: 'true',
      });
      onUpdate();
    } catch (err) {
      console.error('Failed to reject image:', err);
      alert('Failed to reject image');
    } finally {
      setLoading(false);
    }
  }, [image.imageGUID, onUpdate]);

  const handleDelete = useCallback(async () => {
    if (!window.confirm('Are you sure you want to delete this image?')) {
      return;
    }
    setLoading(true);
    try {
      await api.deleteImage(image.imageGUID);
      onUpdate();
    } catch (err) {
      console.error('Failed to delete image:', err);
      alert('Failed to delete image');
    } finally {
      setLoading(false);
    }
  }, [image.imageGUID, onUpdate]);

  const handleGroupSelect = useCallback((num: number) => {
    if (!loading) {
      setGroupNumber(num);
    }
  }, [loading]);

  const handlePrev = useCallback(() => {
    if (hasPrev && !loading) {
      onNavigate('prev');
    }
  }, [hasPrev, loading, onNavigate]);

  const handleNext = useCallback(() => {
    if (hasNext && !loading) {
      onNavigate('next');
    }
  }, [hasNext, loading, onNavigate]);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if user is typing in an input
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return;
      }

      const key = e.key;

      // Arrow keys for navigation
      if (key === 'ArrowLeft') {
        e.preventDefault();
        handlePrev();
        return;
      }
      if (key === 'ArrowRight') {
        e.preventDefault();
        handleNext();
        return;
      }

      // Number keys 0-8 for group selection
      if (key >= '0' && key <= '8') {
        e.preventDefault();
        handleGroupSelect(parseInt(key));
        return;
      }

      // Escape to close
      if (key === 'Escape') {
        e.preventDefault();
        onClose();
        return;
      }

      // Enter to approve
      if (key === 'Enter') {
        e.preventDefault();
        handleApprove();
        return;
      }

      // 'r' to reject
      if (key === 'r' || key === 'R') {
        e.preventDefault();
        handleReject();
        return;
      }

      // 'd' or Delete to delete
      if (key === 'd' || key === 'D' || key === 'Delete') {
        e.preventDefault();
        handleDelete();
        return;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleGroupSelect, handleApprove, handleReject, handleDelete, handlePrev, handleNext, onClose, groupNumber]);

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  return (
    <div className="modal-backdrop" onClick={handleBackdropClick}>
      {/* Left Navigation Arrow */}
      <button
        className={`nav-arrow nav-prev ${!hasPrev ? 'disabled' : ''}`}
        onClick={handlePrev}
        disabled={!hasPrev || loading}
        title="Previous (←)"
      >
        ‹
      </button>

      <div className="modal-content">
        <button className="close-button" onClick={onClose} disabled={loading}>
          ×
        </button>

        <div className="modal-body">
          <div className="image-counter">
            {currentIndex + 1} / {totalImages}
          </div>

          <div className="image-preview">
            <img
              src={api.getImageUrl(image.bucket, image.thumbnail400)}
              alt={image.originalFile}
            />
          </div>

          <div className="image-meta">
            <div className="meta-item">
              <span className="meta-label">File:</span>
              <span className="meta-value filename">{getFilename(image.originalFile)}</span>
            </div>
            <div className="meta-item">
              <span className="meta-label">Size:</span>
              <span className="meta-value">{image.width} × {image.height}px</span>
            </div>
            <div className="meta-item">
              <span className="meta-label">File Size:</span>
              <span className="meta-value">{formatFileSize(image.fileSize)}</span>
            </div>
          </div>

          <div className="group-section">
            <div className="group-label">Assign Group (0-8, 0=None/White):</div>
            <div className="group-buttons">
              {GROUP_COLORS.map((group) => (
                <button
                  key={group.number}
                  className={`group-btn ${groupNumber === group.number ? 'selected' : ''}`}
                  style={{
                    '--group-color': group.color,
                    '--group-text-color': group.textColor,
                  } as React.CSSProperties}
                  onClick={() => handleGroupSelect(group.number)}
                  disabled={loading}
                  title={`${group.name} (Press ${group.number})`}
                >
                  {group.number}
                </button>
              ))}
            </div>
          </div>

          <div className="action-section">
            <button
              onClick={handleApprove}
              disabled={loading}
              className="btn-approve"
              title="Approve (Enter)"
            >
              Approve
            </button>
            <button
              onClick={handleReject}
              disabled={loading}
              className="btn-reject"
              title="Reject (R)"
            >
              Reject
            </button>
            <button
              onClick={handleDelete}
              disabled={loading}
              className="btn-delete"
              title="Delete (D)"
            >
              Delete
            </button>
          </div>

          <div className="keyboard-hints">
            <span>← → Navigate</span>
            <span>0-8: Group</span>
            <span>Enter: Approve</span>
            <span>R: Reject</span>
            <span>D: Delete</span>
            <span>Esc: Close</span>
          </div>
        </div>
      </div>

      {/* Right Navigation Arrow */}
      <button
        className={`nav-arrow nav-next ${!hasNext ? 'disabled' : ''}`}
        onClick={handleNext}
        disabled={!hasNext || loading}
        title="Next (→)"
      >
        ›
      </button>
    </div>
  );
};
