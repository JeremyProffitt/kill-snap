import React, { useState } from 'react';
import { api } from '../services/api';
import { Image } from '../types';
import './ImageModal.css';

interface ImageModalProps {
  image: Image;
  onClose: () => void;
  onUpdate: () => void;
}

const COLOR_CODES = [
  { value: 'red', label: 'Red' },
  { value: 'blue', label: 'Blue' },
  { value: 'green', label: 'Green' },
  { value: 'yellow', label: 'Yellow' },
  { value: 'purple', label: 'Purple' },
  { value: 'orange', label: 'Orange' },
  { value: 'pink', label: 'Pink' },
  { value: 'brown', label: 'Brown' },
];

export const ImageModal: React.FC<ImageModalProps> = ({ image, onClose, onUpdate }) => {
  const [groupNumber, setGroupNumber] = useState<number>(1);
  const [colorCode, setColorCode] = useState<string>('red');
  const [promoted, setPromoted] = useState<boolean>(false);
  const [loading, setLoading] = useState(false);

  const handleApprove = async () => {
    setLoading(true);
    try {
      await api.updateImage(image.imageGUID, {
        groupNumber,
        colorCode,
        promoted,
        reviewed: 'true',
      });
      onUpdate();
    } catch (err) {
      console.error('Failed to approve image:', err);
      alert('Failed to approve image');
    } finally {
      setLoading(false);
    }
  };

  const handleReject = async () => {
    setLoading(true);
    try {
      await api.updateImage(image.imageGUID, {
        reviewed: 'true',
      });
      onUpdate();
    } catch (err) {
      console.error('Failed to reject image:', err);
      alert('Failed to reject image');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
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
  };

  const handleDownload = async () => {
    try {
      const url = await api.getDownloadUrl(image.imageGUID);
      window.open(url, '_blank');
    } catch (err) {
      console.error('Failed to get download URL:', err);
      alert('Failed to download image');
    }
  };

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
          <div className="image-preview">
            <img
              src={api.getImageUrl(image.bucket, image.thumbnail400)}
              alt={image.originalFile}
            />
          </div>

          <div className="image-details">
            <h2>Review Image</h2>

            <div className="detail-group">
              <label>Dimensions:</label>
              <span>{image.width} × {image.height}px</span>
            </div>

            <div className="detail-group">
              <label>File Size:</label>
              <span>{(image.fileSize / 1024).toFixed(1)} KB</span>
            </div>

            <div className="form-section">
              <h3>Review Options</h3>

              <div className="form-field">
                <label htmlFor="groupNumber">Group Number (1-8):</label>
                <input
                  id="groupNumber"
                  type="number"
                  min="1"
                  max="8"
                  value={groupNumber}
                  onChange={(e) => setGroupNumber(parseInt(e.target.value))}
                  disabled={loading}
                />
              </div>

              <div className="form-field">
                <label htmlFor="colorCode">Color Code:</label>
                <select
                  id="colorCode"
                  value={colorCode}
                  onChange={(e) => setColorCode(e.target.value)}
                  disabled={loading}
                >
                  {COLOR_CODES.map((color) => (
                    <option key={color.value} value={color.value}>
                      {color.label}
                    </option>
                  ))}
                </select>
              </div>

              <div className="form-field checkbox-field">
                <label>
                  <input
                    type="checkbox"
                    checked={promoted}
                    onChange={(e) => setPromoted(e.target.checked)}
                    disabled={loading}
                  />
                  <span>Promote</span>
                </label>
              </div>
            </div>

            <div className="action-buttons">
              <button
                onClick={handleApprove}
                disabled={loading}
                className="btn-approve"
              >
                Approve
              </button>
              <button
                onClick={handleReject}
                disabled={loading}
                className="btn-reject"
              >
                Reject
              </button>
              <button
                onClick={handleDownload}
                disabled={loading}
                className="btn-download"
              >
                Download
              </button>
              <button
                onClick={handleDelete}
                disabled={loading}
                className="btn-delete"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
