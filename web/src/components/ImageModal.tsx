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

// Lightroom color labels: Red, Yellow, Green, Blue, Purple
const GROUP_COLORS = [
  { number: 0, color: '#ffffff', name: 'None', textColor: '#333' },
  { number: 1, color: '#e74c3c', name: 'Red', textColor: '#fff' },
  { number: 2, color: '#f1c40f', name: 'Yellow', textColor: '#333' },
  { number: 3, color: '#2ecc71', name: 'Green', textColor: '#fff' },
  { number: 4, color: '#3498db', name: 'Blue', textColor: '#fff' },
  { number: 5, color: '#9b59b6', name: 'Purple', textColor: '#fff' },
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
  const [rating, setRating] = useState<number>(0);
  const [keywords, setKeywords] = useState<string[]>([]);
  const [newKeyword, setNewKeyword] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [regeneratingAI, setRegeneratingAI] = useState(false);
  const [description, setDescription] = useState<string>('');

  const isDeleted = image.status === 'deleted';

  // Reset group, rating, keywords, and description when image changes
  useEffect(() => {
    setGroupNumber(image.groupNumber || 0);
    setRating(image.rating || 0);
    setKeywords(image.keywords || []);
    setDescription(image.description || '');
    setNewKeyword('');
  }, [image.imageGUID, image.groupNumber, image.rating, image.keywords, image.description]);

  const handleApprove = useCallback(async () => {
    setLoading(true);
    try {
      const group = GROUP_COLORS.find(g => g.number === groupNumber);
      await api.updateImage(image.imageGUID, {
        groupNumber,
        colorCode: group?.name.toLowerCase() || 'none',
        rating,
        keywords,
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
  }, [groupNumber, rating, keywords, image.imageGUID, onUpdate]);

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

  const handleUndelete = useCallback(async () => {
    setLoading(true);
    try {
      await api.undeleteImage(image.imageGUID);
      onUpdate();
    } catch (err) {
      console.error('Failed to undelete image:', err);
      alert('Failed to undelete image');
    } finally {
      setLoading(false);
    }
  }, [image.imageGUID, onUpdate]);

  const handleRegenerateAI = useCallback(async () => {
    setRegeneratingAI(true);
    try {
      const result = await api.regenerateAI(image.imageGUID);
      setKeywords(result.keywords);
      setDescription(result.description);
    } catch (err: any) {
      console.error('Failed to regenerate AI content:', err);
      const message = err.response?.data?.error || 'Failed to regenerate AI content';
      alert(message);
    } finally {
      setRegeneratingAI(false);
    }
  }, [image.imageGUID]);

  const handleGroupSelect = useCallback((num: number) => {
    if (!loading) {
      setGroupNumber(num);
    }
  }, [loading]);

  const handleRatingSelect = useCallback((stars: number) => {
    if (!loading) {
      setRating(stars);
    }
  }, [loading]);

  const handleAddKeyword = useCallback(() => {
    const trimmed = newKeyword.trim();
    if (trimmed && !keywords.includes(trimmed) && !loading) {
      setKeywords([...keywords, trimmed]);
      setNewKeyword('');
    }
  }, [newKeyword, keywords, loading]);

  const handleRemoveKeyword = useCallback((keyword: string) => {
    if (!loading) {
      setKeywords(keywords.filter(k => k !== keyword));
    }
  }, [keywords, loading]);

  const handleKeywordKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      e.stopPropagation();
      handleAddKeyword();
    }
  }, [handleAddKeyword]);

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

      // Number keys 0-5 for group selection (Lightroom colors)
      if (key >= '0' && key <= '5' && !isDeleted) {
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

      // Enter to approve (or undelete if deleted)
      if (key === 'Enter') {
        e.preventDefault();
        if (isDeleted) {
          handleUndelete();
        } else {
          handleApprove();
        }
        return;
      }

      // 'r' to reject
      if ((key === 'r' || key === 'R') && !isDeleted) {
        e.preventDefault();
        handleReject();
        return;
      }

      // 'd' or Delete to delete
      if ((key === 'd' || key === 'D' || key === 'Delete') && !isDeleted) {
        e.preventDefault();
        handleDelete();
        return;
      }

      // 'u' to undelete
      if ((key === 'u' || key === 'U') && isDeleted) {
        e.preventDefault();
        handleUndelete();
        return;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleGroupSelect, handleApprove, handleReject, handleDelete, handleUndelete, handlePrev, handleNext, onClose, groupNumber, isDeleted]);

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  return (
    <div className="modal-backdrop" onClick={handleBackdropClick}>
      <div className="modal-content">
        <button className="close-button" onClick={onClose} disabled={loading}>
          ×
        </button>

        <div className="modal-body">
          <div className="modal-header">
            <h2 className="modal-title">Review Image</h2>
            <div className="image-counter">
              {currentIndex + 1} / {totalImages}
            </div>
          </div>

          <div className="image-preview-container">
            {/* Left Navigation Arrow */}
            <button
              className={`image-nav-arrow image-nav-prev ${!hasPrev ? 'disabled' : ''}`}
              onClick={handlePrev}
              disabled={!hasPrev || loading}
              title="Previous (←)"
            >
              ‹
            </button>

            <div className="image-preview">
              <img
                src={api.getImageUrl(image.bucket, image.thumbnail400)}
                alt={image.originalFile}
              />
            </div>

            {/* Right Navigation Arrow */}
            <button
              className={`image-nav-arrow image-nav-next ${!hasNext ? 'disabled' : ''}`}
              onClick={handleNext}
              disabled={!hasNext || loading}
              title="Next (→)"
            >
              ›
            </button>
          </div>

          {/* Image info bar: filename left, actions center, dimensions right */}
          <div className="image-info-bar">
            <div className="info-left">
              <span className="info-filename">{getFilename(image.originalFile)}</span>
            </div>
            <div className="info-center">
              {isDeleted ? (
                <button
                  type="button"
                  className="action-btn undelete"
                  onClick={handleUndelete}
                  disabled={loading}
                  title="Undelete (U)"
                >
                  ↩ Undelete
                </button>
              ) : (
                <>
                  <button
                    type="button"
                    className="action-btn approve"
                    onClick={handleApprove}
                    disabled={loading}
                    title="Approve (Enter)"
                  >
                    ✓
                  </button>
                  <button
                    type="button"
                    className="action-btn reject"
                    onClick={handleReject}
                    disabled={loading}
                    title="Reject (R)"
                  >
                    ✗
                  </button>
                  <button
                    type="button"
                    className="action-btn delete"
                    onClick={handleDelete}
                    disabled={loading}
                    title="Delete (D)"
                  >
                    🗑
                  </button>
                </>
              )}
            </div>
            <div className="info-right">
              <span className="info-dimensions">{image.width}×{image.height}</span>
              <span className="info-filesize">{formatFileSize(image.fileSize)}</span>
            </div>
          </div>

          {/* Controls row: colors left, rating right */}
          {!isDeleted && (
            <div className="controls-row">
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
              <div className="rating-stars">
                {[1, 2, 3, 4, 5].map((star) => (
                  <button
                    key={star}
                    className={`star-btn ${rating >= star ? 'filled' : 'empty'}`}
                    onClick={() => handleRatingSelect(star === rating ? 0 : star)}
                    disabled={loading}
                    title={`${star} star${star > 1 ? 's' : ''}`}
                  >
                    {rating >= star ? '★' : '☆'}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div className="keywords-section">
            <div className="keywords-header">
              <span className="keywords-label">Keywords:</span>
              <button
                className="regenerate-ai-btn"
                onClick={handleRegenerateAI}
                disabled={loading || regeneratingAI}
                title="Regenerate AI keywords and description"
              >
                {regeneratingAI ? 'Analyzing...' : 'Regenerate AI'}
              </button>
            </div>
            <div className="keywords-container">
              {keywords.map((keyword) => (
                <span key={keyword} className="keyword-tag">
                  {keyword}
                  <button
                    type="button"
                    className="keyword-remove"
                    onClick={() => handleRemoveKeyword(keyword)}
                    disabled={loading}
                  >
                    ×
                  </button>
                </span>
              ))}
            </div>
            <div className="keyword-input-row">
              <input
                type="text"
                value={newKeyword}
                onChange={(e) => setNewKeyword(e.target.value)}
                onKeyDown={handleKeywordKeyDown}
                placeholder="Add keyword..."
                disabled={loading}
                className="keyword-input"
              />
              <button
                type="button"
                onClick={handleAddKeyword}
                disabled={loading || !newKeyword.trim()}
                className="keyword-add-btn"
              >
                Add
              </button>
            </div>
          </div>

          <div className="description-section">
            <div className="description-label">AI Description:</div>
            {description ? (
              <p className="description-text">{description}</p>
            ) : (
              <p className="description-placeholder">No AI description yet. Click "Regenerate AI" to generate.</p>
            )}
          </div>

          <div className="keyboard-hints">
            <span>← → Nav</span>
            <span>0-5 Color</span>
            <span>Enter {isDeleted ? 'Undelete' : 'Approve'}</span>
            {!isDeleted && <span>R Reject</span>}
            {!isDeleted && <span>D Delete</span>}
            {isDeleted && <span>U Undelete</span>}
            <span>Esc Close</span>
          </div>
        </div>
      </div>
    </div>
  );
};
