# Kill-Snap UI Usability Enhancements - Implementation Plan

This document outlines the comprehensive plan to implement all usability enhancements for the Kill-Snap application.

---

## Overview

| Phase | Feature | Priority | Estimated Effort | Dependencies |
|-------|---------|----------|------------------|--------------|
| 1 | Multi-Select & Bulk Operations | High | 3-4 days | None |
| 2 | Persistent User Preferences | High | 1-2 days | None |
| 3 | Keyboard Navigation in Gallery | High | 2-3 days | Phase 1 |
| 4 | Search & Filter | Medium | 2-3 days | None |
| 5 | Image Count Badges | Medium | 0.5 day | None |
| 6 | Undo Actions | Medium | 2-3 days | None |
| 7 | Collapsible Sidebar | Medium | 1-2 days | Phase 2 |
| 8 | Thumbnail Hover Preview | Low | 1-2 days | None |
| 9 | Modal Zoom & Pan | Low | 2-3 days | None |
| 10 | Optional Filmstrip Navigation | Low | 1-2 days | None |
| 11 | Progress Estimation | Low | 1 day | None |
| 12 | Confirmation Dialogs | Medium | 1 day | Phase 2 |
| 13 | Drag & Drop | Low | 3-4 days | Phase 1 |
| 14 | Loading Skeleton States | Low | 1 day | None |
| 15 | Keyboard Shortcut Help | Low | 1 day | Phase 3 |
| 16 | Better Empty States | Low | 0.5 day | None |
| 17 | Touch/Mobile Gestures | Low | 2-3 days | Phase 9 |
| 18 | Session Persistence | Low | 1-2 days | Phase 2 |

**Total Estimated Effort: 25-35 days**

---

## Phase 1: Multi-Select & Bulk Operations

### Goal
Allow users to select multiple images and perform bulk actions (approve, reject, delete, add to project).

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageGallery.css`
- `web/src/types.ts`
- `web/src/services/api.ts`

### Implementation Steps

#### 1.1 Add Selection State
```typescript
// In ImageGallery.tsx
const [selectedImages, setSelectedImages] = useState<Set<string>>(new Set());
const [lastSelectedIndex, setLastSelectedIndex] = useState<number | null>(null);
const [isSelectionMode, setIsSelectionMode] = useState(false);
```

#### 1.2 Create Selection Handlers
```typescript
// Toggle single image selection
const toggleImageSelection = (imageGUID: string, index: number, event: React.MouseEvent) => {
  const newSelection = new Set(selectedImages);
  
  if (event.shiftKey && lastSelectedIndex !== null) {
    // Range selection
    const flatImages = getFlatImageList();
    const start = Math.min(lastSelectedIndex, index);
    const end = Math.max(lastSelectedIndex, index);
    for (let i = start; i <= end; i++) {
      newSelection.add(flatImages[i].imageGUID);
    }
  } else if (event.ctrlKey || event.metaKey) {
    // Toggle individual
    if (newSelection.has(imageGUID)) {
      newSelection.delete(imageGUID);
    } else {
      newSelection.add(imageGUID);
    }
  } else {
    // Single select (clear others)
    newSelection.clear();
    newSelection.add(imageGUID);
  }
  
  setSelectedImages(newSelection);
  setLastSelectedIndex(index);
  setIsSelectionMode(newSelection.size > 0);
};

// Select all in date group
const selectAllInDateGroup = (date: string) => {
  const newSelection = new Set(selectedImages);
  groupedImages[date].forEach(img => newSelection.add(img.imageGUID));
  setSelectedImages(newSelection);
  setIsSelectionMode(true);
};

// Clear selection
const clearSelection = () => {
  setSelectedImages(new Set());
  setIsSelectionMode(false);
  setLastSelectedIndex(null);
};
```

#### 1.3 Add Checkbox Overlay to Thumbnails
```tsx
// In thumbnail rendering
<div className={`thumbnail-card ${selectedImages.has(image.imageGUID) ? 'selected' : ''}`}>
  <div 
    className="selection-checkbox"
    onClick={(e) => { e.stopPropagation(); toggleImageSelection(image.imageGUID, index, e); }}
  >
    {selectedImages.has(image.imageGUID) ? '✓' : ''}
  </div>
  {/* existing thumbnail content */}
</div>
```

#### 1.4 Create Floating Bulk Action Bar Component
```tsx
// New component: BulkActionBar.tsx
interface BulkActionBarProps {
  selectedCount: number;
  onApprove: () => void;
  onReject: () => void;
  onDelete: () => void;
  onAddToProject: () => void;
  onClearSelection: () => void;
  onSelectAll: () => void;
}

const BulkActionBar: React.FC<BulkActionBarProps> = ({...}) => {
  if (selectedCount === 0) return null;
  
  return (
    <div className="bulk-action-bar">
      <span className="selection-count">{selectedCount} selected</span>
      <button onClick={onSelectAll}>Select All</button>
      <button onClick={onApprove} className="approve-btn">Approve</button>
      <button onClick={onReject} className="reject-btn">Reject</button>
      <button onClick={onDelete} className="delete-btn">Delete</button>
      <button onClick={onAddToProject}>Add to Project</button>
      <button onClick={onClearSelection} className="clear-btn">Clear</button>
    </div>
  );
};
```

#### 1.5 Add Bulk API Endpoints (if not existing)
```typescript
// In api.ts
export const bulkUpdateImages = async (
  imageGUIDs: string[], 
  update: UpdateImageRequest
): Promise<void> => {
  await Promise.all(imageGUIDs.map(guid => updateImage(guid, update)));
};
```

#### 1.6 CSS Styles
```css
/* Selection checkbox */
.selection-checkbox {
  position: absolute;
  top: 8px;
  left: 8px;
  width: 24px;
  height: 24px;
  border: 2px solid white;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.5);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.2s;
  z-index: 10;
}

.thumbnail-card:hover .selection-checkbox,
.thumbnail-card.selected .selection-checkbox {
  opacity: 1;
}

.thumbnail-card.selected .selection-checkbox {
  background: #3498db;
  border-color: #3498db;
}

/* Bulk action bar */
.bulk-action-bar {
  position: fixed;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: #1a1a2e;
  border: 1px solid #2a3a5a;
  border-radius: 8px;
  padding: 12px 20px;
  display: flex;
  align-items: center;
  gap: 12px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
  z-index: 1000;
  animation: slideUp 0.3s ease;
}

@keyframes slideUp {
  from { transform: translateX(-50%) translateY(100px); opacity: 0; }
  to { transform: translateX(-50%) translateY(0); opacity: 1; }
}
```

---

## Phase 2: Persistent User Preferences

### Goal
Save and restore user preferences using localStorage.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageModal.tsx`
- New file: `web/src/services/preferences.ts`

### Implementation Steps

#### 2.1 Create Preferences Service
```typescript
// web/src/services/preferences.ts
export interface UserPreferences {
  thumbnailSize: number;
  sidebarCollapsed: boolean;
  defaultStatusFilter: string;
  defaultColorFilter: number | null;
  sortOrder: 'asc' | 'desc';
  showFilmstrip: boolean;
  confirmOnDelete: boolean;
  showKeyboardShortcuts: boolean;
}

const PREFERENCES_KEY = 'kill-snap-preferences';

const defaultPreferences: UserPreferences = {
  thumbnailSize: 200,
  sidebarCollapsed: false,
  defaultStatusFilter: 'unreviewed',
  defaultColorFilter: null,
  sortOrder: 'desc',
  showFilmstrip: false,
  confirmOnDelete: true,
  showKeyboardShortcuts: true,
};

export const getPreferences = (): UserPreferences => {
  try {
    const stored = localStorage.getItem(PREFERENCES_KEY);
    if (stored) {
      return { ...defaultPreferences, ...JSON.parse(stored) };
    }
  } catch (e) {
    console.error('Failed to load preferences:', e);
  }
  return defaultPreferences;
};

export const savePreferences = (prefs: Partial<UserPreferences>): void => {
  try {
    const current = getPreferences();
    const updated = { ...current, ...prefs };
    localStorage.setItem(PREFERENCES_KEY, JSON.stringify(updated));
  } catch (e) {
    console.error('Failed to save preferences:', e);
  }
};

export const savePreference = <K extends keyof UserPreferences>(
  key: K, 
  value: UserPreferences[K]
): void => {
  savePreferences({ [key]: value });
};
```

#### 2.2 Integrate with ImageGallery
```typescript
// In ImageGallery.tsx
import { getPreferences, savePreference, UserPreferences } from '../services/preferences';

// Initialize state from preferences
const [preferences, setPreferences] = useState<UserPreferences>(getPreferences);
const [thumbnailSize, setThumbnailSize] = useState(preferences.thumbnailSize);
const [statusFilter, setStatusFilter] = useState(preferences.defaultStatusFilter);

// Save on change
const handleThumbnailSizeChange = (size: number) => {
  setThumbnailSize(size);
  savePreference('thumbnailSize', size);
};
```

---

## Phase 3: Keyboard Navigation in Gallery

### Goal
Enable full keyboard navigation without opening the modal.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageGallery.css`

### Implementation Steps

#### 3.1 Add Focus Management State
```typescript
const [focusedImageIndex, setFocusedImageIndex] = useState<number | null>(null);
const galleryRef = useRef<HTMLDivElement>(null);
```

#### 3.2 Keyboard Event Handler
```typescript
useEffect(() => {
  const handleKeyDown = (e: KeyboardEvent) => {
    // Don't handle if typing in an input
    if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
      return;
    }
    
    const flatImages = getFlatImageList();
    
    switch (e.key) {
      case 'ArrowRight':
      case 'l': // vim-style
        e.preventDefault();
        setFocusedImageIndex(prev => 
          prev === null ? 0 : Math.min(prev + 1, flatImages.length - 1)
        );
        break;
        
      case 'ArrowLeft':
      case 'h': // vim-style
        e.preventDefault();
        setFocusedImageIndex(prev => 
          prev === null ? 0 : Math.max(prev - 1, 0)
        );
        break;
        
      case 'ArrowDown':
      case 'j': // vim-style
        e.preventDefault();
        // Move down one row (estimate based on column count)
        setFocusedImageIndex(prev => 
          prev === null ? 0 : Math.min(prev + columnCount, flatImages.length - 1)
        );
        break;
        
      case 'ArrowUp':
      case 'k': // vim-style
        e.preventDefault();
        setFocusedImageIndex(prev => 
          prev === null ? 0 : Math.max(prev - columnCount, 0)
        );
        break;
        
      case 'Enter':
        if (focusedImageIndex !== null) {
          e.preventDefault();
          openModal(flatImages[focusedImageIndex]);
        }
        break;
        
      case ' ': // Space to toggle selection
        if (focusedImageIndex !== null) {
          e.preventDefault();
          toggleImageSelection(flatImages[focusedImageIndex].imageGUID, focusedImageIndex, e as any);
        }
        break;
        
      case '1': case '2': case '3': case '4': case '5':
        if (focusedImageIndex !== null) {
          e.preventDefault();
          const group = parseInt(e.key);
          handleColorChange(flatImages[focusedImageIndex], group);
        }
        break;
        
      case '/':
        e.preventDefault();
        document.getElementById('search-input')?.focus();
        break;
        
      case 'Escape':
        clearSelection();
        setFocusedImageIndex(null);
        break;
        
      case 'Home':
        e.preventDefault();
        setFocusedImageIndex(0);
        break;
        
      case 'End':
        e.preventDefault();
        setFocusedImageIndex(flatImages.length - 1);
        break;
    }
  };
  
  window.addEventListener('keydown', handleKeyDown);
  return () => window.removeEventListener('keydown', handleKeyDown);
}, [focusedImageIndex, selectedImages, columnCount]);
```

#### 3.3 Visual Focus Indicator
```css
.thumbnail-card.focused {
  outline: 3px solid #3498db;
  outline-offset: 2px;
}
```

#### 3.4 Auto-scroll to Focused Image
```typescript
useEffect(() => {
  if (focusedImageIndex !== null) {
    const element = document.querySelector(`[data-image-index="${focusedImageIndex}"]`);
    element?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }
}, [focusedImageIndex]);
```

---

## Phase 4: Search & Filter

### Goal
Add text search to filter images by filename and keywords.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageGallery.css`

### Implementation Steps

#### 4.1 Add Search State
```typescript
const [searchQuery, setSearchQuery] = useState('');
const [debouncedSearch, setDebouncedSearch] = useState('');

// Debounce search input
useEffect(() => {
  const timer = setTimeout(() => setDebouncedSearch(searchQuery), 300);
  return () => clearTimeout(timer);
}, [searchQuery]);
```

#### 4.2 Fuzzy Match Function
```typescript
const fuzzyMatch = (text: string, query: string): boolean => {
  const lowerText = text.toLowerCase();
  const lowerQuery = query.toLowerCase();
  
  // Exact substring match
  if (lowerText.includes(lowerQuery)) return true;
  
  // Fuzzy match (all characters in order)
  let queryIndex = 0;
  for (let i = 0; i < lowerText.length && queryIndex < lowerQuery.length; i++) {
    if (lowerText[i] === lowerQuery[queryIndex]) {
      queryIndex++;
    }
  }
  return queryIndex === lowerQuery.length;
};
```

#### 4.3 Filter Images
```typescript
const filterImages = (images: Image[]): Image[] => {
  if (!debouncedSearch) return images;
  
  return images.filter(img => {
    // Match filename
    if (fuzzyMatch(img.originalFile, debouncedSearch)) return true;
    
    // Match keywords
    if (img.keywords?.some(kw => fuzzyMatch(kw, debouncedSearch))) return true;
    
    // Match description
    if (img.description && fuzzyMatch(img.description, debouncedSearch)) return true;
    
    return false;
  });
};
```

#### 4.4 Search Input UI
```tsx
<div className="search-container">
  <input
    id="search-input"
    type="text"
    placeholder="Search files, keywords... (Press /)"
    value={searchQuery}
    onChange={(e) => setSearchQuery(e.target.value)}
    className="search-input"
  />
  {searchQuery && (
    <button 
      className="search-clear" 
      onClick={() => setSearchQuery('')}
    >
      ×
    </button>
  )}
</div>
```

#### 4.5 Search CSS
```css
.search-container {
  position: relative;
  margin-bottom: 16px;
}

.search-input {
  width: 100%;
  padding: 10px 36px 10px 12px;
  background: #16213e;
  border: 1px solid #2a3a5a;
  border-radius: 6px;
  color: white;
  font-size: 14px;
}

.search-input:focus {
  outline: none;
  border-color: #3498db;
}

.search-clear {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  color: #8892a6;
  font-size: 20px;
  cursor: pointer;
}
```

---

## Phase 5: Image Count Badges

### Goal
Display image counts on filter buttons.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageGallery.css`

### Implementation Steps

#### 5.1 Compute Counts
```typescript
const imageCounts = useMemo(() => {
  const counts = {
    all: images.length,
    unreviewed: 0,
    approved: 0,
    rejected: 0,
    deleted: 0,
    colors: { 1: 0, 2: 0, 3: 0, 4: 0, 5: 0 } as Record<number, number>,
  };
  
  images.forEach(img => {
    if (img.reviewed === 'no') counts.unreviewed++;
    else if (img.status === 'approved') counts.approved++;
    else if (img.status === 'rejected') counts.rejected++;
    else if (img.status === 'deleted') counts.deleted++;
    
    if (img.groupNumber) {
      counts.colors[img.groupNumber] = (counts.colors[img.groupNumber] || 0) + 1;
    }
  });
  
  return counts;
}, [images]);
```

#### 5.2 Update Filter Buttons
```tsx
<button 
  className={`filter-btn ${statusFilter === 'unreviewed' ? 'active' : ''}`}
  onClick={() => setStatusFilter('unreviewed')}
>
  Unreviewed
  <span className="count-badge">{imageCounts.unreviewed}</span>
</button>
```

#### 5.3 Badge CSS
```css
.count-badge {
  display: inline-block;
  background: rgba(255, 255, 255, 0.1);
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 11px;
  margin-left: 6px;
}

.filter-btn.active .count-badge {
  background: rgba(255, 255, 255, 0.2);
}
```

---

## Phase 6: Undo Actions

### Goal
Allow users to undo recent actions.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/NotificationBanner.tsx`
- `web/src/components/NotificationBanner.css`
- New file: `web/src/services/undoManager.ts`

### Implementation Steps

#### 6.1 Create Undo Manager
```typescript
// web/src/services/undoManager.ts
export interface UndoAction {
  id: string;
  type: 'update' | 'delete' | 'bulk';
  description: string;
  imageGUIDs: string[];
  previousState: Partial<Image>[];
  timestamp: number;
}

class UndoManager {
  private stack: UndoAction[] = [];
  private maxSize = 20;
  private listeners: ((action: UndoAction | null) => void)[] = [];
  
  push(action: UndoAction) {
    this.stack.push(action);
    if (this.stack.length > this.maxSize) {
      this.stack.shift();
    }
    this.notify();
  }
  
  pop(): UndoAction | null {
    const action = this.stack.pop() || null;
    this.notify();
    return action;
  }
  
  peek(): UndoAction | null {
    return this.stack[this.stack.length - 1] || null;
  }
  
  subscribe(listener: (action: UndoAction | null) => void) {
    this.listeners.push(listener);
    return () => {
      this.listeners = this.listeners.filter(l => l !== listener);
    };
  }
  
  private notify() {
    this.listeners.forEach(l => l(this.peek()));
  }
}

export const undoManager = new UndoManager();
```

#### 6.2 Record Actions Before Changes
```typescript
// Before updating an image
const recordAndUpdate = async (image: Image, update: UpdateImageRequest) => {
  const previousState = {
    groupNumber: image.groupNumber,
    rating: image.rating,
    status: image.status,
    reviewed: image.reviewed,
  };
  
  undoManager.push({
    id: crypto.randomUUID(),
    type: 'update',
    description: `Changed ${Object.keys(update).join(', ')}`,
    imageGUIDs: [image.imageGUID],
    previousState: [previousState],
    timestamp: Date.now(),
  });
  
  await updateImage(image.imageGUID, update);
};
```

#### 6.3 Handle Ctrl+Z
```typescript
useEffect(() => {
  const handleKeyDown = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'z') {
      e.preventDefault();
      handleUndo();
    }
  };
  
  window.addEventListener('keydown', handleKeyDown);
  return () => window.removeEventListener('keydown', handleKeyDown);
}, []);

const handleUndo = async () => {
  const action = undoManager.pop();
  if (!action) return;
  
  try {
    for (let i = 0; i < action.imageGUIDs.length; i++) {
      await updateImage(action.imageGUIDs[i], action.previousState[i]);
    }
    showNotification(`Undone: ${action.description}`, 'success');
    refreshImages();
  } catch (error) {
    showNotification('Failed to undo action', 'error');
  }
};
```

#### 6.4 Add Undo Button to Notifications
```tsx
// In NotificationBanner.tsx
interface NotificationBannerProps {
  message: string;
  type: 'success' | 'error' | 'info';
  onDismiss: () => void;
  undoAction?: () => void;
}

{undoAction && (
  <button className="undo-button" onClick={undoAction}>
    Undo
  </button>
)}
```

---

## Phase 7: Collapsible Sidebar

### Goal
Allow users to collapse the sidebar for more gallery space.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageGallery.css`

### Implementation Steps

#### 7.1 Add Collapse State
```typescript
const [sidebarCollapsed, setSidebarCollapsed] = useState(preferences.sidebarCollapsed);

const toggleSidebar = () => {
  const newState = !sidebarCollapsed;
  setSidebarCollapsed(newState);
  savePreference('sidebarCollapsed', newState);
};
```

#### 7.2 Sidebar Toggle Button
```tsx
<button className="sidebar-toggle" onClick={toggleSidebar}>
  {sidebarCollapsed ? '→' : '←'}
</button>

<aside className={`sidebar ${sidebarCollapsed ? 'collapsed' : ''}`}>
  {sidebarCollapsed ? (
    <div className="sidebar-icons">
      {/* Icon-only buttons */}
    </div>
  ) : (
    // Full sidebar content
  )}
</aside>
```

#### 7.3 CSS for Collapsed State
```css
.sidebar {
  width: 200px;
  transition: width 0.3s ease;
}

.sidebar.collapsed {
  width: 60px;
}

.sidebar.collapsed .sidebar-label,
.sidebar.collapsed .filter-text {
  display: none;
}

.sidebar-toggle {
  position: absolute;
  right: -12px;
  top: 50%;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: #3498db;
  border: none;
  color: white;
  cursor: pointer;
  z-index: 10;
}

/* Auto-collapse on narrow screens */
@media (max-width: 1000px) {
  .sidebar {
    width: 60px;
  }
  .sidebar .sidebar-label,
  .sidebar .filter-text {
    display: none;
  }
}
```

---

## Phase 8: Thumbnail Hover Preview

### Goal
Show larger preview on hover without opening modal.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageGallery.css`
- New file: `web/src/components/HoverPreview.tsx`

### Implementation Steps

#### 8.1 Create HoverPreview Component
```typescript
// web/src/components/HoverPreview.tsx
import React, { useState, useEffect } from 'react';
import { Image } from '../types';
import './HoverPreview.css';

interface HoverPreviewProps {
  image: Image | null;
  position: { x: number; y: number };
}

export const HoverPreview: React.FC<HoverPreviewProps> = ({ image, position }) => {
  const [visible, setVisible] = useState(false);
  
  useEffect(() => {
    if (image) {
      const timer = setTimeout(() => setVisible(true), 500);
      return () => clearTimeout(timer);
    } else {
      setVisible(false);
    }
  }, [image]);
  
  if (!image || !visible) return null;
  
  // Calculate position to keep preview on screen
  const previewWidth = 400;
  const previewHeight = 300;
  const padding = 20;
  
  let left = position.x + padding;
  let top = position.y - previewHeight / 2;
  
  if (left + previewWidth > window.innerWidth) {
    left = position.x - previewWidth - padding;
  }
  if (top < padding) top = padding;
  if (top + previewHeight > window.innerHeight - padding) {
    top = window.innerHeight - previewHeight - padding;
  }
  
  return (
    <div 
      className="hover-preview"
      style={{ left, top }}
    >
      <img src={image.thumbnail400} alt={image.originalFile} />
      <div className="hover-preview-info">
        <div className="filename">{image.originalFile}</div>
        <div className="dimensions">{image.width} × {image.height}</div>
        {image.keywords && (
          <div className="keywords">{image.keywords.slice(0, 5).join(', ')}</div>
        )}
      </div>
    </div>
  );
};
```

#### 8.2 CSS
```css
.hover-preview {
  position: fixed;
  z-index: 2000;
  background: #1a1a2e;
  border: 1px solid #2a3a5a;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
  animation: fadeIn 0.2s ease;
  pointer-events: none;
}

.hover-preview img {
  max-width: 400px;
  max-height: 300px;
  display: block;
}

.hover-preview-info {
  padding: 12px;
  background: rgba(0, 0, 0, 0.5);
}

.hover-preview .filename {
  font-weight: bold;
  margin-bottom: 4px;
}

.hover-preview .dimensions {
  color: #8892a6;
  font-size: 12px;
}

@keyframes fadeIn {
  from { opacity: 0; transform: scale(0.95); }
  to { opacity: 1; transform: scale(1); }
}
```

---

## Phase 9: Modal Zoom & Pan

### Goal
Allow zooming and panning in the image modal.

### Files to Modify
- `web/src/components/ImageModal.tsx`
- `web/src/components/ImageModal.css`

### Implementation Steps

#### 9.1 Add Zoom State
```typescript
const [zoom, setZoom] = useState(1);
const [pan, setPan] = useState({ x: 0, y: 0 });
const [isDragging, setIsDragging] = useState(false);
const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
const [fitMode, setFitMode] = useState<'fit' | 'fill'>('fit');

// Reset zoom when image changes
useEffect(() => {
  setZoom(1);
  setPan({ x: 0, y: 0 });
}, [image]);
```

#### 9.2 Zoom Handlers
```typescript
const handleWheel = (e: React.WheelEvent) => {
  e.preventDefault();
  const delta = e.deltaY > 0 ? 0.9 : 1.1;
  setZoom(prev => Math.min(Math.max(prev * delta, 0.5), 5));
};

const handleMouseDown = (e: React.MouseEvent) => {
  if (zoom > 1) {
    setIsDragging(true);
    setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y });
  }
};

const handleMouseMove = (e: React.MouseEvent) => {
  if (isDragging) {
    setPan({
      x: e.clientX - dragStart.x,
      y: e.clientY - dragStart.y,
    });
  }
};

const handleMouseUp = () => {
  setIsDragging(false);
};

const toggleFitMode = () => {
  setFitMode(prev => prev === 'fit' ? 'fill' : 'fit');
  setZoom(1);
  setPan({ x: 0, y: 0 });
};
```

#### 9.3 Image Container
```tsx
<div 
  className="modal-image-container"
  onWheel={handleWheel}
  onMouseDown={handleMouseDown}
  onMouseMove={handleMouseMove}
  onMouseUp={handleMouseUp}
  onMouseLeave={handleMouseUp}
>
  <img
    src={image.thumbnail400}
    alt={image.originalFile}
    style={{
      transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
      cursor: zoom > 1 ? (isDragging ? 'grabbing' : 'grab') : 'zoom-in',
      objectFit: fitMode,
    }}
  />
  <div className="zoom-controls">
    <button onClick={() => setZoom(prev => Math.min(prev * 1.2, 5))}>+</button>
    <span>{Math.round(zoom * 100)}%</span>
    <button onClick={() => setZoom(prev => Math.max(prev * 0.8, 0.5))}>-</button>
    <button onClick={() => { setZoom(1); setPan({ x: 0, y: 0 }); }}>Reset</button>
    <button onClick={toggleFitMode}>{fitMode === 'fit' ? 'Fill' : 'Fit'}</button>
  </div>
</div>
```

---

## Phase 10: Optional Filmstrip Navigation

### Goal
Add toggleable filmstrip thumbnail strip in modal.

### Files to Modify
- `web/src/components/ImageModal.tsx`
- `web/src/components/ImageModal.css`
- `web/src/services/preferences.ts`

### Implementation Steps

#### 10.1 Add Filmstrip State
```typescript
const [showFilmstrip, setShowFilmstrip] = useState(preferences.showFilmstrip);

const toggleFilmstrip = () => {
  const newState = !showFilmstrip;
  setShowFilmstrip(newState);
  savePreference('showFilmstrip', newState);
};
```

#### 10.2 Filmstrip Component
```tsx
{showFilmstrip && (
  <div className="filmstrip">
    <div className="filmstrip-track" ref={filmstripRef}>
      {images.map((img, index) => (
        <div
          key={img.imageGUID}
          className={`filmstrip-thumb ${img.imageGUID === image.imageGUID ? 'active' : ''}`}
          onClick={() => navigateToImage(index)}
        >
          <img src={img.thumbnail50} alt="" />
        </div>
      ))}
    </div>
  </div>
)}

<button className="filmstrip-toggle" onClick={toggleFilmstrip}>
  {showFilmstrip ? 'Hide Filmstrip' : 'Show Filmstrip'}
</button>
```

#### 10.3 Filmstrip CSS
```css
.filmstrip {
  position: absolute;
  bottom: 60px;
  left: 0;
  right: 0;
  height: 80px;
  background: rgba(0, 0, 0, 0.8);
  overflow-x: auto;
  overflow-y: hidden;
}

.filmstrip-track {
  display: flex;
  gap: 4px;
  padding: 8px;
  min-width: min-content;
}

.filmstrip-thumb {
  width: 60px;
  height: 60px;
  flex-shrink: 0;
  cursor: pointer;
  opacity: 0.6;
  transition: opacity 0.2s, transform 0.2s;
}

.filmstrip-thumb:hover {
  opacity: 0.8;
}

.filmstrip-thumb.active {
  opacity: 1;
  outline: 2px solid #3498db;
  transform: scale(1.1);
}

.filmstrip-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.filmstrip-toggle {
  position: absolute;
  bottom: 10px;
  right: 10px;
  background: rgba(0, 0, 0, 0.5);
  border: 1px solid #2a3a5a;
  color: #8892a6;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
}
```

---

## Phase 11: Progress Estimation

### Goal
Show ETA and detailed progress for long operations.

### Files to Modify
- `web/src/components/TransferBanner.tsx`
- `web/src/components/ZipProgressBanner.tsx`

### Implementation Steps

#### 11.1 Track Transfer Speed
```typescript
interface TransferProgress {
  current: number;
  total: number;
  startTime: number;
  bytesTransferred: number;
}

const [progress, setProgress] = useState<TransferProgress | null>(null);

const estimateTimeRemaining = (): string => {
  if (!progress || progress.current === 0) return 'Calculating...';
  
  const elapsed = Date.now() - progress.startTime;
  const rate = progress.current / elapsed; // items per ms
  const remaining = progress.total - progress.current;
  const etaMs = remaining / rate;
  
  if (etaMs < 60000) {
    return `~${Math.ceil(etaMs / 1000)}s remaining`;
  } else {
    return `~${Math.ceil(etaMs / 60000)}m remaining`;
  }
};
```

#### 11.2 Update Banner UI
```tsx
<div className="transfer-banner">
  <div className="transfer-info">
    <span className="transfer-count">
      Processing {progress.current} of {progress.total}
    </span>
    <span className="transfer-eta">{estimateTimeRemaining()}</span>
  </div>
  <div className="progress-bar">
    <div 
      className="progress-fill" 
      style={{ width: `${(progress.current / progress.total) * 100}%` }}
    />
  </div>
  <div className="transfer-filename">{currentFile}</div>
</div>
```

---

## Phase 12: Confirmation Dialogs

### Goal
Add optional confirmation for destructive actions.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/services/preferences.ts`
- New file: `web/src/components/ConfirmDialog.tsx`

### Implementation Steps

#### 12.1 Create ConfirmDialog Component
```typescript
// web/src/components/ConfirmDialog.tsx
interface ConfirmDialogProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
  showDontAskAgain?: boolean;
  onDontAskAgain?: (checked: boolean) => void;
}

export const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  isOpen,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  onConfirm,
  onCancel,
  showDontAskAgain = false,
  onDontAskAgain,
}) => {
  const [dontAsk, setDontAsk] = useState(false);
  
  if (!isOpen) return null;
  
  const handleConfirm = () => {
    if (dontAsk && onDontAskAgain) {
      onDontAskAgain(true);
    }
    onConfirm();
  };
  
  return (
    <div className="confirm-dialog-overlay">
      <div className="confirm-dialog">
        <h3>{title}</h3>
        <p>{message}</p>
        {showDontAskAgain && (
          <label className="dont-ask-checkbox">
            <input 
              type="checkbox" 
              checked={dontAsk}
              onChange={(e) => setDontAsk(e.target.checked)}
            />
            Don't ask again
          </label>
        )}
        <div className="confirm-dialog-buttons">
          <button onClick={onCancel}>{cancelLabel}</button>
          <button onClick={handleConfirm} className="confirm-btn">
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
};
```

#### 12.2 Integrate with Actions
```typescript
const handleDelete = async (image: Image) => {
  if (preferences.confirmOnDelete) {
    setConfirmDialog({
      isOpen: true,
      title: 'Delete Image',
      message: `Are you sure you want to delete "${image.originalFile}"?`,
      onConfirm: () => performDelete(image),
    });
  } else {
    await performDelete(image);
  }
};
```

---

## Phase 13: Drag & Drop

### Goal
Enable drag-and-drop for reordering and color assignment.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageGallery.css`

### Implementation Steps

#### 13.1 Add Drag State
```typescript
const [draggedImage, setDraggedImage] = useState<Image | null>(null);
const [dropTarget, setDropTarget] = useState<string | null>(null);
```

#### 13.2 Drag Handlers
```typescript
const handleDragStart = (e: React.DragEvent, image: Image) => {
  setDraggedImage(image);
  e.dataTransfer.effectAllowed = 'move';
  e.dataTransfer.setData('text/plain', image.imageGUID);
};

const handleDragOver = (e: React.DragEvent, target: string) => {
  e.preventDefault();
  setDropTarget(target);
};

const handleDrop = async (e: React.DragEvent, target: string) => {
  e.preventDefault();
  
  if (draggedImage) {
    if (target.startsWith('color-')) {
      const colorGroup = parseInt(target.replace('color-', ''));
      await handleColorChange(draggedImage, colorGroup);
    }
  }
  
  setDraggedImage(null);
  setDropTarget(null);
};
```

#### 13.3 Color Drop Zones
```tsx
<div className="color-drop-zones">
  {[1, 2, 3, 4, 5].map(color => (
    <div
      key={color}
      className={`color-drop-zone color-${color} ${dropTarget === `color-${color}` ? 'active' : ''}`}
      onDragOver={(e) => handleDragOver(e, `color-${color}`)}
      onDrop={(e) => handleDrop(e, `color-${color}`)}
    >
      Drop for {['', 'Red', 'Yellow', 'Green', 'Blue', 'Purple'][color]}
    </div>
  ))}
</div>
```

---

## Phase 14: Loading Skeleton States

### Goal
Replace spinners with skeleton loaders.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageGallery.css`
- New file: `web/src/components/SkeletonLoader.tsx`

### Implementation Steps

#### 14.1 Create Skeleton Component
```typescript
// web/src/components/SkeletonLoader.tsx
export const ThumbnailSkeleton: React.FC = () => (
  <div className="thumbnail-skeleton">
    <div className="skeleton-image"></div>
    <div className="skeleton-meta">
      <div className="skeleton-line"></div>
      <div className="skeleton-line short"></div>
    </div>
  </div>
);

export const GallerySkeleton: React.FC<{ count?: number }> = ({ count = 12 }) => (
  <div className="gallery-skeleton">
    {Array.from({ length: count }).map((_, i) => (
      <ThumbnailSkeleton key={i} />
    ))}
  </div>
);
```

#### 14.2 Skeleton CSS with Shimmer
```css
.thumbnail-skeleton {
  background: #1a1a2e;
  border-radius: 8px;
  overflow: hidden;
}

.skeleton-image {
  width: 100%;
  padding-bottom: 75%;
  background: linear-gradient(
    90deg,
    #1a1a2e 0%,
    #2a3a5a 50%,
    #1a1a2e 100%
  );
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
}

.skeleton-line {
  height: 12px;
  margin: 8px;
  background: #2a3a5a;
  border-radius: 4px;
  animation: shimmer 1.5s infinite;
}

.skeleton-line.short {
  width: 60%;
}

@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
```

---

## Phase 15: Keyboard Shortcut Help

### Goal
Provide discoverable keyboard shortcut documentation.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- New file: `web/src/components/KeyboardShortcutsHelp.tsx`

### Implementation Steps

#### 15.1 Create Help Overlay Component
```typescript
// web/src/components/KeyboardShortcutsHelp.tsx
const shortcuts = [
  { category: 'Navigation', items: [
    { keys: ['←', '→'], description: 'Previous/Next image' },
    { keys: ['↑', '↓'], description: 'Move up/down in grid' },
    { keys: ['Home', 'End'], description: 'First/Last image' },
    { keys: ['Enter'], description: 'Open selected image' },
    { keys: ['Esc'], description: 'Close modal / Clear selection' },
  ]},
  { category: 'Actions', items: [
    { keys: ['1-5'], description: 'Assign color group' },
    { keys: ['Enter'], description: 'Approve image' },
    { keys: ['R'], description: 'Reject image' },
    { keys: ['D', 'Delete'], description: 'Delete image' },
    { keys: ['U'], description: 'Undelete image' },
  ]},
  { category: 'Selection', items: [
    { keys: ['Space'], description: 'Toggle selection' },
    { keys: ['Shift+Click'], description: 'Range select' },
    { keys: ['Ctrl+Click'], description: 'Add to selection' },
    { keys: ['Ctrl+A'], description: 'Select all' },
  ]},
  { category: 'Other', items: [
    { keys: ['/'], description: 'Focus search' },
    { keys: ['?'], description: 'Show this help' },
    { keys: ['Ctrl+Z'], description: 'Undo last action' },
  ]},
];

export const KeyboardShortcutsHelp: React.FC<{ onClose: () => void }> = ({ onClose }) => (
  <div className="shortcuts-overlay" onClick={onClose}>
    <div className="shortcuts-dialog" onClick={e => e.stopPropagation()}>
      <h2>Keyboard Shortcuts</h2>
      <button className="close-btn" onClick={onClose}>×</button>
      <div className="shortcuts-grid">
        {shortcuts.map(category => (
          <div key={category.category} className="shortcut-category">
            <h3>{category.category}</h3>
            {category.items.map((item, i) => (
              <div key={i} className="shortcut-item">
                <div className="shortcut-keys">
                  {item.keys.map(key => <kbd key={key}>{key}</kbd>)}
                </div>
                <div className="shortcut-desc">{item.description}</div>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  </div>
);
```

#### 15.2 Add ? Key Handler
```typescript
// In ImageGallery.tsx
case '?':
  e.preventDefault();
  setShowShortcutsHelp(true);
  break;
```

---

## Phase 16: Better Empty States

### Goal
Improve empty state messaging with guidance.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageGallery.css`

### Implementation Steps

#### 16.1 Create Empty State Component
```tsx
const EmptyState: React.FC<{ filter: string }> = ({ filter }) => {
  const messages: Record<string, { title: string; message: string; action?: string }> = {
    unreviewed: {
      title: 'No images to review',
      message: 'All images have been reviewed! Great job.',
      action: 'View approved images →',
    },
    approved: {
      title: 'No approved images',
      message: 'Images you approve will appear here.',
    },
    rejected: {
      title: 'No rejected images',
      message: 'Images you reject will appear here.',
    },
    deleted: {
      title: 'Trash is empty',
      message: 'Deleted images will appear here for 90 days.',
    },
    all: {
      title: 'No images found',
      message: 'Upload images to your S3 bucket to get started.',
    },
  };
  
  const content = messages[filter] || messages.all;
  
  return (
    <div className="empty-state">
      <div className="empty-state-icon">📷</div>
      <h3>{content.title}</h3>
      <p>{content.message}</p>
      {content.action && (
        <button className="empty-state-action">{content.action}</button>
      )}
    </div>
  );
};
```

---

## Phase 17: Touch/Mobile Gestures

### Goal
Add touch-friendly interactions for tablet users.

### Files to Modify
- `web/src/components/ImageModal.tsx`
- `web/src/components/ImageGallery.tsx`

### Implementation Steps

#### 17.1 Add Touch Handlers for Swipe
```typescript
const touchStart = useRef<{ x: number; y: number } | null>(null);

const handleTouchStart = (e: React.TouchEvent) => {
  touchStart.current = {
    x: e.touches[0].clientX,
    y: e.touches[0].clientY,
  };
};

const handleTouchEnd = (e: React.TouchEvent) => {
  if (!touchStart.current) return;
  
  const deltaX = e.changedTouches[0].clientX - touchStart.current.x;
  const deltaY = e.changedTouches[0].clientY - touchStart.current.y;
  
  const minSwipe = 50;
  
  if (Math.abs(deltaX) > Math.abs(deltaY)) {
    // Horizontal swipe
    if (deltaX > minSwipe) {
      navigatePrevious();
    } else if (deltaX < -minSwipe) {
      navigateNext();
    }
  } else {
    // Vertical swipe
    if (deltaY < -minSwipe) {
      handleApprove();
    } else if (deltaY > minSwipe) {
      handleReject();
    }
  }
  
  touchStart.current = null;
};
```

#### 17.2 Pinch-to-Zoom
```typescript
const handleTouchMove = (e: React.TouchEvent) => {
  if (e.touches.length === 2) {
    // Pinch zoom
    const distance = Math.hypot(
      e.touches[0].clientX - e.touches[1].clientX,
      e.touches[0].clientY - e.touches[1].clientY
    );
    
    if (lastPinchDistance.current) {
      const scale = distance / lastPinchDistance.current;
      setZoom(prev => Math.min(Math.max(prev * scale, 0.5), 5));
    }
    
    lastPinchDistance.current = distance;
  }
};
```

---

## Phase 18: Session Persistence

### Goal
Remember scroll position and enable deep linking.

### Files to Modify
- `web/src/components/ImageGallery.tsx`
- `web/src/components/ImageModal.tsx`

### Implementation Steps

#### 18.1 Save/Restore Scroll Position
```typescript
const SCROLL_KEY = 'kill-snap-scroll';

// Save scroll position before unload
useEffect(() => {
  const saveScroll = () => {
    sessionStorage.setItem(SCROLL_KEY, String(window.scrollY));
  };
  
  window.addEventListener('beforeunload', saveScroll);
  return () => window.removeEventListener('beforeunload', saveScroll);
}, []);

// Restore on mount
useEffect(() => {
  const saved = sessionStorage.getItem(SCROLL_KEY);
  if (saved) {
    setTimeout(() => window.scrollTo(0, parseInt(saved)), 100);
    sessionStorage.removeItem(SCROLL_KEY);
  }
}, []);
```

#### 18.2 URL-Based State
```typescript
// Use URL params for current view state
const updateURLState = (imageGUID?: string) => {
  const url = new URL(window.location.href);
  if (imageGUID) {
    url.searchParams.set('image', imageGUID);
  } else {
    url.searchParams.delete('image');
  }
  window.history.replaceState({}, '', url.toString());
};

// Read initial state from URL
useEffect(() => {
  const params = new URLSearchParams(window.location.search);
  const imageGUID = params.get('image');
  if (imageGUID) {
    const image = images.find(img => img.imageGUID === imageGUID);
    if (image) openModal(image);
  }
}, [images]);
```

---

## New Files Summary

| File | Purpose |
|------|---------|
| `web/src/services/preferences.ts` | User preference management |
| `web/src/services/undoManager.ts` | Undo/redo action stack |
| `web/src/components/BulkActionBar.tsx` | Floating bulk action toolbar |
| `web/src/components/BulkActionBar.css` | Styles for bulk action bar |
| `web/src/components/HoverPreview.tsx` | Thumbnail hover preview popup |
| `web/src/components/HoverPreview.css` | Hover preview styles |
| `web/src/components/ConfirmDialog.tsx` | Confirmation dialog modal |
| `web/src/components/ConfirmDialog.css` | Confirmation dialog styles |
| `web/src/components/SkeletonLoader.tsx` | Loading skeleton components |
| `web/src/components/SkeletonLoader.css` | Skeleton animation styles |
| `web/src/components/KeyboardShortcutsHelp.tsx` | Keyboard shortcuts overlay |
| `web/src/components/KeyboardShortcutsHelp.css` | Shortcuts help styles |

---

## Implementation Order Recommendation

### Sprint 1 (Week 1-2): Core Productivity
1. **Phase 2: Persistent User Preferences** - Foundation for other features
2. **Phase 5: Image Count Badges** - Quick win, high visibility
3. **Phase 3: Keyboard Navigation in Gallery** - Power user productivity
4. **Phase 1: Multi-Select & Bulk Operations** - Major time savings

### Sprint 2 (Week 3-4): Search & Safety
5. **Phase 4: Search & Filter** - Essential for large libraries
6. **Phase 6: Undo Actions** - Prevents user frustration
7. **Phase 12: Confirmation Dialogs** - Safety net for destructive actions
8. **Phase 7: Collapsible Sidebar** - Better space utilization

### Sprint 3 (Week 5-6): Enhanced Viewing
9. **Phase 8: Thumbnail Hover Preview** - Faster preview
10. **Phase 9: Modal Zoom & Pan** - Detail inspection
11. **Phase 10: Optional Filmstrip Navigation** - Visual navigation
12. **Phase 14: Loading Skeleton States** - Better perceived performance

### Sprint 4 (Week 7-8): Polish & Mobile
13. **Phase 15: Keyboard Shortcut Help** - Discoverability
14. **Phase 16: Better Empty States** - Onboarding
15. **Phase 11: Progress Estimation** - User confidence
16. **Phase 18: Session Persistence** - Convenience

### Sprint 5 (Week 9-10): Advanced Features
17. **Phase 17: Touch/Mobile Gestures** - Mobile support
18. **Phase 13: Drag & Drop** - Advanced interaction

---

## Testing Checklist

For each phase, verify:
- [ ] Feature works as expected
- [ ] No regressions in existing functionality
- [ ] Keyboard accessibility maintained
- [ ] Responsive behavior on different screen sizes
- [ ] Performance with large image sets (1000+ images)
- [ ] Error handling for edge cases
- [ ] Preferences persist across sessions
