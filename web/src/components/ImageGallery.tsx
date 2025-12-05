import React, { useState, useEffect, useCallback } from 'react';
import { api, ImageFilters } from '../services/api';
import { authService } from '../services/auth';
import { Image, Project } from '../types';
import { ImageModal } from './ImageModal';
import { ProjectModal } from './ProjectModal';
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

type StateFilter = 'unreviewed' | 'approved' | 'rejected' | 'all';

// Helper to render star rating
const renderStars = (rating: number, maxStars: number = 5) => {
  return Array.from({ length: maxStars }, (_, i) => (
    <span key={i} className={`star ${i < rating ? 'filled' : 'empty'}`}>
      {i < rating ? '★' : '☆'}
    </span>
  ));
};

export const ImageGallery: React.FC<ImageGalleryProps> = ({ onLogout }) => {
  const [images, setImages] = useState<Image[]>([]);
  const [selectedImageIndex, setSelectedImageIndex] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [processingId, setProcessingId] = useState<string | null>(null);
  const [stateFilter, setStateFilter] = useState<StateFilter>('unreviewed');
  const [groupFilter, setGroupFilter] = useState<number | 'all'>('all');
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProject, setSelectedProject] = useState<string>('');
  const [showProjectModal, setShowProjectModal] = useState(false);

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

  const handleQuickAction = async (
    e: React.MouseEvent,
    image: Image,
    action: 'approve' | 'reject' | 'delete',
    groupNumber?: number
  ) => {
    e.stopPropagation();
    e.preventDefault();
    if (processingId) return;

    if (action === 'delete') {
      if (!window.confirm('Delete this image?')) return;
    }

    setProcessingId(image.imageGUID);
    try {
      if (action === 'delete') {
        await api.deleteImage(image.imageGUID);
      } else if (action === 'approve' && groupNumber !== undefined) {
        const colorName = GROUP_COLORS.find(g => g.number === groupNumber)?.name.toLowerCase() || 'white';
        await api.updateImage(image.imageGUID, {
          groupNumber,
          colorCode: colorName,
          promoted: false,
          reviewed: 'true',
        });
      } else if (action === 'reject') {
        await api.updateImage(image.imageGUID, {
          groupNumber: 0,
          colorCode: 'white',
          reviewed: 'true',
        });
      }
      await loadImages();
    } catch (err) {
      console.error(`Failed to ${action} image:`, err);
      alert(`Failed to ${action} image`);
    } finally {
      setProcessingId(null);
    }
  };

  const handleLogout = () => {
    authService.logout();
    onLogout();
  };

  const handleProjectCreated = () => {
    loadProjects();
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

  const getStateLabel = () => {
    switch (stateFilter) {
      case 'unreviewed': return 'unreviewed';
      case 'approved': return 'approved';
      case 'rejected': return 'rejected';
      case 'all': return 'total';
    }
  };

  const selectedImage = selectedImageIndex !== null ? images[selectedImageIndex] : null;

  return (
    <div className="gallery-container">
      <header className="gallery-header">
        <h1>Image Review</h1>
        <div className="header-controls">
          <div className="filter-group">
            <label>View:</label>
            <select
              value={selectedProject}
              onChange={(e) => handleProjectChange(e.target.value)}
              className="filter-select project-select"
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
              <div className="filter-group">
                <label>Status:</label>
                <select
                  value={stateFilter}
                  onChange={(e) => setStateFilter(e.target.value as StateFilter)}
                  className="filter-select"
                >
                  <option value="unreviewed">Unreviewed</option>
                  <option value="approved">Approved</option>
                  <option value="rejected">Rejected</option>
                  <option value="all">All</option>
                </select>
              </div>
              <div className="filter-group">
                <label>Group:</label>
                <select
                  value={groupFilter}
                  onChange={(e) => setGroupFilter(e.target.value === 'all' ? 'all' : parseInt(e.target.value))}
                  className="filter-select"
                >
                  <option value="all">All Groups</option>
                  {GROUP_COLORS.map((group) => (
                    <option key={group.number} value={group.number}>
                      {group.number}: {group.name}
                    </option>
                  ))}
                </select>
              </div>
            </>
          )}
          <button
            onClick={() => setShowProjectModal(true)}
            className="projects-button"
          >
            Projects
          </button>
        </div>
        <div className="header-actions">
          <span className="image-count">
            {images.length} {selectedProject ? 'project' : getStateLabel()} images
          </span>
          <button onClick={handleLogout} className="logout-button">
            Logout
          </button>
        </div>
      </header>

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
          {images.map((image, index) => (
            <div
              key={image.imageGUID}
              className={`gallery-item ${processingId === image.imageGUID ? 'processing' : ''}`}
              onClick={() => handleImageClick(index)}
            >
              <div className="thumbnail-container">
                <img
                  src={api.getImageUrl(image.bucket, image.thumbnail400)}
                  alt={image.originalFile}
                  className="thumbnail"
                />
                <div className="quick-actions-overlay">
                  <button
                    type="button"
                    className="quick-btn approve"
                    onClick={(e) => handleQuickAction(e, image, 'approve', 1)}
                    title="Quick Approve (Group 1)"
                  >
                    ✓
                  </button>
                  <button
                    type="button"
                    className="quick-btn reject"
                    onClick={(e) => handleQuickAction(e, image, 'reject')}
                    title="Reject"
                  >
                    ✗
                  </button>
                  <button
                    type="button"
                    className="quick-btn delete"
                    onClick={(e) => handleQuickAction(e, image, 'delete')}
                    title="Delete"
                  >
                    🗑
                  </button>
                </div>
              </div>
              <div className="image-info">
                <div className="image-meta-row">
                  <div className="image-dimensions">
                    {image.width}×{image.height}
                  </div>
                  {image.rating ? (
                    <div className="image-rating-display">
                      {renderStars(image.rating)}
                    </div>
                  ) : null}
                </div>
                <div className="group-buttons-mini">
                  {GROUP_COLORS.slice(1).map((group) => (
                    <button
                      key={group.number}
                      type="button"
                      className="group-btn-mini"
                      style={{ backgroundColor: group.color }}
                      onClick={(e) => handleQuickAction(e, image, 'approve', group.number)}
                      title={`Approve as Group ${group.number} (${group.name})`}
                    >
                      {group.number}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {selectedImage && (
        <ImageModal
          image={selectedImage}
          onClose={handleCloseModal}
          onUpdate={handleImageUpdate}
          onNavigate={handleNavigate}
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
