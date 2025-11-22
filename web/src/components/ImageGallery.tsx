import React, { useState, useEffect } from 'react';
import { api } from '../services/api';
import { authService } from '../services/auth';
import { Image } from '../types';
import { ImageModal } from './ImageModal';
import './ImageGallery.css';

interface ImageGalleryProps {
  onLogout: () => void;
}

export const ImageGallery: React.FC<ImageGalleryProps> = ({ onLogout }) => {
  const [images, setImages] = useState<Image[]>([]);
  const [selectedImage, setSelectedImage] = useState<Image | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

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
  }, []);

  const handleImageClick = (image: Image) => {
    setSelectedImage(image);
  };

  const handleCloseModal = () => {
    setSelectedImage(null);
  };

  const handleImageUpdate = async () => {
    setSelectedImage(null);
    await loadImages();
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
          {images.map((image) => (
            <div
              key={image.imageGUID}
              className="gallery-item"
              onClick={() => handleImageClick(image)}
            >
              <img
                src={api.getImageUrl(image.bucket, image.thumbnail50)}
                alt={image.originalFile}
                className="thumbnail"
              />
              <div className="image-info">
                <div className="image-dimensions">
                  {image.width}x{image.height}
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
        />
      )}
    </div>
  );
};
