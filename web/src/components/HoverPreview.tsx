import React, { useState, useEffect } from 'react';
import { Image } from '../types';
import { api } from '../services/api';
import './HoverPreview.css';

interface HoverPreviewProps {
  image: Image | null;
  position: { x: number; y: number };
  delay?: number;
}

const formatFileSize = (bytes: number): string => {
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
  }
  return `${(bytes / 1024).toFixed(1)}KB`;
};

const getFilename = (path: string): string => {
  const parts = path.split('/');
  return parts[parts.length - 1];
};

// Get display filename - prioritizes originalFilename, falls back to extracting from originalFile
const getDisplayFilename = (image: Image): string => {
  if (image.originalFilename) {
    return image.originalFilename + '.jpg';
  }
  return getFilename(image.originalFile);
};

export const HoverPreview: React.FC<HoverPreviewProps> = ({
  image,
  position,
  delay = 500,
}) => {
  const [visible, setVisible] = useState(false);
  const [imageLoaded, setImageLoaded] = useState(false);

  useEffect(() => {
    setImageLoaded(false);
    if (image) {
      const timer = setTimeout(() => setVisible(true), delay);
      return () => clearTimeout(timer);
    } else {
      setVisible(false);
    }
  }, [image, delay]);

  if (!image || !visible) return null;

  // Calculate position to keep preview on screen
  const previewWidth = 420;
  const previewHeight = 350;
  const padding = 20;

  let left = position.x + padding;
  let top = position.y - previewHeight / 2;

  // Adjust if overflowing right edge
  if (left + previewWidth > window.innerWidth - padding) {
    left = position.x - previewWidth - padding;
  }

  // Adjust if overflowing top
  if (top < padding) {
    top = padding;
  }

  // Adjust if overflowing bottom
  if (top + previewHeight > window.innerHeight - padding) {
    top = window.innerHeight - previewHeight - padding;
  }

  return (
    <div
      className={`hover-preview ${imageLoaded ? 'loaded' : ''}`}
      style={{ left, top }}
    >
      <div className="hover-preview-image-container">
        <img
          src={api.getImageUrl(image.bucket, image.thumbnail400)}
          alt={getDisplayFilename(image)}
          onLoad={() => setImageLoaded(true)}
        />
        {!imageLoaded && (
          <div className="hover-preview-loading">
            <div className="hover-preview-spinner" />
          </div>
        )}
      </div>
      <div className="hover-preview-info">
        <div className="hover-preview-filename">{getDisplayFilename(image)}</div>
        <div className="hover-preview-meta">
          <span>{image.width} x {image.height}</span>
          <span>{formatFileSize(image.fileSize)}</span>
        </div>
        {image.keywords && image.keywords.length > 0 && (
          <div className="hover-preview-keywords">
            {image.keywords.slice(0, 6).join(', ')}
            {image.keywords.length > 6 && ` +${image.keywords.length - 6} more`}
          </div>
        )}
        {image.rating && image.rating > 0 && (
          <div className="hover-preview-rating">
            {'★'.repeat(image.rating)}{'☆'.repeat(5 - image.rating)}
          </div>
        )}
      </div>
    </div>
  );
};
