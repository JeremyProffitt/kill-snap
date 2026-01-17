import React, { useState, useEffect } from 'react';
import { THEME_COLORS, UI_STYLES, DEFAULT_THEME_COLOR, DEFAULT_UI_STYLE, ThemeColor, UIStyle } from '../theme/themeConstants';
import { api } from '../services/api';
import './ThemeSettings.css';

interface ThemeSettingsProps {
  isOpen: boolean;
  onClose: () => void;
  currentColorId: string;
  currentStyleId: string;
  onThemeChange: (colorId: string, styleId: string) => void;
}

export const ThemeSettings: React.FC<ThemeSettingsProps> = ({
  isOpen,
  onClose,
  currentColorId,
  currentStyleId,
  onThemeChange,
}) => {
  const [selectedColor, setSelectedColor] = useState(currentColorId);
  const [selectedStyle, setSelectedStyle] = useState(currentStyleId);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState<'colors' | 'styles'>('colors');

  useEffect(() => {
    setSelectedColor(currentColorId);
    setSelectedStyle(currentStyleId);
  }, [currentColorId, currentStyleId, isOpen]);

  if (!isOpen) return null;

  const handleColorSelect = (colorId: string) => {
    setSelectedColor(colorId);
  };

  const handleStyleSelect = (styleId: string) => {
    setSelectedStyle(styleId);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      // Save to API
      await api.saveUserSettings({ themeColor: selectedColor, themeStyle: selectedStyle });
      onThemeChange(selectedColor, selectedStyle);
      onClose();
    } catch (error) {
      console.error('Failed to save theme settings:', error);
      // Still apply the change locally
      onThemeChange(selectedColor, selectedStyle);
      onClose();
    } finally {
      setSaving(false);
    }
  };

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  const currentColor = THEME_COLORS.find(c => c.id === selectedColor) || THEME_COLORS[0];
  const currentStyle = UI_STYLES.find(s => s.id === selectedStyle) || UI_STYLES[0];

  return (
    <div className="theme-settings-backdrop" onClick={handleBackdropClick}>
      <div className="theme-settings-modal">
        <div className="theme-settings-header">
          <h2>Appearance Settings</h2>
          <button className="theme-settings-close" onClick={onClose}>X</button>
        </div>

        <div className="theme-settings-tabs">
          <button
            className={`theme-tab ${activeTab === 'colors' ? 'active' : ''}`}
            onClick={() => setActiveTab('colors')}
          >
            Colors ({THEME_COLORS.length})
          </button>
          <button
            className={`theme-tab ${activeTab === 'styles' ? 'active' : ''}`}
            onClick={() => setActiveTab('styles')}
          >
            Styles ({UI_STYLES.length})
          </button>
        </div>

        <div className="theme-settings-content">
          {activeTab === 'colors' && (
            <div className="theme-colors-grid">
              {THEME_COLORS.map((color) => (
                <button
                  key={color.id}
                  className={`theme-color-item ${selectedColor === color.id ? 'selected' : ''}`}
                  onClick={() => handleColorSelect(color.id)}
                  title={color.name}
                >
                  <div className="color-preview">
                    <div className="color-swatch-bg" style={{ backgroundColor: color.background }} />
                    <div className="color-swatch-sidebar" style={{ backgroundColor: color.sidebar }} />
                    <div className="color-swatch-primary" style={{ backgroundColor: color.primary }} />
                  </div>
                  <span className="color-name">{color.name}</span>
                </button>
              ))}
            </div>
          )}

          {activeTab === 'styles' && (
            <div className="theme-styles-grid">
              {UI_STYLES.map((style) => (
                <button
                  key={style.id}
                  className={`theme-style-item ${selectedStyle === style.id ? 'selected' : ''}`}
                  onClick={() => handleStyleSelect(style.id)}
                  title={style.name}
                >
                  <div
                    className="style-preview"
                    style={{
                      borderRadius: style.borderRadius === '0' ? '0' : '6px',
                    }}
                  >
                    <div
                      className="style-button-preview"
                      style={{
                        borderRadius: style.buttonStyle === 'pill' ? '20px' :
                                     style.buttonStyle === 'rounded' ? '6px' : '0',
                      }}
                    />
                    <div className="style-info">
                      <span className="style-spacing">{style.spacing[0].toUpperCase()}</span>
                      <span className="style-shadow">{style.shadowIntensity[0].toUpperCase()}</span>
                    </div>
                  </div>
                  <span className="style-name">{style.name}</span>
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="theme-settings-preview">
          <div className="preview-label">Preview</div>
          <div
            className="preview-container"
            style={{
              backgroundColor: currentColor.background,
              borderRadius: currentStyle.borderRadius,
            }}
          >
            <div
              className="preview-sidebar"
              style={{ backgroundColor: currentColor.sidebar }}
            >
              <div
                className="preview-button"
                style={{
                  backgroundColor: currentColor.primary,
                  borderRadius: currentStyle.buttonStyle === 'pill' ? '12px' :
                               currentStyle.buttonStyle === 'rounded' ? '4px' : '0',
                }}
              />
            </div>
            <div className="preview-content">
              <div className="preview-cards">
                <div
                  className="preview-card"
                  style={{
                    borderRadius: currentStyle.borderRadius,
                    backgroundColor: currentColor.sidebar,
                  }}
                />
                <div
                  className="preview-card"
                  style={{
                    borderRadius: currentStyle.borderRadius,
                    backgroundColor: currentColor.sidebar,
                  }}
                />
              </div>
            </div>
          </div>
        </div>

        <div className="theme-settings-footer">
          <button className="theme-cancel-btn" onClick={onClose}>
            Cancel
          </button>
          <button
            className="theme-save-btn"
            onClick={handleSave}
            disabled={saving}
            style={{ backgroundColor: currentColor.primary }}
          >
            {saving ? 'Saving...' : 'Apply Theme'}
          </button>
        </div>
      </div>
    </div>
  );
};
