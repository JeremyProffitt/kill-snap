import React, { useState, useEffect } from 'react';
import { api } from '../services/api';
import { authService } from '../services/auth';
import { Image } from '../types';
import { ImageModal } from './ImageModal';
import './ImageGallery.css';

interface ImageGalleryProps {
  onLogout: () => void;
}

const GROUP_COLORS = [
  { number: 1, color: '#e74c3c' },
  { number: 2, color: '#3498db' },
  { number: 3, color: '#2ecc71' },
  { number: 4, color: '#f1c40f' },
  { number: 5, color: '#9b59b6' },
  { number: 6, color: '#e67e22' },
  { number: 7, color: '#e91e63' },
  { number: 8, color: '#795548' },
];

export const ImageGallery: React.FC<ImageGalleryProps> = ({ onLogout }) => {
  const [images, setImages] = useState<Image[]>([]);
  const [selectedImageIndex, setSelectedImageIndex] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [processingId, setProcessingId] = useState<string | null>(null);

  const loadImages = async () => {
    try {
      setLoading(true);
      const data = await api.getImages();
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
  };

  useEffect(() => {
    loadImages();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleImageClick = (index: number) => {
    setSelectedImageIndex(index);
  };

  const handleCloseModal = () => {
    setSelectedImageIndex(null);
  };

  const handleImageUpdate = async () => {
    await loadImages();
    // Keep modal open if there are still images, move to next or close
    if (selectedImageIndex !== null) {
      const newImages = await api.getImages();
      if (newImages.length === 0) {
        setSelectedImageIndex(null);
      } else if (selectedImageIndex >= newImages.length) {
        setSelectedImageIndex(newImages.length - 1);
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

  const handleQuickApprove = async (e: React.MouseEvent, image: Image, groupNumber: number) => {
    e.stopPropagation();
    if (processingId) return;

    setProcessingId(image.imageGUID);
    try {
      const group = GROUP_COLORS.find(g => g.number === groupNumber);
      await api.updateImage(image.imageGUID, {
        groupNumber,
        colorCode: ['red', 'blue', 'green', 'yellow', 'purple', 'orange', 'pink', 'brown'][groupNumber - 1],
        promoted: false,
        reviewed: 'true',
      });
      await loadImages();
    } catch (err) {
      console.error('Failed to approve image:', err);
      alert('Failed to approve image');
    } finally {
      setProcessingId(null);
    }
  };

  const handleQuickReject = async (e: React.MouseEvent, image: Image) => {
    e.stopPropagation();
    if (processingId) return;

    setProcessingId(image.imageGUID);
    try {
      await api.updateImage(image.imageGUID, {
        reviewed: 'true',
      });
      await loadImages();
    } catch (err) {
      console.error('Failed to reject image:', err);
      alert('Failed to reject image');
    } finally {
      setProcessingId(null);
    }
  };

  const handleQuickDelete = async (e: React.MouseEvent, image: Image) => {
    e.stopPropagation();
    if (processingId) return;
    if (!window.confirm('Delete this image?')) return;

    setProcessingId(image.imageGUID);
    try {
      await api.deleteImage(image.imageGUID);
      await loadImages();
    } catch (err) {
      console.error('Failed to delete image:', err);
      alert('Failed to delete image');
    } finally {
      setProcessingId(null);
    }
  };

  const handleLogout = () => {
    authService.logout();
    onLogout();
  };

  if (loading) {
    return <div className="loading">Loading images...</div>;
  }

  if (error) {
    return <div className="error-message">{error}</div>;
  }

  const selectedImage = selectedImageIndex !== null ? images[selectedImageIndex] : null;

  return (
    <div className="gallery-container">
      <header className="gallery-header">
        <h1>Image Review</h1>
        <div className="header-actions">
          <span className="image-count">{images.length} unreviewed images</span>
          <button onClick={handleLogout} className="logout-button">
            Logout
          </button>
        </div>
      </header>

      {images.length === 0 ? (
        <div className="empty-state">
          <p>No unreviewed images found</p>
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
                <div className="quick-actions">
                  <button
                    className="quick-btn approve"
                    onClick={(e) => handleQuickApprove(e, image, 1)}
                    title="Quick Approve (Group 1)"
                  >
                    ✓
                  </button>
                  <button
                    className="quick-btn reject"
                    onClick={(e) => handleQuickReject(e, image)}
                    title="Reject"
                  >
                    ✗
                  </button>
                  <button
                    className="quick-btn delete"
                    onClick={(e) => handleQuickDelete(e, image)}
                    title="Delete"
                  >
                    🗑
                  </button>
                </div>
              </div>
              <div className="image-info">
                <div className="image-dimensions">
                  {image.width}×{image.height}
                </div>
                <div className="group-buttons-mini">
                  {GROUP_COLORS.map((group) => (
                    <button
                      key={group.number}
                      className="group-btn-mini"
                      style={{ backgroundColor: group.color }}
                      onClick={(e) => handleQuickApprove(e, image, group.number)}
                      title={`Approve as Group ${group.number}`}
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
    </div>
  );
};
