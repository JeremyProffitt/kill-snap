import React, { useRef, useEffect } from 'react';
import { Image } from '../types';
import { api } from '../services/api';
import './Filmstrip.css';

interface FilmstripProps {
  images: Image[];
  currentImageGUID: string;
  onImageSelect: (imageGUID: string) => void;
  visible: boolean;
}

export const Filmstrip: React.FC<FilmstripProps> = ({
  images,
  currentImageGUID,
  onImageSelect,
  visible,
}) => {
  const trackRef = useRef<HTMLDivElement>(null);
  const currentIndex = images.findIndex(img => img.imageGUID === currentImageGUID);

  // Scroll to keep current image visible
  useEffect(() => {
    if (!visible || !trackRef.current || currentIndex === -1) return;

    const track = trackRef.current;
    const thumbs = track.querySelectorAll('.filmstrip-thumb');
    const currentThumb = thumbs[currentIndex] as HTMLElement;

    if (currentThumb) {
      const trackRect = track.getBoundingClientRect();
      const thumbRect = currentThumb.getBoundingClientRect();
      
      // Check if thumb is outside visible area
      if (thumbRect.left < trackRect.left || thumbRect.right > trackRect.right) {
        currentThumb.scrollIntoView({
          behavior: 'smooth',
          block: 'nearest',
          inline: 'center',
        });
      }
    }
  }, [currentIndex, visible]);

  if (!visible) return null;

  return (
    <div className="filmstrip">
      <div className="filmstrip-track" ref={trackRef}>
        {images.map((img, index) => (
          <button
            key={img.imageGUID}
            className={`filmstrip-thumb ${img.imageGUID === currentImageGUID ? 'active' : ''}`}
            onClick={() => onImageSelect(img.imageGUID)}
            title={`Image ${index + 1}`}
          >
            <img
              src={api.getImageUrl(img.bucket, img.thumbnail400)}
              alt=""
              loading="lazy"
            />
            {img.groupNumber && img.groupNumber > 0 && (
              <div
                className="filmstrip-color-indicator"
                style={{ backgroundColor: getColorForGroup(img.groupNumber) }}
              />
            )}
          </button>
        ))}
      </div>
      <div className="filmstrip-position">
        {currentIndex + 1} / {images.length}
      </div>
    </div>
  );
};

const getColorForGroup = (groupNumber: number): string => {
  const colors: Record<number, string> = {
    1: '#e74c3c',
    2: '#f1c40f',
    3: '#2ecc71',
    4: '#3498db',
    5: '#9b59b6',
  };
  return colors[groupNumber] || 'transparent';
};
