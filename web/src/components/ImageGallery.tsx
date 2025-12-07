import React, { useState, useEffect, useCallback } from 'react';
import { api, ImageFilters } from '../services/api';
import { authService } from '../services/auth';
import { Image, Project } from '../types';
import { ImageModal } from './ImageModal';
import { ProjectModal } from './ProjectModal';
import { TransferBanner, TransferProgress } from './TransferBanner';
import { ZipProgressBanner } from './ZipProgressBanner';
import { NotificationBanner, Notification } from './NotificationBanner';
import './ImageGallery.css';

interface ImageGalleryProps {
  onLogout: () => void;
}

// Lightroom color labels: Red, Yellow, Green, Blue, Purple
const GROUP_COLORS = [
  { number: 0, color: '#ffffff', name: 'None' },
  { number: 1, color: '#e74c3c', name: 'Red' },
  { number: 2, color: '#f1c40f', name: 'Yellow' },
  { number: 3, color: '#2ecc71', name: 'Green' },
  { number: 4, color: '#3498db', name: 'Blue' },
  { number: 5, color: '#9b59b6', name: 'Purple' },
];

type StateFilter = 'unreviewed' | 'approved' | 'rejected' | 'deleted' | 'all';

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

export const ImageGallery: React.FC<ImageGalleryProps> = ({ onLogout }) => {
  const [images, setImages] = useState<Image[]>([]);
  const [selectedImageIndex, setSelectedImageIndex] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [processingIds, setProcessingIds] = useState<Set<string>>(new Set());
  const [stateFilter, setStateFilter] = useState<StateFilter>('unreviewed');
  const [groupFilter, setGroupFilter] = useState<number | 'all'>('all');
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProject, setSelectedProject] = useState<string>('');
  const [hoverRating, setHoverRating] = useState<{ imageGUID: string; stars: number } | null>(null);
  const [targetProject, setTargetProject] = useState<string>('');
  const [addingToProject, setAddingToProject] = useState(false);
  const [showProjectModal, setShowProjectModal] = useState(false);
  const [transferProgress, setTransferProgress] = useState<TransferProgress>({
    isActive: false,
    currentFile: '',
    currentIndex: 0,
    totalCount: 0,
    projectName: '',
    status: 'transferring',
  });
  const [notification, setNotification] = useState<Notification | null>(null);

  const loadProjects = useCallback(async () => {
    try {
      const data = await api.getProjects();
      setProjects(data);
    } catch (err: any) {
      console.error('Failed to load projects:', err);
    }
  }, []);

  const loadImages = useCallback(async () => {
    try {
      setLoading(true);
      let data: Image[];

      if (selectedProject) {
        // Load images from selected project
        data = await api.getProjectImages(selectedProject);
      } else {
        // Load images with filters
        const filters: ImageFilters = {
          state: stateFilter,
          group: groupFilter,
        };
        data = await api.getImages(filters);
      }

      setImages(data);
      setError('');
    } catch (err: any) {
      if (err.response?.status === 401) {
        authService.logout();
        onLogout();
      } else {
        setError('Failed to load images');
      }
    } finally {
      setLoading(false);
    }
  }, [stateFilter, groupFilter, selectedProject, onLogout]);

  useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  useEffect(() => {
    loadImages();
  }, [loadImages]);

  // Auto-refresh when images have pending/moving status
  useEffect(() => {
    const hasPendingMoves = images.some(
      img => img.moveStatus === 'pending' || img.moveStatus === 'moving'
    );

    if (hasPendingMoves) {
      const interval = setInterval(() => {
        loadImages();
      }, 2000); // Poll every 2 seconds

      return () => clearInterval(interval);
    }
  }, [images, loadImages]);

  const handleImageClick = (index: number) => {
    setSelectedImageIndex(index);
  };

  const handleCloseModal = () => {
    setSelectedImageIndex(null);
  };

  const handleImageUpdate = async () => {
    await loadImages();
    if (selectedImageIndex !== null) {
      if (images.length <= 1) {
        setSelectedImageIndex(null);
      } else if (selectedImageIndex >= images.length - 1) {
        setSelectedImageIndex(Math.max(0, images.length - 2));
      }
    }
  };

  const handleNavigate = (direction: 'prev' | 'next') => {
    if (selectedImageIndex === null) return;
    if (direction === 'prev' && selectedImageIndex > 0) {
      setSelectedImageIndex(selectedImageIndex - 1);
    } else if (direction === 'next' && selectedImageIndex < images.length - 1) {
      setSelectedImageIndex(selectedImageIndex + 1);
    }
  };

  // Update local images array when properties change (for persistence without API call)
  const handlePropertyChange = useCallback((imageGUID: string, updates: Partial<Image>) => {
    setImages(prevImages =>
      prevImages.map(img =>
        img.imageGUID === imageGUID ? { ...img, ...updates } : img
      )
    );
  }, []);

  const showNotification = useCallback((message: string, type: 'success' | 'error' | 'info') => {
    setNotification({ id: Date.now().toString(), message, type });
  }, []);

  const dismissNotification = useCallback(() => {
    setNotification(null);
  }, []);

  const handleQuickAction = async (
    e: React.MouseEvent,
    image: Image,
    action: 'approve' | 'reject' | 'delete' | 'undelete',
    groupNumber?: number
  ) => {
    e.stopPropagation();
    e.preventDefault();

    const imageId = image.imageGUID;

    // Check if this specific image is already being processed
    if (processingIds.has(imageId)) return;

    // Add this image to processing set
    setProcessingIds(prev => new Set(prev).add(imageId));

    // Determine if this action should remove the item from current view
    const shouldRemoveFromView = (): boolean => {
      if (selectedProject) return false; // Don't remove from project views
      if (stateFilter === 'all') return false; // Don't remove from 'all' view

      switch (action) {
        case 'approve':
          return stateFilter === 'unreviewed' || stateFilter === 'rejected';
        case 'reject':
          return stateFilter === 'unreviewed' || stateFilter === 'approved';
        case 'delete':
          return stateFilter !== 'deleted';
        case 'undelete':
          return stateFilter === 'deleted';
        default:
          return false;
      }
    };

    // Store original image and index for potential rollback
    const originalImage = { ...image };
    const originalIndex = images.findIndex(img => img.imageGUID === imageId);
    const willRemove = shouldRemoveFromView();

    // Optimistic update
    if (willRemove) {
      // Remove from list immediately
      setImages(prev => prev.filter(img => img.imageGUID !== imageId));
    } else {
      // Just update properties
      if (action === 'approve' && groupNumber !== undefined) {
        const colorName = GROUP_COLORS.find(g => g.number === groupNumber)?.name.toLowerCase() || 'white';
        handlePropertyChange(imageId, { groupNumber, colorCode: colorName });
      } else if (action === 'delete') {
        handlePropertyChange(imageId, { status: 'deleted' });
      } else if (action === 'undelete') {
        handlePropertyChange(imageId, { status: 'inbox' });
      }
    }

    try {
      if (action === 'delete') {
        await api.deleteImage(imageId);
      } else if (action === 'undelete') {
        await api.undeleteImage(imageId);
      } else if (action === 'approve' && groupNumber !== undefined) {
        const colorName = GROUP_COLORS.find(g => g.number === groupNumber)?.name.toLowerCase() || 'white';
        await api.updateImage(imageId, {
          groupNumber,
          colorCode: colorName,
          promoted: false,
          reviewed: 'true',
        });
      } else if (action === 'reject') {
        await api.updateImage(imageId, {
          groupNumber: 0,
          colorCode: 'white',
          reviewed: 'true',
        });
      }
      // Success - no refresh needed, optimistic update already applied
    } catch (err: any) {
      console.error(`Failed to ${action} image:`, err);
      const errorMessage = err.response?.data?.error || `Failed to ${action} image`;
      showNotification(errorMessage, 'error');

      // Rollback on error
      if (willRemove && originalIndex !== -1) {
        // Re-insert the image at its original position
        setImages(prev => {
          const newImages = [...prev];
          newImages.splice(originalIndex, 0, originalImage);
          return newImages;
        });
      } else {
        // Restore original properties
        handlePropertyChange(imageId, originalImage);
      }
    } finally {
      // Remove this image from processing set
      setProcessingIds(prev => {
        const next = new Set(prev);
        next.delete(imageId);
        return next;
      });
    }
  };

  const handleLogout = () => {
    authService.logout();
    onLogout();
  };

  // Handle star rating click on thumbnail
  const handleThumbnailRating = async (e: React.MouseEvent, image: Image, stars: number) => {
    e.stopPropagation();
    e.preventDefault();
    if (processingIds.has(image.imageGUID)) return;

    // Toggle off if clicking the same rating
    const newRating = image.rating === stars ? 0 : stars;

    // Optimistic local update
    handlePropertyChange(image.imageGUID, { rating: newRating });

    // Save to backend
    try {
      await api.updateImage(image.imageGUID, { rating: newRating });
    } catch (err) {
      console.error('Failed to update rating:', err);
      // Revert on error
      handlePropertyChange(image.imageGUID, { rating: image.rating });
    }
  };

  const handleProjectCreated = async () => {
    await loadProjects();
    loadImages();
  };

  const handleProjectChange = (projectId: string) => {
    setSelectedProject(projectId);
    if (projectId) {
      // Clear filters when viewing a project
      setStateFilter('all');
      setGroupFilter('all');
    }
  };

  const handleAddToProject = async () => {
    if (!targetProject || addingToProject) return;

    const projectName = projects.find(p => p.projectId === targetProject)?.name || 'project';

    setAddingToProject(true);
    setTransferProgress({
      isActive: true,
      currentFile: '',
      currentIndex: 0,
      totalCount: 0,
      projectName,
      status: 'transferring',
    });

    try {
      const filters = groupFilter !== 'all' ? { group: groupFilter } : { all: true };
      const result = await api.addToProjectWithProgress(
        targetProject,
        filters,
        (currentFile, currentIndex, totalCount) => {
          setTransferProgress(prev => ({
            ...prev,
            currentFile,
            currentIndex,
            totalCount,
          }));
        }
      );

      setTransferProgress(prev => ({
        ...prev,
        currentIndex: result.movedCount,
        totalCount: result.movedCount,
        status: 'complete',
      }));

      loadProjects();
      loadImages();
    } catch (err) {
      console.error('Failed to add to project:', err);
      setTransferProgress(prev => ({
        ...prev,
        status: 'error',
        errorMessage: 'Failed to add images to project',
      }));
    } finally {
      setAddingToProject(false);
    }
  };

  const handleDismissTransfer = () => {
    setTransferProgress(prev => ({
      ...prev,
      isActive: false,
    }));
  };

  const selectedImage = selectedImageIndex !== null ? images[selectedImageIndex] : null;

  return (
    <div className="gallery-container">
      <ZipProgressBanner projects={projects} onComplete={loadProjects} />
      <TransferBanner progress={transferProgress} onDismiss={handleDismissTransfer} />
      <NotificationBanner notification={notification} onDismiss={dismissNotification} />
      <aside className="sidebar">
        <div className="sidebar-top">
          <h1 className="sidebar-title">Kill Snap</h1>

          <div className="image-count-container">
            <span className="image-count-label">
              {selectedProject ? 'Project' : 'Unreviewed'}
            </span>
            <span className="image-count-number">
              {images.length}
            </span>
          </div>

          <div className="sidebar-section">
            <label className="sidebar-label">Project</label>
            <select
              value={selectedProject}
              onChange={(e) => handleProjectChange(e.target.value)}
              className="sidebar-select"
            >
              <option value="">Inbox</option>
              {projects.map((project) => (
                <option key={project.projectId} value={project.projectId}>
                  {project.name} ({project.imageCount})
                </option>
              ))}
            </select>
          </div>

          {!selectedProject && (
            <>
              <div className="sidebar-section">
                <label className="sidebar-label">Status</label>
                <select
                  value={stateFilter}
                  onChange={(e) => setStateFilter(e.target.value as StateFilter)}
                  className="sidebar-select"
                >
                  <option value="unreviewed">Unreviewed</option>
                  <option value="approved">Approved</option>
                  <option value="rejected">Rejected</option>
                  <option value="deleted">Deleted</option>
                  <option value="all">All</option>
                </select>
              </div>

              <div className="sidebar-section">
                <label className="sidebar-label">Group</label>
                <div className="group-boxes-row">
                  <button
                    className={`group-box group-all ${groupFilter === 'all' ? 'active' : ''}`}
                    onClick={() => {
                      setGroupFilter('all');
                      setStateFilter('unreviewed');
                    }}
                    title="All Groups"
                  >
                    All
                  </button>
                  <button
                    className={`group-box group-ungrouped ${groupFilter === 0 ? 'active' : ''}`}
                    onClick={() => {
                      setGroupFilter(0);
                      setStateFilter('unreviewed');
                    }}
                    title="Ungrouped"
                  >
                    Ungrouped
                  </button>
                </div>
                <div className="group-boxes-row">
                  {GROUP_COLORS.slice(1).map((group) => (
                    <button
                      key={group.number}
                      className={`group-box ${groupFilter === group.number ? 'active' : ''}`}
                      style={{ backgroundColor: group.color }}
                      onClick={() => {
                        setGroupFilter(group.number);
                        setStateFilter('approved');
                      }}
                      title={group.name}
                    >
                      {group.number}
                    </button>
                  ))}
                </div>
              </div>

              <div className="sidebar-divider"></div>

              <div className="sidebar-section">
                <label className="sidebar-label">Projects</label>
                <select
                  value={targetProject}
                  onChange={(e) => setTargetProject(e.target.value)}
                  className="sidebar-select"
                >
                  <option value="">Select Project...</option>
                  {projects.map((project) => (
                    <option key={project.projectId} value={project.projectId}>
                      {project.name}
                    </option>
                  ))}
                </select>
                <button
                  onClick={handleAddToProject}
                  disabled={!targetProject || addingToProject}
                  className="sidebar-button primary"
                >
                  {addingToProject ? 'Adding...' : 'Add to Project'}
                </button>
              </div>

              <button
                onClick={() => setShowProjectModal(true)}
                className="sidebar-button secondary"
              >
                Manage
              </button>
            </>
          )}

          {selectedProject && (
            <button
              onClick={() => setShowProjectModal(true)}
              className="sidebar-button secondary"
            >
              Manage
            </button>
          )}
        </div>

        <div className="sidebar-bottom">
          <button onClick={handleLogout} className="logout-button">
            Logout
          </button>
        </div>
      </aside>

      <main className="gallery-main">

      {loading ? (
        <div className="loading">Loading images...</div>
      ) : error ? (
        <div className="error-message">{error}</div>
      ) : images.length === 0 ? (
        <div className="empty-state">
          <p>No {stateFilter === 'all' ? '' : stateFilter} images found</p>
        </div>
      ) : (
        <div className="gallery-grid">
          {images.map((image, index) => {
            const isDeleted = image.status === 'deleted';
            return (
              <div
                key={image.imageGUID}
                className={`gallery-item ${processingIds.has(image.imageGUID) ? 'processing' : ''} ${isDeleted ? 'deleted' : ''}`}
                onClick={() => handleImageClick(index)}
              >
                <div
                  className="thumbnail-container"
                  style={{
                    aspectRatio: image.width && image.height
                      ? `${image.width} / ${image.height}`
                      : '1 / 1'
                  }}
                >
                  <img
                    src={api.getImageUrl(image.bucket, image.thumbnail400)}
                    alt={image.originalFile}
                    className="thumbnail"
                  />
                  {(image.moveStatus === 'pending' || image.moveStatus === 'moving') && (
                    <div className="move-status-indicator">
                      <span className="spinner"></span>
                      <span className="status-text">
                        {image.moveStatus === 'pending' ? 'Queued' : 'Moving...'}
                      </span>
                    </div>
                  )}
                  {image.moveStatus === 'failed' && (
                    <div className="move-status-indicator error">
                      <span className="status-text">Move Failed</span>
                    </div>
                  )}
                </div>
                <div className="image-info">
                  {/* Row 1: Filename left, dimensions + size right */}
                  <div className="info-row-1">
                    <span className="thumb-filename">{getFilename(image.originalFile)}</span>
                    <span className="thumb-size-info">
                      {image.width}×{image.height} - {formatFileSize(image.fileSize)}
                    </span>
                  </div>
                  {/* Row 2: Colors left, actions center, rating right */}
                  <div className="info-row-2">
                    {isDeleted ? (
                      <button
                        type="button"
                        className="undelete-btn"
                        onClick={(e) => handleQuickAction(e, image, 'undelete')}
                        title="Undelete"
                      >
                        ↩ Undelete
                      </button>
                    ) : (
                      <>
                        <div className="thumb-colors">
                          {GROUP_COLORS.slice(1).map((group) => (
                            <button
                              key={group.number}
                              type="button"
                              className={`color-btn ${image.groupNumber === group.number ? 'selected' : ''}`}
                              style={{ backgroundColor: group.color }}
                              onClick={(e) => handleQuickAction(e, image, 'approve', group.number)}
                              title={`${group.name} (${group.number})`}
                            >
                              {group.number}
                            </button>
                          ))}
                        </div>
                        <div className="thumb-actions">
                          <button
                            type="button"
                            className="action-btn-mini approve"
                            onClick={(e) => handleQuickAction(e, image, 'approve', image.groupNumber || 1)}
                            title="Approve"
                          >
                            ✓
                          </button>
                          <button
                            type="button"
                            className="action-btn-mini reject"
                            onClick={(e) => handleQuickAction(e, image, 'reject')}
                            title="Reject"
                          >
                            ✗
                          </button>
                          <button
                            type="button"
                            className="action-btn-mini delete"
                            onClick={(e) => handleQuickAction(e, image, 'delete')}
                            title="Delete"
                          >
                            🗑
                          </button>
                        </div>
                        <div
                          className="thumb-rating"
                          onMouseLeave={() => setHoverRating(null)}
                        >
                          {[1, 2, 3, 4, 5].map((star) => {
                            const currentRating = image.rating ?? 0;
                            const isHovering = hoverRating?.imageGUID === image.imageGUID;
                            const displayRating = isHovering ? hoverRating.stars : currentRating;
                            const isFilled = displayRating >= star;
                            return (
                              <button
                                key={star}
                                type="button"
                                className={`thumb-star-btn ${isFilled ? 'filled' : 'empty'} ${isHovering && star <= hoverRating.stars ? 'hover-preview' : ''}`}
                                onClick={(e) => handleThumbnailRating(e, image, star)}
                                onMouseEnter={() => setHoverRating({ imageGUID: image.imageGUID, stars: star })}
                                title={`${star} star${star > 1 ? 's' : ''}`}
                              >
                                {isFilled ? '★' : '☆'}
                              </button>
                            );
                          })}
                        </div>
                      </>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
      </main>

      {selectedImage && (
        <ImageModal
          image={selectedImage}
          onClose={handleCloseModal}
          onUpdate={handleImageUpdate}
          onNavigate={handleNavigate}
          onPropertyChange={handlePropertyChange}
          onNotify={showNotification}
          hasPrev={selectedImageIndex! > 0}
          hasNext={selectedImageIndex! < images.length - 1}
          currentIndex={selectedImageIndex!}
          totalImages={images.length}
        />
      )}

      {showProjectModal && (
        <ProjectModal
          onClose={() => setShowProjectModal(false)}
          onProjectCreated={handleProjectCreated}
          existingProjects={projects}
        />
      )}
    </div>
  );
};
