import React, { useState, useEffect, useCallback, useRef } from 'react';
import { api } from '../services/api';
import { Image, Project } from '../types';
import { Filmstrip } from './Filmstrip';
import './ImageModal.css';

interface ImageModalProps {
  image: Image;
  images: Image[];
  projects: Project[];
  onClose: () => void;
  onUpdate: () => void;
  onNavigate: (direction: 'prev' | 'next') => void;
  onPropertyChange: (imageGUID: string, updates: Partial<Image>) => void;
  onNotify: (message: string, type: 'success' | 'error' | 'info') => void;
  onProjectsUpdate: () => void;
  hasPrev: boolean;
  hasNext: boolean;
  currentIndex: number;
  totalImages: number;
}

const GROUP_COLORS = [
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
  images,
  projects,
  onClose,
  onUpdate,
  onNavigate,
  onPropertyChange,
  onNotify,
  onProjectsUpdate,
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
  const [hoverRating, setHoverRating] = useState<number | null>(null);
  const [addingToProject, setAddingToProject] = useState(false);

  // Zoom and pan state
  const [zoom, setZoom] = useState(1);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
  
  // Touch state
  const lastPinchDistance = useRef<number | null>(null);
  const touchStart = useRef<{ x: number; y: number } | null>(null);

  const isDeleted = image.status === 'deleted';

  useEffect(() => {
    setGroupNumber(image.groupNumber || 0);
    setRating(image.rating || 0);
    setKeywords(image.keywords || []);
    setDescription(image.description || '');
    setNewKeyword('');
    setHoverRating(null);
    setZoom(1);
    setPan({ x: 0, y: 0 });
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
    } catch (err: any) {
      const errorMessage = err.response?.data?.error || 'Failed to approve image';
      onNotify(errorMessage, 'error');
    } finally {
      setLoading(false);
    }
  }, [groupNumber, rating, keywords, image.imageGUID, onUpdate, onNotify]);

  const handleReject = useCallback(async () => {
    setLoading(true);
    try {
      await api.updateImage(image.imageGUID, {
        groupNumber: 0,
        colorCode: 'none',
        reviewed: 'true',
      });
      onUpdate();
    } catch (err: any) {
      const errorMessage = err.response?.data?.error || 'Failed to reject image';
      onNotify(errorMessage, 'error');
    } finally {
      setLoading(false);
    }
  }, [image.imageGUID, onUpdate, onNotify]);

  const handleDelete = useCallback(async () => {
    setLoading(true);
    try {
      await api.deleteImage(image.imageGUID);
      onUpdate();
    } catch (err: any) {
      const errorMessage = err.response?.data?.error || 'Failed to delete image';
      onNotify(errorMessage, 'error');
    } finally {
      setLoading(false);
    }
  }, [image.imageGUID, onUpdate, onNotify]);

  const handleUndelete = useCallback(async () => {
    setLoading(true);
    try {
      await api.undeleteImage(image.imageGUID);
      onUpdate();
    } catch (err: any) {
      const errorMessage = err.response?.data?.error || 'Failed to undelete image';
      onNotify(errorMessage, 'error');
    } finally {
      setLoading(false);
    }
  }, [image.imageGUID, onUpdate, onNotify]);

  const handleRegenerateAI = useCallback(async () => {
    setRegeneratingAI(true);
    try {
      const result = await api.regenerateAI(image.imageGUID);
      setKeywords(result.keywords);
      setDescription(result.description);
      onPropertyChange(image.imageGUID, {
        keywords: result.keywords,
        description: result.description
      });
    } catch (err: any) {
      const message = err.response?.data?.error || 'Failed to regenerate AI content';
      onNotify(message, 'error');
    } finally {
      setRegeneratingAI(false);
    }
  }, [image.imageGUID, onPropertyChange, onNotify]);

  const handleGroupSelect = useCallback(async (num: number) => {
    if (!loading) {
      setGroupNumber(num);
      const group = GROUP_COLORS.find(g => g.number === num);
      const colorCode = group?.name.toLowerCase() || 'none';
      onPropertyChange(image.imageGUID, { groupNumber: num, colorCode });
      try {
        await api.updateImage(image.imageGUID, { groupNumber: num, colorCode });
      } catch (err) {
        console.error('Failed to save group:', err);
      }
    }
  }, [loading, image.imageGUID, onPropertyChange]);

  const handleRatingSelect = useCallback(async (stars: number) => {
    if (!loading) {
      setRating(stars);
      onPropertyChange(image.imageGUID, { rating: stars });
      try {
        await api.updateImage(image.imageGUID, { rating: stars });
      } catch (err) {
        console.error('Failed to save rating:', err);
      }
    }
  }, [loading, image.imageGUID, onPropertyChange]);

  const handleAddToProject = useCallback(async (projectId: string) => {
    if (!projectId || addingToProject) return;

    setAddingToProject(true);
    try {
      // Add to project (single image by its GUID)
      await api.addToProject(projectId, { group: image.groupNumber || 0, imageGUID: image.imageGUID });
      const project = projects.find(p => p.projectId === projectId);
      onNotify(`Added to ${project?.name || 'project'}`, 'success');
      onProjectsUpdate();
    } catch (err: any) {
      const message = err.response?.data?.error || 'Failed to add to project';
      onNotify(message, 'error');
    } finally {
      setAddingToProject(false);
    }
  }, [addingToProject, image, projects, onNotify, onProjectsUpdate]);

  const handleAddKeyword = useCallback(async () => {
    const trimmed = newKeyword.trim();
    if (trimmed && !keywords.includes(trimmed) && !loading) {
      const newKeywords = [...keywords, trimmed];
      setKeywords(newKeywords);
      setNewKeyword('');
      onPropertyChange(image.imageGUID, { keywords: newKeywords });
      try {
        await api.updateImage(image.imageGUID, { keywords: newKeywords });
      } catch (err) {
        console.error('Failed to save keywords:', err);
      }
    }
  }, [newKeyword, keywords, loading, image.imageGUID, onPropertyChange]);

  const handleRemoveKeyword = useCallback(async (keyword: string) => {
    if (!loading) {
      const newKeywords = keywords.filter(k => k !== keyword);
      setKeywords(newKeywords);
      onPropertyChange(image.imageGUID, { keywords: newKeywords });
      try {
        await api.updateImage(image.imageGUID, { keywords: newKeywords });
      } catch (err) {
        console.error('Failed to save keywords:', err);
      }
    }
  }, [keywords, loading, image.imageGUID, onPropertyChange]);

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

  // Zoom handlers
  const handleWheel = useCallback((e: React.WheelEvent) => {
    e.preventDefault();
    const delta = e.deltaY > 0 ? 0.9 : 1.1;
    setZoom(prev => Math.min(Math.max(prev * delta, 0.5), 5));
  }, []);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    if (zoom > 1) {
      setIsDragging(true);
      setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y });
    }
  }, [zoom, pan]);

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    if (isDragging) {
      setPan({
        x: e.clientX - dragStart.x,
        y: e.clientY - dragStart.y,
      });
    }
  }, [isDragging, dragStart]);

  const handleMouseUp = useCallback(() => {
    setIsDragging(false);
  }, []);

  const handleDoubleClick = useCallback(() => {
    if (zoom === 1) {
      setZoom(2);
    } else {
      setZoom(1);
      setPan({ x: 0, y: 0 });
    }
  }, [zoom]);

  const resetZoom = useCallback(() => {
    setZoom(1);
    setPan({ x: 0, y: 0 });
  }, []);

  // Touch handlers
  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    if (e.touches.length === 1) {
      touchStart.current = { x: e.touches[0].clientX, y: e.touches[0].clientY };
    } else if (e.touches.length === 2) {
      const distance = Math.hypot(
        e.touches[0].clientX - e.touches[1].clientX,
        e.touches[0].clientY - e.touches[1].clientY
      );
      lastPinchDistance.current = distance;
    }
  }, []);

  const handleTouchMove = useCallback((e: React.TouchEvent) => {
    if (e.touches.length === 2 && lastPinchDistance.current) {
      const distance = Math.hypot(
        e.touches[0].clientX - e.touches[1].clientX,
        e.touches[0].clientY - e.touches[1].clientY
      );
      const scale = distance / lastPinchDistance.current;
      setZoom(prev => Math.min(Math.max(prev * scale, 0.5), 5));
      lastPinchDistance.current = distance;
    }
  }, []);

  const handleTouchEnd = useCallback((e: React.TouchEvent) => {
    if (e.changedTouches.length === 1 && touchStart.current) {
      const deltaX = e.changedTouches[0].clientX - touchStart.current.x;
      const deltaY = e.changedTouches[0].clientY - touchStart.current.y;
      const minSwipe = 50;

      if (Math.abs(deltaX) > Math.abs(deltaY) && zoom === 1) {
        if (deltaX > minSwipe) handlePrev();
        else if (deltaX < -minSwipe) handleNext();
      }
    }
    touchStart.current = null;
    lastPinchDistance.current = null;
  }, [zoom, handlePrev, handleNext]);

  const handleFilmstripSelect = useCallback((imageGUID: string) => {
    const index = images.findIndex(img => img.imageGUID === imageGUID);
    if (index !== -1 && index !== currentIndex) {
      if (index < currentIndex) {
        for (let i = 0; i < currentIndex - index; i++) onNavigate('prev');
      } else {
        for (let i = 0; i < index - currentIndex; i++) onNavigate('next');
      }
    }
  }, [images, currentIndex, onNavigate]);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;

      const key = e.key;

      if (key === 'ArrowLeft') { e.preventDefault(); handlePrev(); return; }
      if (key === 'ArrowRight') { e.preventDefault(); handleNext(); return; }
      if (key >= '1' && key <= '5' && !isDeleted) { e.preventDefault(); handleGroupSelect(parseInt(key)); return; }
      if (key === 'Escape') { e.preventDefault(); onClose(); return; }
      if (key === 'Enter') { e.preventDefault(); isDeleted ? handleUndelete() : handleApprove(); return; }
      if ((key === 'r' || key === 'R') && !isDeleted) { e.preventDefault(); handleReject(); return; }
      if ((key === 'd' || key === 'D' || key === 'Delete') && !isDeleted) { e.preventDefault(); handleDelete(); return; }
      if ((key === 'u' || key === 'U') && isDeleted) { e.preventDefault(); handleUndelete(); return; }
      if (key === '+' || key === '=') { e.preventDefault(); setZoom(prev => Math.min(prev * 1.2, 5)); return; }
      if (key === '-') { e.preventDefault(); setZoom(prev => Math.max(prev * 0.8, 0.5)); return; }
      if (key === '0') { e.preventDefault(); resetZoom(); return; }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleGroupSelect, handleApprove, handleReject, handleDelete, handleUndelete, handlePrev, handleNext, onClose, isDeleted, resetZoom]);

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) onClose();
  };

  return (
    <div className="modal-backdrop" onClick={handleBackdropClick}>
      <div className="modal-content with-filmstrip">
        <button className="close-button" onClick={onClose} disabled={loading}>X</button>

        <div className="modal-body">
          <div className="modal-header">
            <h2 className="modal-title">Review Image</h2>
            <div className="image-counter">{currentIndex + 1} / {totalImages}</div>
          </div>

          <div 
            className="image-preview-container"
            onWheel={handleWheel}
            onMouseDown={handleMouseDown}
            onMouseMove={handleMouseMove}
            onMouseUp={handleMouseUp}
            onMouseLeave={handleMouseUp}
            onDoubleClick={handleDoubleClick}
            onTouchStart={handleTouchStart}
            onTouchMove={handleTouchMove}
            onTouchEnd={handleTouchEnd}
          >
            <button
              className={`image-nav-arrow image-nav-prev ${!hasPrev ? 'disabled' : ''}`}
              onClick={handlePrev}
              disabled={!hasPrev || loading}
              title="Previous"
            >
              &lt;
            </button>

            <div className="image-preview">
              <img
                src={api.getImageUrl(image.bucket, image.thumbnail400)}
                alt={image.originalFile}
                style={{
                  transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
                  cursor: zoom > 1 ? (isDragging ? 'grabbing' : 'grab') : 'zoom-in',
                  transition: isDragging ? 'none' : 'transform 0.1s ease',
                }}
                draggable={false}
              />
            </div>

            <button
              className={`image-nav-arrow image-nav-next ${!hasNext ? 'disabled' : ''}`}
              onClick={handleNext}
              disabled={!hasNext || loading}
              title="Next"
            >
              &gt;
            </button>

            {zoom !== 1 && (
              <div className="zoom-controls">
                <span className="zoom-level">{Math.round(zoom * 100)}%</span>
                <button onClick={resetZoom} className="zoom-reset-btn">Reset</button>
              </div>
            )}
          </div>

          <div className="action-buttons-row">
            {isDeleted ? (
              <button type="button" className="action-btn undelete" onClick={handleUndelete} disabled={loading} title="Undelete (U)">
                Undelete
              </button>
            ) : (
              <>
                <button type="button" className="action-btn approve" onClick={handleApprove} disabled={loading} title="Approve (Enter)">V</button>
                <button type="button" className="action-btn reject" onClick={handleReject} disabled={loading} title="Reject (R)">X</button>
                <button type="button" className="action-btn delete" onClick={handleDelete} disabled={loading} title="Delete (D)">D</button>
              </>
            )}
          </div>

          <div className="image-info-bar">
            <div className="info-left">
              <span className="info-filename">{getFilename(image.originalFile)}</span>
            </div>
            <div className="info-right">
              <span className="info-dimensions">{image.width}x{image.height} - {formatFileSize(image.fileSize)}</span>
            </div>
          </div>

          {!isDeleted && (
            <div className="controls-row">
              <div className="group-buttons">
                {GROUP_COLORS.map((group) => (
                  <button
                    key={group.number}
                    className={`group-btn ${groupNumber === group.number ? 'selected' : ''}`}
                    style={{ '--group-color': group.color, '--group-text-color': group.textColor } as React.CSSProperties}
                    onClick={() => handleGroupSelect(group.number)}
                    disabled={loading}
                    title={`${group.name} (${group.number})`}
                  >
                    {group.number}
                  </button>
                ))}
              </div>
              <div className="add-to-project-dropdown">
                <select
                  value=""
                  onChange={(e) => handleAddToProject(e.target.value)}
                  disabled={loading || addingToProject || projects.length === 0}
                  className="project-select"
                >
                  <option value="">{addingToProject ? 'Adding...' : 'Add to Project'}</option>
                  {projects.map((project) => (
                    <option key={project.projectId} value={project.projectId}>
                      {project.name}
                    </option>
                  ))}
                </select>
              </div>
              <div className="rating-stars" onMouseLeave={() => setHoverRating(null)}>
                {[1, 2, 3, 4, 5].map((star) => {
                  const displayRating = hoverRating !== null ? hoverRating : rating;
                  const isFilled = displayRating >= star;
                  return (
                    <button
                      key={star}
                      className={`star-btn ${isFilled ? 'filled' : 'empty'}`}
                      onClick={() => handleRatingSelect(star === rating ? 0 : star)}
                      onMouseEnter={() => setHoverRating(star)}
                      disabled={loading}
                      title={`${star} star${star > 1 ? 's' : ''}`}
                    >
                      {isFilled ? '*' : 'o'}
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          <div className="keywords-section">
            <div className="keywords-header"><span className="keywords-label">Keywords</span></div>
            <div className="keywords-list">
              {keywords.length > 0 ? (
                keywords.map((keyword, index) => (
                  <span key={keyword} className="keyword-item">
                    {keyword}
                    <button type="button" className="keyword-delete-btn" onClick={() => handleRemoveKeyword(keyword)} disabled={loading}>x</button>
                    {index < keywords.length - 1 && <span className="keyword-comma">,</span>}
                  </span>
                ))
              ) : (
                <span className="no-keywords">No keywords</span>
              )}
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
              <button type="button" onClick={handleAddKeyword} disabled={loading || !newKeyword.trim()} className="keyword-add-btn">Add</button>
              <button className="ai-generate-btn" onClick={handleRegenerateAI} disabled={loading || regeneratingAI}>
                {regeneratingAI ? 'Analyzing...' : 'Ask AI to Generate'}
              </button>
            </div>
          </div>

          <div className="description-section">
            <div className="description-label">AI Description:</div>
            {description ? (
              <p className="description-text">{description}</p>
            ) : (
              <p className="description-placeholder">No AI description yet.</p>
            )}
          </div>

          <div className="keyboard-hints">
            <span>←/→ Nav</span>
            <span>1-5 Color</span>
            <span>Enter {isDeleted ? 'Undelete' : 'Approve'}</span>
            {!isDeleted && <span>R Reject</span>}
            {!isDeleted && <span>D Delete</span>}
            {isDeleted && <span>U Undelete</span>}
            <span>+/- Zoom</span>
            <span>Esc Close</span>
          </div>
        </div>

        <Filmstrip
          images={images}
          currentImageGUID={image.imageGUID}
          onImageSelect={handleFilmstripSelect}
          visible={true}
        />
      </div>
    </div>
  );
};
