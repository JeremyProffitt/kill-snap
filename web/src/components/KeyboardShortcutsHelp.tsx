import React, { useEffect, useCallback } from 'react';
import './KeyboardShortcutsHelp.css';

interface KeyboardShortcutsHelpProps {
  isOpen: boolean;
  onClose: () => void;
  context?: 'gallery' | 'modal';
}

interface ShortcutItem {
  keys: string[];
  description: string;
}

interface ShortcutCategory {
  category: string;
  items: ShortcutItem[];
}

const galleryShortcuts: ShortcutCategory[] = [
  {
    category: 'Navigation',
    items: [
      { keys: ['Arrow Keys'], description: 'Move focus between images' },
      { keys: ['H', 'J', 'K', 'L'], description: 'Vim-style navigation' },
      { keys: ['Home'], description: 'Go to first image' },
      { keys: ['End'], description: 'Go to last image' },
      { keys: ['Enter'], description: 'Open focused image in modal' },
    ],
  },
  {
    category: 'Selection',
    items: [
      { keys: ['Space'], description: 'Toggle selection on focused image' },
      { keys: ['Shift + Click'], description: 'Range select images' },
      { keys: ['Ctrl + Click'], description: 'Add/remove from selection' },
      { keys: ['Ctrl + A'], description: 'Select all visible images' },
      { keys: ['Esc'], description: 'Clear selection' },
    ],
  },
  {
    category: 'Actions',
    items: [
      { keys: ['1', '2', '3', '4', '5'], description: 'Assign color to focused image' },
      { keys: ['Ctrl + Z'], description: 'Undo last action' },
      { keys: ['/'], description: 'Focus search input' },
      { keys: ['?'], description: 'Show this help' },
    ],
  },
];

const modalShortcuts: ShortcutCategory[] = [
  {
    category: 'Navigation',
    items: [
      { keys: ['Left Arrow'], description: 'Previous image' },
      { keys: ['Right Arrow'], description: 'Next image' },
      { keys: ['Esc'], description: 'Close modal' },
    ],
  },
  {
    category: 'Actions',
    items: [
      { keys: ['1', '2', '3', '4', '5'], description: 'Assign color group' },
      { keys: ['Enter'], description: 'Approve image' },
      { keys: ['R'], description: 'Reject image' },
      { keys: ['D', 'Delete'], description: 'Delete image' },
      { keys: ['U'], description: 'Undelete image (when deleted)' },
    ],
  },
  {
    category: 'Zoom (when enabled)',
    items: [
      { keys: ['Scroll'], description: 'Zoom in/out' },
      { keys: ['Click + Drag'], description: 'Pan when zoomed' },
      { keys: ['Double Click'], description: 'Reset zoom' },
    ],
  },
];

export const KeyboardShortcutsHelp: React.FC<KeyboardShortcutsHelpProps> = ({
  isOpen,
  onClose,
  context = 'gallery',
}) => {
  const shortcuts = context === 'modal' ? modalShortcuts : galleryShortcuts;

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape' || e.key === '?') {
      e.preventDefault();
      onClose();
    }
  }, [onClose]);

  useEffect(() => {
    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
      return () => window.removeEventListener('keydown', handleKeyDown);
    }
  }, [isOpen, handleKeyDown]);

  if (!isOpen) return null;

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  return (
    <div className="shortcuts-overlay" onClick={handleBackdropClick}>
      <div className="shortcuts-dialog">
        <div className="shortcuts-header">
          <h2>Keyboard Shortcuts</h2>
          <button className="shortcuts-close-btn" onClick={onClose}>
            Close
          </button>
        </div>
        
        <div className="shortcuts-content">
          {shortcuts.map((category) => (
            <div key={category.category} className="shortcut-category">
              <h3>{category.category}</h3>
              <div className="shortcut-list">
                {category.items.map((item, index) => (
                  <div key={index} className="shortcut-item">
                    <div className="shortcut-keys">
                      {item.keys.map((key, keyIndex) => (
                        <React.Fragment key={keyIndex}>
                          <kbd>{key}</kbd>
                          {keyIndex < item.keys.length - 1 && (
                            <span className="shortcut-separator">or</span>
                          )}
                        </React.Fragment>
                      ))}
                    </div>
                    <div className="shortcut-description">{item.description}</div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>

        <div className="shortcuts-footer">
          <span>Press <kbd>?</kbd> to toggle this help</span>
        </div>
      </div>
    </div>
  );
};
