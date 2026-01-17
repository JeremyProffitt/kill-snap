import React from 'react';
import './SkeletonLoader.css';

interface ThumbnailSkeletonProps {
  aspectRatio?: string;
}

export const ThumbnailSkeleton: React.FC<ThumbnailSkeletonProps> = ({
  aspectRatio = '4 / 3',
}) => (
  <div className="thumbnail-skeleton">
    <div className="skeleton-image" style={{ aspectRatio }} />
    <div className="skeleton-info">
      <div className="skeleton-line filename" />
      <div className="skeleton-line meta" />
      <div className="skeleton-row">
        <div className="skeleton-colors" />
        <div className="skeleton-actions" />
        <div className="skeleton-rating" />
      </div>
    </div>
  </div>
);

interface GallerySkeletonProps {
  count?: number;
}

export const GallerySkeleton: React.FC<GallerySkeletonProps> = ({ count = 12 }) => {
  // Generate random aspect ratios for visual variety
  const aspectRatios = ['4 / 3', '3 / 2', '16 / 9', '1 / 1', '2 / 3'];
  
  return (
    <div className="gallery-skeleton">
      {Array.from({ length: count }).map((_, i) => (
        <ThumbnailSkeleton
          key={i}
          aspectRatio={aspectRatios[i % aspectRatios.length]}
        />
      ))}
    </div>
  );
};

export const DateSectionSkeleton: React.FC = () => (
  <div className="date-section-skeleton">
    <div className="skeleton-date-header">
      <div className="skeleton-date-title" />
      <div className="skeleton-date-line" />
    </div>
    <GallerySkeleton count={6} />
  </div>
);

interface PageSkeletonProps {
  sections?: number;
}

export const PageSkeleton: React.FC<PageSkeletonProps> = ({ sections = 2 }) => (
  <div className="page-skeleton">
    {Array.from({ length: sections }).map((_, i) => (
      <DateSectionSkeleton key={i} />
    ))}
  </div>
);
