import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { api, ImageFilters } from '../services/api';
import { authService } from '../services/auth';
import { Image, Project, ZipFile } from '../types';
import { ImageModal } from './ImageModal';
import { ProjectModal } from './ProjectModal';
import { TransferBanner, TransferProgress } from './TransferBanner';
import { ZipProgressBanner } from './ZipProgressBanner';
import { NotificationBanner, Notification } from './NotificationBanner';
import { BulkActionBar } from './BulkActionBar';
import { ConfirmDialog } from './ConfirmDialog';
import { KeyboardShortcutsHelp } from './KeyboardShortcutsHelp';
import { ThemeSettings } from './ThemeSettings';
import { StatsPage } from './StatsPage';
import { EmptyState } from './EmptyState';
import { PageSkeleton } from './SkeletonLoader';
import { getPreferences, savePreference, savePreferences, UserPreferences } from '../services/preferences';
import { undoManager, generateUndoId } from '../services/undoManager';
import { saveScrollPosition, restoreScrollPosition, updateURLState, getURLParam } from '../services/sessionStorage';
import { THEME_COLORS } from '../theme/themeConstants';
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

type StateFilter = 'unreviewed' | 'approved' | 'rejected' | 'deleted' | 'all';

const formatFileSize = (bytes: number): string => {
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)}M`;
  }
  return `${(bytes / 1024).toFixed(1)}K`;
};

const getFilename = (path: string): string => {
  const parts = path.split('/');
  return parts[parts.length - 1];
};

// Get display filename - prioritizes originalFilename, falls back to extracting from originalFile
const getDisplayFilename = (image: Image): string => {
  if (image.originalFilename) {
    // Add .jpg extension for display since originalFilename doesn't include it
    return image.originalFilename + '.jpg';
  }
  return getFilename(image.originalFile);
};

// Extract date from image (prefer EXIF DateTimeOriginal, fall back to InsertedDateTime)
const getImageDate = (image: Image): string => {
  if (image.exifData?.DateTimeOriginal) {
    const cleaned = image.exifData.DateTimeOriginal.replace(/"/g, '');
    const match = cleaned.match(/^(\d{4}):(\d{2}):(\d{2})/);
    if (match) {
      return `${match[1]}-${match[2]}-${match[3]}`;
    }
  }
  if (image.exifData?.DateTime) {
    const cleaned = image.exifData.DateTime.replace(/"/g, '');
    const match = cleaned.match(/^(\d{4}):(\d{2}):(\d{2})/);
    if (match) {
      return `${match[1]}-${match[2]}-${match[3]}`;
    }
  }
  if (image.insertedDateTime) {
    return image.insertedDateTime.split('T')[0];
  }
  return 'Unknown';
};

const formatDateForDisplay = (dateStr: string): string => {
  if (dateStr === 'Unknown') return 'Unknown Date';
  const date = new Date(dateStr);
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric'
  });
};

// Fuzzy match function for search
const fuzzyMatch = (text: string, query: string): boolean => {
  const lowerText = text.toLowerCase();
  const lowerQuery = query.toLowerCase();
  if (lowerText.includes(lowerQuery)) return true;
  let queryIndex = 0;
  for (let i = 0; i < lowerText.length && queryIndex < lowerQuery.length; i++) {
    if (lowerText[i] === lowerQuery[queryIndex]) {
      queryIndex++;
    }
  }
  return queryIndex === lowerQuery.length;
};

export const ImageGallery: React.FC<ImageGalleryProps> = ({ onLogout }) => {
  // Load preferences
  const [preferences, setPreferences] = useState<UserPreferences>(getPreferences);
  
  // Core state
  const [images, setImages] = useState<Image[]>([]);
  const [selectedImageGUID, setSelectedImageGUID] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingProgress, setLoadingProgress] = useState<{ loaded: number; loading: boolean }>({ loaded: 0, loading: false });
  const [error, setError] = useState('');
  const [processingIds, setProcessingIds] = useState<Set<string>>(new Set());
  
  // Filter state
  const [stateFilter, setStateFilter] = useState<StateFilter>(preferences.defaultStatusFilter as StateFilter);
  const [groupFilter, setGroupFilter] = useState<number | 'all'>(preferences.defaultColorFilter);
  const [filenameSearch, setFilenameSearch] = useState('');
  const [keywordSearch, setKeywordSearch] = useState('');
  const [debouncedFilename, setDebouncedFilename] = useState('');
  const [debouncedKeyword, setDebouncedKeyword] = useState('');
  
  // Project state
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProject, setSelectedProject] = useState<string>('');
  const [targetProject, setTargetProject] = useState<string>('');
  const [addingToProject, setAddingToProject] = useState(false);
  const [showProjectModal, setShowProjectModal] = useState(false);
  const [showAddProjectDialog, setShowAddProjectDialog] = useState(false);
  const [newProjectName, setNewProjectName] = useState('');
  const [creatingProject, setCreatingProject] = useState(false);
  const [showArchivedProjects, setShowArchivedProjects] = useState(false);
  
  // UI state
  const [hoverRating, setHoverRating] = useState<{ imageGUID: string; stars: number } | null>(null);
  const [selectedDate, setSelectedDate] = useState<string>('');
  const [thumbnailSize, setThumbnailSize] = useState<number>(preferences.thumbnailSize);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(preferences.sidebarCollapsed);
  const [generatingZip, setGeneratingZip] = useState(false);
  const [downloadingZip, setDownloadingZip] = useState<string | null>(null);
  
  // Selection state (multi-select)
  const [selectedImages, setSelectedImages] = useState<Set<string>>(new Set());
  const [lastSelectedIndex, setLastSelectedIndex] = useState<number | null>(null);
  const [focusedImageIndex, setFocusedImageIndex] = useState<number | null>(null);

  // Confirm dialog state
  const [confirmDialog, setConfirmDialog] = useState<{
    isOpen: boolean;
    title: string;
    message: string;
    onConfirm: () => void;
    confirmLabel?: string;
    confirmVariant?: 'danger' | 'primary' | 'warning';
  }>({ isOpen: false, title: '', message: '', onConfirm: () => {} });
  
  // Keyboard shortcuts help
  const [showShortcutsHelp, setShowShortcutsHelp] = useState(false);

  // Theme settings
  const [showThemeSettings, setShowThemeSettings] = useState(false);

  // Stats page
  const [showStats, setShowStats] = useState(false);
  
  // Transfer and notification state
  const [transferProgress, setTransferProgress] = useState<TransferProgress>({
    isActive: false,
    currentFile: '',
    currentIndex: 0,
    totalCount: 0,
    projectName: '',
    status: 'transferring',
  });
  const [notification, setNotification] = useState<Notification | null>(null);
  
  // Refs
  const galleryRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  // Debounce search inputs
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedFilename(filenameSearch), 300);
    return () => clearTimeout(timer);
  }, [filenameSearch]);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedKeyword(keywordSearch), 300);
    return () => clearTimeout(timer);
  }, [keywordSearch]);

  // Apply theme colors as CSS variables
  useEffect(() => {
    const color = THEME_COLORS.find(c => c.id === preferences.themeColor) || THEME_COLORS[0];
    document.documentElement.style.setProperty('--theme-primary', color.primary);
    document.documentElement.style.setProperty('--theme-secondary', color.secondary);
    document.documentElement.style.setProperty('--theme-background', color.background);
    document.documentElement.style.setProperty('--theme-sidebar', color.sidebar);
  }, [preferences.themeColor]);

  // Theme change handler
  const handleThemeChange = useCallback((colorId: string, styleId: string) => {
    savePreferences({ themeColor: colorId, themeStyle: styleId });
    setPreferences(prev => ({ ...prev, themeColor: colorId, themeStyle: styleId }));
  }, []);

  // Save scroll position before unload
  useEffect(() => {
    const handleBeforeUnload = () => saveScrollPosition();
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, []);

  // Restore scroll position on mount
  useEffect(() => {
    restoreScrollPosition();
  }, []);

  // Check URL for deep-linked image
  useEffect(() => {
    const imageGUID = getURLParam('image');
    if (imageGUID && images.length > 0) {
      const image = images.find(img => img.imageGUID === imageGUID);
      if (image) {
        setSelectedImageGUID(imageGUID);
      }
    }
  }, [images]);

  const loadProjects = useCallback(async () => {
    try {
      // Always fetch all projects including archived so we can filter in frontend
      const data = await api.getProjects(true);
      setProjects(data);
    } catch (err: any) {
      console.error('Failed to load projects:', err);
    }
  }, []);

  const loadImages = useCallback(async () => {
    try {
      setLoading(true);
      setLoadingProgress({ loaded: 0, loading: true });
      let data: Image[];

      if (selectedProject) {
        data = await api.getProjectImages(selectedProject);
        setLoadingProgress({ loaded: data.length, loading: false });
      } else {
        const filters: ImageFilters = {
          state: stateFilter,
          group: groupFilter,
        };
        data = await api.getImages(filters, (progress) => {
          setLoadingProgress(progress);
        });
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
      setLoadingProgress(prev => ({ ...prev, loading: false }));
    }
  }, [stateFilter, groupFilter, selectedProject, onLogout]);

  useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  useEffect(() => {
    loadImages();
  }, [loadImages]);

  // Auto-refresh when images have pending/moving status
  useEffect(() => {
    const hasPendingMoves = images.some(
      img => img.moveStatus === 'pending' || img.moveStatus === 'moving'
    );

    if (hasPendingMoves) {
      const interval = setInterval(() => {
        loadImages();
      }, 2000);

      return () => clearInterval(interval);
    }
  }, [images, loadImages]);

  // Image counts for badges
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

      if (img.groupNumber && img.groupNumber > 0) {
        counts.colors[img.groupNumber] = (counts.colors[img.groupNumber] || 0) + 1;
      }
    });

    return counts;
  }, [images]);

  // Filter projects: active (non-archived) and check if any archived exist
  const activeProjects = useMemo(() =>
    projects.filter(p => !p.archived),
    [projects]
  );
  const hasArchivedProjects = useMemo(() =>
    projects.some(p => p.archived),
    [projects]
  );
  // Get projects to show in dropdowns based on showArchivedProjects toggle
  const visibleProjects = useMemo(() =>
    showArchivedProjects ? projects : activeProjects,
    [projects, activeProjects, showArchivedProjects]
  );

  // Filter images by search queries (filename and keyword separately)
  const searchFilteredImages = useMemo(() => {
    let result = images;

    if (debouncedFilename) {
      result = result.filter(img =>
        fuzzyMatch(img.originalFilename || '', debouncedFilename) ||
        fuzzyMatch(getFilename(img.originalFile), debouncedFilename)
      );
    }

    if (debouncedKeyword) {
      result = result.filter(img =>
        img.keywords?.some(kw => fuzzyMatch(kw, debouncedKeyword))
      );
    }

    return result;
  }, [images, debouncedFilename, debouncedKeyword]);

  // Calculate date counts from filtered images
  const dateCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    searchFilteredImages.forEach(img => {
      const date = getImageDate(img);
      counts[date] = (counts[date] || 0) + 1;
    });
    return Object.entries(counts)
      .sort((a, b) => b[0].localeCompare(a[0]))
      .map(([date, count]) => ({ date, count }));
  }, [searchFilteredImages]);

  // Filter images by selected date
  const filteredImages = useMemo(() => {
    if (!selectedDate) return searchFilteredImages;
    return searchFilteredImages.filter(img => getImageDate(img) === selectedDate);
  }, [searchFilteredImages, selectedDate]);

  // Group filtered images by date
  const imagesByDate = useMemo(() => {
    const groups: { date: string; images: Image[] }[] = [];
    const dateMap: Record<string, Image[]> = {};

    filteredImages.forEach(img => {
      const date = getImageDate(img);
      if (!dateMap[date]) {
        dateMap[date] = [];
      }
      dateMap[date].push(img);
    });

    const sortedDates = Object.keys(dateMap).sort((a, b) => b.localeCompare(a));
    sortedDates.forEach(date => {
      const sortedImages = dateMap[date].sort((a, b) =>
        getDisplayFilename(a).localeCompare(getDisplayFilename(b))
      );
      groups.push({ date, images: sortedImages });
    });

    return groups;
  }, [filteredImages]);

  // Flatten imagesByDate into a single sorted array
  const sortedImages = useMemo(() => {
    return imagesByDate.flatMap(group => group.images);
  }, [imagesByDate]);

  // Find selected image by GUID
  const selectedImage = selectedImageGUID
    ? sortedImages.find(img => img.imageGUID === selectedImageGUID) || null
    : null;

  // Calculate index for display purposes
  const selectedImageIndex = selectedImageGUID
    ? sortedImages.findIndex(img => img.imageGUID === selectedImageGUID)
    : -1;
  
  const currentProject = projects.find(p => p.projectId === selectedProject);
  const completedZips = currentProject?.zipFiles?.filter(z => z.status === 'complete') || [];
  const isGeneratingZipForProject = currentProject?.zipFiles?.some(z => z.status === 'generating') || false;

  // Notification helpers
  const showNotification = useCallback((message: string, type: 'success' | 'error' | 'info') => {
    setNotification({ id: Date.now().toString(), message, type });
  }, []);

  const dismissNotification = useCallback(() => {
    setNotification(null);
  }, []);

  // Update local images array when properties change
  const handlePropertyChange = useCallback((imageGUID: string, updates: Partial<Image>) => {
    setImages(prevImages =>
      prevImages.map(img =>
        img.imageGUID === imageGUID ? { ...img, ...updates } : img
      )
    );
  }, []);

  // Selection handlers
  const toggleImageSelection = useCallback((imageGUID: string, index: number, event?: React.MouseEvent | KeyboardEvent) => {
    const newSelection = new Set(selectedImages);
    const shiftKey = event && 'shiftKey' in event && event.shiftKey;
    const ctrlKey = event && (('ctrlKey' in event && event.ctrlKey) || ('metaKey' in event && event.metaKey));

    if (shiftKey && lastSelectedIndex !== null) {
      const start = Math.min(lastSelectedIndex, index);
      const end = Math.max(lastSelectedIndex, index);
      for (let i = start; i <= end; i++) {
        newSelection.add(sortedImages[i].imageGUID);
      }
    } else if (ctrlKey) {
      if (newSelection.has(imageGUID)) {
        newSelection.delete(imageGUID);
      } else {
        newSelection.add(imageGUID);
      }
    } else {
      newSelection.clear();
      newSelection.add(imageGUID);
    }

    setSelectedImages(newSelection);
    setLastSelectedIndex(index);
  }, [selectedImages, lastSelectedIndex, sortedImages]);

  const selectAllImages = useCallback(() => {
    const allGuids = new Set(sortedImages.map(img => img.imageGUID));
    setSelectedImages(allGuids);
  }, [sortedImages]);

  const clearSelection = useCallback(() => {
    setSelectedImages(new Set());
    setLastSelectedIndex(null);
    setFocusedImageIndex(null);
  }, []);

  const removeFromSelection = useCallback((imageGUIDs: string[]) => {
    setSelectedImages(prev => {
      const next = new Set(prev);
      imageGUIDs.forEach(id => next.delete(id));
      return next;
    });
  }, []);

  const selectAllInDateGroup = useCallback((dateImages: Image[]) => {
    const newSelection = new Set(selectedImages);
    dateImages.forEach(img => newSelection.add(img.imageGUID));
    setSelectedImages(newSelection);
  }, [selectedImages]);

  // Undo handler
  const handleUndo = useCallback(async () => {
    const action = undoManager.pop();
    if (!action) {
      showNotification('Nothing to undo', 'info');
      return;
    }

    try {
      for (let i = 0; i < action.imageGUIDs.length; i++) {
        await api.updateImage(action.imageGUIDs[i], action.previousState[i] as any);
      }
      showNotification(`Undone: ${action.description}`, 'success');
      loadImages();
    } catch (error) {
      showNotification('Failed to undo action', 'error');
    }
  }, [loadImages, showNotification]);

  // Quick action handler with undo support
  const handleQuickAction = useCallback(async (
    e: React.MouseEvent,
    image: Image,
    action: 'approve' | 'reject' | 'delete' | 'undelete',
    groupNumber?: number
  ) => {
    e.stopPropagation();
    e.preventDefault();

    const imageId = image.imageGUID;

    if (processingIds.has(imageId)) return;

    setProcessingIds(prev => new Set(prev).add(imageId));

    const previousState: Partial<Image> = {
      groupNumber: image.groupNumber,
      colorCode: image.colorCode,
      rating: image.rating,
      status: image.status,
      reviewed: image.reviewed,
    };

    const shouldRemoveFromView = (): boolean => {
      if (selectedProject) return false;

      if (action === 'approve' && groupNumber !== undefined && groupFilter !== 'all') {
        if (typeof groupFilter === 'number' && groupFilter > 0 && groupNumber !== groupFilter) {
          return true;
        }
        if (groupFilter === 0 && groupNumber > 0) {
          return true;
        }
      }

      if (stateFilter === 'all') return false;

      switch (action) {
        case 'approve':
          return stateFilter === 'unreviewed' || stateFilter === 'rejected';
        case 'reject':
          return stateFilter === 'unreviewed' || stateFilter === 'approved';
        case 'delete':
          return stateFilter !== 'deleted';
        case 'undelete':
          return stateFilter === 'deleted';
        default:
          return false;
      }
    };

    const originalImage = { ...image };
    const originalIndex = images.findIndex(img => img.imageGUID === imageId);
    const willRemove = shouldRemoveFromView();

    if (willRemove) {
      setImages(prev => prev.filter(img => img.imageGUID !== imageId));
      removeFromSelection([imageId]);
    } else {
      if (action === 'approve' && groupNumber !== undefined) {
        const colorName = GROUP_COLORS.find(g => g.number === groupNumber)?.name.toLowerCase() || 'white';
        handlePropertyChange(imageId, { groupNumber, colorCode: colorName });
      } else if (action === 'delete') {
        handlePropertyChange(imageId, { status: 'deleted' });
        removeFromSelection([imageId]);
      } else if (action === 'undelete') {
        handlePropertyChange(imageId, { status: 'inbox' });
      }
    }

    try {
      if (action === 'delete') {
        await api.deleteImage(imageId);
      } else if (action === 'undelete') {
        await api.undeleteImage(imageId);
      } else if (action === 'approve' && groupNumber !== undefined) {
        const colorName = GROUP_COLORS.find(g => g.number === groupNumber)?.name.toLowerCase() || 'white';
        await api.updateImage(imageId, {
          groupNumber,
          colorCode: colorName,
          promoted: false,
          reviewed: 'true',
        });
      } else if (action === 'reject') {
        await api.updateImage(imageId, {
          groupNumber: 0,
          colorCode: 'white',
          reviewed: 'true',
        });
      }

      undoManager.push({
        id: generateUndoId(),
        type: 'update',
        description: `${action} image`,
        imageGUIDs: [imageId],
        previousState: [previousState],
        timestamp: Date.now(),
      });
    } catch (err: any) {
      console.error(`Failed to ${action} image:`, err);
      const errorMessage = err.response?.data?.error || `Failed to ${action} image`;
      showNotification(errorMessage, 'error');

      if (willRemove && originalIndex !== -1) {
        setImages(prev => {
          const newImages = [...prev];
          newImages.splice(originalIndex, 0, originalImage);
          return newImages;
        });
      } else {
        handlePropertyChange(imageId, originalImage);
      }
    } finally {
      setProcessingIds(prev => {
        const next = new Set(prev);
        next.delete(imageId);
        return next;
      });
    }
  }, [processingIds, selectedProject, groupFilter, stateFilter, images, handlePropertyChange, showNotification, removeFromSelection]);

  // Bulk action handlers
  const performBulkApprove = useCallback(async (imagesToProcess: Image[], groupNumber: number) => {
    const imageIds = imagesToProcess.map(img => img.imageGUID);
    setProcessingIds(prev => {
      const next = new Set(prev);
      imageIds.forEach(id => next.add(id));
      return next;
    });

    const colorName = GROUP_COLORS.find(g => g.number === groupNumber)?.name.toLowerCase() || 'white';

    const previousStates = imagesToProcess.map(img => ({
      groupNumber: img.groupNumber,
      colorCode: img.colorCode,
      rating: img.rating,
      status: img.status,
      reviewed: img.reviewed,
    }));

    imagesToProcess.forEach(img => {
      handlePropertyChange(img.imageGUID, { groupNumber, colorCode: colorName });
    });

    const results = await Promise.allSettled(
      imagesToProcess.map(image =>
        api.updateImage(image.imageGUID, {
          groupNumber,
          colorCode: colorName,
          promoted: false,
          reviewed: 'true',
        })
      )
    );

    const failures = results.filter(r => r.status === 'rejected');
    if (failures.length > 0) {
      showNotification(`${failures.length} of ${imagesToProcess.length} images failed to approve`, 'error');
      loadImages();
    } else {
      showNotification(`${imagesToProcess.length} images approved`, 'success');
      undoManager.push({
        id: generateUndoId(),
        type: 'bulk',
        description: `bulk approve ${imagesToProcess.length} images`,
        imageGUIDs: imageIds,
        previousState: previousStates,
        timestamp: Date.now(),
      });
    }

    setProcessingIds(prev => {
      const next = new Set(prev);
      imageIds.forEach(id => next.delete(id));
      return next;
    });
    clearSelection();
  }, [handlePropertyChange, showNotification, loadImages, clearSelection]);

  const handleBulkApprove = useCallback(async (groupNumber: number) => {
    const imagesToProcess = Array.from(selectedImages)
      .map(guid => sortedImages.find(img => img.imageGUID === guid))
      .filter((img): img is Image => img !== undefined && !processingIds.has(img.imageGUID));

    if (imagesToProcess.length === 0) return;

    if (preferences.confirmOnDelete && imagesToProcess.length > 5) {
      setConfirmDialog({
        isOpen: true,
        title: 'Bulk Approve',
        message: `Are you sure you want to approve ${imagesToProcess.length} images?`,
        confirmLabel: 'Approve All',
        confirmVariant: 'primary',
        onConfirm: () => performBulkApprove(imagesToProcess, groupNumber),
      });
    } else {
      await performBulkApprove(imagesToProcess, groupNumber);
    }
  }, [selectedImages, sortedImages, processingIds, preferences.confirmOnDelete, performBulkApprove]);

  const handleBulkReject = useCallback(async () => {
    const imagesToProcess = Array.from(selectedImages)
      .map(guid => sortedImages.find(img => img.imageGUID === guid))
      .filter((img): img is Image => img !== undefined);

    if (imagesToProcess.length === 0) return;

    const imageIds = imagesToProcess.map(img => img.imageGUID);
    setProcessingIds(prev => {
      const next = new Set(prev);
      imageIds.forEach(id => next.add(id));
      return next;
    });

    const results = await Promise.allSettled(
      imagesToProcess.map(image =>
        api.updateImage(image.imageGUID, {
          groupNumber: 0,
          colorCode: 'white',
          reviewed: 'true',
        })
      )
    );

    const failures = results.filter(r => r.status === 'rejected');
    if (failures.length > 0) {
      showNotification(`${failures.length} of ${imagesToProcess.length} images failed to reject`, 'error');
    } else {
      showNotification(`${imagesToProcess.length} images rejected`, 'success');
    }

    loadImages();
    setProcessingIds(prev => {
      const next = new Set(prev);
      imageIds.forEach(id => next.delete(id));
      return next;
    });
    clearSelection();
  }, [selectedImages, sortedImages, showNotification, loadImages, clearSelection]);

  const performBulkDelete = useCallback(async (imagesToProcess: Image[]) => {
    const imageIds = imagesToProcess.map(img => img.imageGUID);
    setProcessingIds(prev => {
      const next = new Set(prev);
      imageIds.forEach(id => next.add(id));
      return next;
    });

    const results = await Promise.allSettled(
      imagesToProcess.map(image => api.deleteImage(image.imageGUID))
    );

    const failures = results.filter(r => r.status === 'rejected');
    if (failures.length > 0) {
      showNotification(`${failures.length} of ${imagesToProcess.length} images failed to delete`, 'error');
    } else {
      showNotification(`${imagesToProcess.length} images deleted`, 'success');
    }

    loadImages();
    setProcessingIds(prev => {
      const next = new Set(prev);
      imageIds.forEach(id => next.delete(id));
      return next;
    });
    clearSelection();
  }, [showNotification, loadImages, clearSelection]);

  const handleBulkDelete = useCallback(async () => {
    const imagesToProcess = Array.from(selectedImages)
      .map(guid => sortedImages.find(img => img.imageGUID === guid))
      .filter((img): img is Image => img !== undefined);

    if (imagesToProcess.length === 0) return;

    if (preferences.confirmOnDelete) {
      setConfirmDialog({
        isOpen: true,
        title: 'Delete Images',
        message: `Are you sure you want to delete ${imagesToProcess.length} images?`,
        confirmLabel: 'Delete All',
        confirmVariant: 'danger',
        onConfirm: () => performBulkDelete(imagesToProcess),
      });
    } else {
      await performBulkDelete(imagesToProcess);
    }
  }, [selectedImages, sortedImages, preferences.confirmOnDelete, performBulkDelete]);

  const handleBulkAddToProject = useCallback(() => {
    if (!targetProject) {
      showNotification('Please select a project first', 'info');
      return;
    }
    handleAddToProject();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [targetProject, showNotification]);

  // Date bulk action handler
  const handleDateBulkAction = useCallback(async (
    dateImages: Image[],
    action: 'approve' | 'reject' | 'delete',
    groupNumber?: number
  ) => {
    const imagesToProcess = dateImages.filter(img => !processingIds.has(img.imageGUID));
    if (imagesToProcess.length === 0) return;

    const imageIds = imagesToProcess.map(img => img.imageGUID);
    setProcessingIds(prev => {
      const next = new Set(prev);
      imageIds.forEach(id => next.add(id));
      return next;
    });

    const shouldRemoveFromView = (): boolean => {
      if (selectedProject) return false;
      if (action === 'approve' && groupNumber !== undefined && groupFilter !== 'all') {
        if (typeof groupFilter === 'number' && groupFilter > 0 && groupNumber !== groupFilter) {
          return true;
        }
        if (groupFilter === 0 && groupNumber > 0) {
          return true;
        }
      }
      if (stateFilter === 'all') return false;
      switch (action) {
        case 'approve':
          return stateFilter === 'unreviewed' || stateFilter === 'rejected';
        case 'reject':
          return stateFilter === 'unreviewed' || stateFilter === 'approved';
        case 'delete':
          return stateFilter !== 'deleted';
        default:
          return false;
      }
    };

    const willRemove = shouldRemoveFromView();

    if (willRemove) {
      setImages(prev => prev.filter(img => !imageIds.includes(img.imageGUID)));
      removeFromSelection(imageIds);
    } else {
      imagesToProcess.forEach(img => {
        if (action === 'approve' && groupNumber !== undefined) {
          const colorName = GROUP_COLORS.find(g => g.number === groupNumber)?.name.toLowerCase() || 'white';
          handlePropertyChange(img.imageGUID, { groupNumber, colorCode: colorName });
        } else if (action === 'delete') {
          handlePropertyChange(img.imageGUID, { status: 'deleted' });
        }
      });
      // Remove deleted images from selection even if not removed from view
      if (action === 'delete') {
        removeFromSelection(imageIds);
      }
    }

    const results = await Promise.allSettled(
      imagesToProcess.map(async (image) => {
        if (action === 'delete') {
          return api.deleteImage(image.imageGUID);
        } else if (action === 'approve' && groupNumber !== undefined) {
          const colorName = GROUP_COLORS.find(g => g.number === groupNumber)?.name.toLowerCase() || 'white';
          return api.updateImage(image.imageGUID, {
            groupNumber,
            colorCode: colorName,
            promoted: false,
            reviewed: 'true',
          });
        } else if (action === 'reject') {
          return api.updateImage(image.imageGUID, {
            groupNumber: 0,
            colorCode: 'white',
            reviewed: 'true',
          });
        }
      })
    );

    const failures = results.filter(r => r.status === 'rejected');
    if (failures.length > 0) {
      showNotification(`${failures.length} of ${imagesToProcess.length} images failed to ${action}`, 'error');
      loadImages();
    } else {
      showNotification(`${imagesToProcess.length} images ${action === 'approve' ? 'approved' : action === 'reject' ? 'rejected' : 'deleted'}`, 'success');
    }

    setProcessingIds(prev => {
      const next = new Set(prev);
      imageIds.forEach(id => next.delete(id));
      return next;
    });
  }, [processingIds, selectedProject, groupFilter, stateFilter, handlePropertyChange, showNotification, loadImages, removeFromSelection]);

  // Thumbnail rating handler
  const handleThumbnailRating = useCallback(async (e: React.MouseEvent, image: Image, stars: number) => {
    e.stopPropagation();
    e.preventDefault();
    if (processingIds.has(image.imageGUID)) return;

    const newRating = image.rating === stars ? 0 : stars;
    handlePropertyChange(image.imageGUID, { rating: newRating });

    try {
      await api.updateImage(image.imageGUID, { rating: newRating });
    } catch (err) {
      console.error('Failed to update rating:', err);
      handlePropertyChange(image.imageGUID, { rating: image.rating });
    }
  }, [processingIds, handlePropertyChange]);

  // Navigation handlers
  const handleImageClick = useCallback((imageGUID: string) => {
    setSelectedImageGUID(imageGUID);
    updateURLState({ image: imageGUID });
  }, []);

  const handleCloseModal = useCallback(() => {
    setSelectedImageGUID(null);
    updateURLState({ image: null });
  }, []);

  const handleImageUpdate = useCallback(async () => {
    await loadImages();
  }, [loadImages]);

  const handleNavigate = useCallback((direction: 'prev' | 'next') => {
    if (selectedImageGUID === null) return;
    const currentIndex = sortedImages.findIndex(img => img.imageGUID === selectedImageGUID);
    if (currentIndex === -1) return;

    if (direction === 'prev' && currentIndex > 0) {
      const newGUID = sortedImages[currentIndex - 1].imageGUID;
      setSelectedImageGUID(newGUID);
      updateURLState({ image: newGUID });
    } else if (direction === 'next' && currentIndex < sortedImages.length - 1) {
      const newGUID = sortedImages[currentIndex + 1].imageGUID;
      setSelectedImageGUID(newGUID);
      updateURLState({ image: newGUID });
    }
  }, [selectedImageGUID, sortedImages]);

  // Project handlers
  const handleProjectCreated = useCallback(async (newProjectId?: string) => {
    await loadProjects();
    if (newProjectId) {
      setTargetProject(newProjectId);
    }
    loadImages();
  }, [loadProjects, loadImages]);

  const handleProjectChange = useCallback((projectId: string) => {
    setSelectedProject(projectId);
    setSelectedDate('');
    if (projectId) {
      setStateFilter('all');
      setGroupFilter('all');
    }
  }, []);

  const handleAddToProject = useCallback(async () => {
    if (!targetProject || addingToProject) return;

    const projectName = projects.find(p => p.projectId === targetProject)?.name || 'project';

    setAddingToProject(true);
    setTransferProgress({
      isActive: true,
      currentFile: '',
      currentIndex: 0,
      totalCount: 0,
      projectName,
      status: 'transferring',
    });

    try {
      const filters = groupFilter !== 'all' ? { group: groupFilter } : { all: true };
      const result = await api.addToProjectWithProgress(
        targetProject,
        filters,
        (currentFile, currentIndex, totalCount) => {
          setTransferProgress(prev => ({
            ...prev,
            currentFile,
            currentIndex,
            totalCount,
          }));
        }
      );

      setTransferProgress(prev => ({
        ...prev,
        currentIndex: result.movedCount,
        totalCount: result.movedCount,
        status: 'complete',
      }));

      loadProjects();
      loadImages();
    } catch (err) {
      console.error('Failed to add to project:', err);
      setTransferProgress(prev => ({
        ...prev,
        status: 'error',
        errorMessage: 'Failed to add images to project',
      }));
    } finally {
      setAddingToProject(false);
    }
  }, [targetProject, addingToProject, projects, groupFilter, loadProjects, loadImages]);

  const handleDismissTransfer = useCallback(() => {
    setTransferProgress(prev => ({
      ...prev,
      isActive: false,
    }));
  }, []);

  const handleCreateProjectInline = useCallback(async () => {
    if (!newProjectName.trim()) return;

    setCreatingProject(true);
    try {
      const result = await api.createProject(newProjectName.trim());
      setNewProjectName('');
      setShowAddProjectDialog(false);
      if (result?.projectId) {
        setTargetProject(result.projectId);
        await loadProjects();
      }
    } catch (err) {
      console.error('Failed to create project:', err);
      showNotification('Failed to create project', 'error');
    } finally {
      setCreatingProject(false);
    }
  }, [newProjectName, loadProjects, showNotification]);

  const handleGenerateZip = useCallback(async () => {
    if (!selectedProject) return;

    setGeneratingZip(true);
    try {
      await api.generateZip(selectedProject);
      showNotification('Zip generation started', 'success');
      await loadProjects();
    } catch (err: any) {
      console.error('Failed to generate zip:', err);
      const errorMsg = err.response?.data?.error || 'Failed to start zip generation';
      showNotification(errorMsg, 'error');
    } finally {
      setGeneratingZip(false);
    }
  }, [selectedProject, showNotification, loadProjects]);

  const handleDownloadZip = useCallback(async (project: Project, zipFile: ZipFile) => {
    const zipId = `${project.projectId}-${zipFile.key}`;
    setDownloadingZip(zipId);
    try {
      const response = await api.getZipDownload(project.projectId, zipFile.key);
      const link = document.createElement('a');
      link.href = response.url;
      link.download = response.filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    } catch (err) {
      console.error('Failed to download zip:', err);
      showNotification('Failed to download zip file', 'error');
    } finally {
      setDownloadingZip(null);
    }
  }, [showNotification]);

  // UI handlers
  const handleLogout = useCallback(() => {
    authService.logout();
    onLogout();
  }, [onLogout]);

  const toggleSidebar = useCallback(() => {
    const newState = !sidebarCollapsed;
    setSidebarCollapsed(newState);
    savePreference('sidebarCollapsed', newState);
  }, [sidebarCollapsed]);

  const handleThumbnailSizeChange = useCallback((size: number) => {
    setThumbnailSize(size);
    savePreference('thumbnailSize', size);
  }, []);

  // Get estimated column count based on thumbnail size
  const getColumnCount = useCallback((): number => {
    const columnCounts: Record<number, number> = {
      1: 8, 2: 6, 3: 4, 4: 3, 5: 2
    };
    return columnCounts[thumbnailSize] || 4;
  }, [thumbnailSize]);

  // Get flat index for an image
  const getFlatIndex = useCallback((imageGUID: string): number => {
    return sortedImages.findIndex(img => img.imageGUID === imageGUID);
  }, [sortedImages]);

  // Keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return;
      }
      if (selectedImageGUID) return;

      const key = e.key;

      if (key === '?') {
        e.preventDefault();
        setShowShortcutsHelp(prev => !prev);
        return;
      }

      if ((e.ctrlKey || e.metaKey) && key === 'z') {
        e.preventDefault();
        handleUndo();
        return;
      }

      if ((e.ctrlKey || e.metaKey) && key === 'a') {
        e.preventDefault();
        selectAllImages();
        return;
      }

      if (key === '/') {
        e.preventDefault();
        searchInputRef.current?.focus();
        return;
      }

      if (key === 'Escape') {
        e.preventDefault();
        clearSelection();
        setShowShortcutsHelp(false);
        return;
      }

      const columnCount = getColumnCount();

      if (key === 'ArrowRight' || key === 'l') {
        e.preventDefault();
        setFocusedImageIndex(prev =>
          prev === null ? 0 : Math.min(prev + 1, sortedImages.length - 1)
        );
        return;
      }

      if (key === 'ArrowLeft' || key === 'h') {
        e.preventDefault();
        setFocusedImageIndex(prev =>
          prev === null ? 0 : Math.max(prev - 1, 0)
        );
        return;
      }

      if (key === 'ArrowDown' || key === 'j') {
        e.preventDefault();
        setFocusedImageIndex(prev =>
          prev === null ? 0 : Math.min(prev + columnCount, sortedImages.length - 1)
        );
        return;
      }

      if (key === 'ArrowUp' || key === 'k') {
        e.preventDefault();
        setFocusedImageIndex(prev =>
          prev === null ? 0 : Math.max(prev - columnCount, 0)
        );
        return;
      }

      if (key === 'Home') {
        e.preventDefault();
        setFocusedImageIndex(0);
        return;
      }

      if (key === 'End') {
        e.preventDefault();
        setFocusedImageIndex(sortedImages.length - 1);
        return;
      }

      if (key === 'Enter' && focusedImageIndex !== null) {
        e.preventDefault();
        handleImageClick(sortedImages[focusedImageIndex].imageGUID);
        return;
      }

      if (key === ' ' && focusedImageIndex !== null) {
        e.preventDefault();
        toggleImageSelection(sortedImages[focusedImageIndex].imageGUID, focusedImageIndex, e);
        return;
      }

      if (key >= '1' && key <= '5' && focusedImageIndex !== null) {
        e.preventDefault();
        const image = sortedImages[focusedImageIndex];
        if (image && image.status !== 'deleted') {
          handleQuickAction({ stopPropagation: () => {}, preventDefault: () => {} } as React.MouseEvent, image, 'approve', parseInt(key));
        }
        return;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selectedImageGUID, focusedImageIndex, sortedImages, handleUndo, selectAllImages, clearSelection, toggleImageSelection, handleQuickAction, handleImageClick, getColumnCount]);

  // Scroll focused image into view
  useEffect(() => {
    if (focusedImageIndex !== null) {
      const element = document.querySelector(`[data-image-index="${focusedImageIndex}"]`);
      element?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }
  }, [focusedImageIndex]);

  return (
    <div className="gallery-container">
      <ZipProgressBanner projects={projects} onComplete={loadProjects} />
      <TransferBanner progress={transferProgress} onDismiss={handleDismissTransfer} />
      <NotificationBanner notification={notification} onDismiss={dismissNotification} />
      <KeyboardShortcutsHelp isOpen={showShortcutsHelp} onClose={() => setShowShortcutsHelp(false)} />
      <ThemeSettings
        isOpen={showThemeSettings}
        onClose={() => setShowThemeSettings(false)}
        currentColorId={preferences.themeColor}
        currentStyleId={preferences.themeStyle}
        onThemeChange={handleThemeChange}
      />
      {showStats && (
        <StatsPage onClose={() => setShowStats(false)} />
      )}
      
      <ConfirmDialog
        isOpen={confirmDialog.isOpen}
        title={confirmDialog.title}
        message={confirmDialog.message}
        confirmLabel={confirmDialog.confirmLabel}
        confirmVariant={confirmDialog.confirmVariant}
        onConfirm={() => {
          confirmDialog.onConfirm();
          setConfirmDialog(prev => ({ ...prev, isOpen: false }));
        }}
        onCancel={() => setConfirmDialog(prev => ({ ...prev, isOpen: false }))}
        showDontAskAgain
        onDontAskAgain={(checked) => {
          if (checked) {
            savePreference('confirmOnDelete', false);
            setPreferences(prev => ({ ...prev, confirmOnDelete: false }));
          }
        }}
      />

      <aside className={`sidebar ${sidebarCollapsed ? 'collapsed' : ''}`}>
        <div className="sidebar-top">
          <div className="sidebar-header">
            <button
              className="sidebar-toggle-btn"
              onClick={toggleSidebar}
              title={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            >
              {sidebarCollapsed ? '»' : '«'}
            </button>
            <h1 className="sidebar-title">{sidebarCollapsed ? 'KS' : 'Kill Snap'}</h1>
            {!sidebarCollapsed && (
              <button
                className="shortcuts-btn"
                onClick={() => setShowShortcutsHelp(true)}
                title="Keyboard shortcuts (?)"
              >
                ⌨
              </button>
            )}
          </div>

          <div className="image-count-container">
            <span className="image-count-label">
              {selectedProject ? 'Project' : selectedDate ? 'Date' : 'Unreviewed'}
            </span>
            <span className="image-count-number">
              {filteredImages.length}
            </span>
          </div>

          {!sidebarCollapsed && (
            <>
              <div className="sidebar-section search-section">
                <label className="search-label">Filename</label>
                <div className="search-input-wrapper">
                  <input
                    ref={searchInputRef}
                    type="text"
                    placeholder="Search filename..."
                    value={filenameSearch}
                    onChange={(e) => setFilenameSearch(e.target.value)}
                    className="search-input"
                  />
                  {filenameSearch && (
                    <button
                      className="search-clear-btn"
                      onClick={() => setFilenameSearch('')}
                      title="Clear"
                    >
                      x
                    </button>
                  )}
                </div>
                <label className="search-label">Keyword</label>
                <div className="search-input-wrapper">
                  <input
                    type="text"
                    placeholder="Search keywords..."
                    value={keywordSearch}
                    onChange={(e) => setKeywordSearch(e.target.value)}
                    className="search-input"
                  />
                  {keywordSearch && (
                    <button
                      className="search-clear-btn"
                      onClick={() => setKeywordSearch('')}
                      title="Clear"
                    >
                      x
                    </button>
                  )}
                </div>
              </div>

              <div className="sidebar-section thumbnail-size-section">
                <label className="sidebar-label">Thumbnail Size</label>
                <div className="thumbnail-slider-container">
                  <span className="slider-label-small">S</span>
                  <input
                    type="range"
                    min="1"
                    max="5"
                    value={thumbnailSize}
                    onChange={(e) => handleThumbnailSizeChange(Number(e.target.value))}
                    className="thumbnail-slider"
                  />
                  <span className="slider-label-large">L</span>
                </div>
              </div>

              <div className="sidebar-section">
                <label className="sidebar-label">View by Date</label>
                <select
                  value={selectedDate}
                  onChange={(e) => setSelectedDate(e.target.value)}
                  className="sidebar-select"
                >
                  <option value="">All Dates</option>
                  {dateCounts.map(({ date, count }) => (
                    <option key={date} value={date}>
                      {formatDateForDisplay(date)} ({count})
                    </option>
                  ))}
                </select>
              </div>

              <div className="sidebar-section">
                <label className="sidebar-label">View by Project</label>
                <select
                  value={selectedProject}
                  onChange={(e) => handleProjectChange(e.target.value)}
                  className="sidebar-select"
                >
                  <option value="">Inbox</option>
                  {visibleProjects.map((project) => (
                    <option key={project.projectId} value={project.projectId}>
                      {project.name} ({project.imageCount}){project.archived ? ' [Archived]' : ''}
                    </option>
                  ))}
                </select>
                {hasArchivedProjects && (
                  <label className="show-archived-checkbox">
                    <input
                      type="checkbox"
                      checked={showArchivedProjects}
                      onChange={(e) => setShowArchivedProjects(e.target.checked)}
                    />
                    <span>Show archived projects</span>
                  </label>
                )}
              </div>

              {!selectedProject && (
                <>
                  <div className="sidebar-section">
                    <label className="sidebar-label">View by Status</label>
                    <select
                      value={stateFilter}
                      onChange={(e) => setStateFilter(e.target.value as StateFilter)}
                      className="sidebar-select"
                    >
                      <option value="unreviewed">Unreviewed ({imageCounts.unreviewed})</option>
                      <option value="approved">Approved ({imageCounts.approved})</option>
                      <option value="rejected">Rejected ({imageCounts.rejected})</option>
                      <option value="deleted">Deleted ({imageCounts.deleted})</option>
                      <option value="all">All ({imageCounts.all})</option>
                    </select>
                  </div>

                  <div className="sidebar-section">
                    <label className="sidebar-label">View by Group</label>
                    <div className="group-boxes-row">
                      <button
                        className={`group-box group-all ${groupFilter === 'all' ? 'active' : ''}`}
                        onClick={() => {
                          setGroupFilter('all');
                          setStateFilter('unreviewed');
                        }}
                        title="All Groups"
                      >
                        All
                      </button>
                      <button
                        className={`group-box group-ungrouped ${groupFilter === 0 ? 'active' : ''}`}
                        onClick={() => {
                          setGroupFilter(0);
                          setStateFilter('unreviewed');
                        }}
                        title="Ungrouped"
                      >
                        Ungrouped
                      </button>
                    </div>
                    <div className="group-boxes-row">
                      {GROUP_COLORS.slice(1).map((group) => (
                        <button
                          key={group.number}
                          className={`group-box ${groupFilter === group.number ? 'active' : ''}`}
                          style={{ backgroundColor: group.color }}
                          onClick={() => {
                            setGroupFilter(group.number);
                            setStateFilter('approved');
                          }}
                          title={`${group.name} (${imageCounts.colors[group.number] || 0})`}
                        >
                          {group.number}
                        </button>
                      ))}
                    </div>
                  </div>

                  <div className="sidebar-divider double-margin"></div>

                  <div className="sidebar-section add-to-project-section">
                    <select
                      value={targetProject}
                      onChange={(e) => setTargetProject(e.target.value)}
                      className="sidebar-select"
                    >
                      <option value="">Select Project...</option>
                      {activeProjects.map((project) => (
                        <option key={project.projectId} value={project.projectId}>
                          {project.name}
                        </option>
                      ))}
                    </select>
                    <button
                      onClick={handleAddToProject}
                      disabled={!targetProject || addingToProject}
                      className="sidebar-button primary"
                    >
                      {addingToProject ? 'Adding...' : 'Add to Project'}
                    </button>
                  </div>

                  <div className="sidebar-section">
                    <div className="projects-buttons-row">
                      <button
                        onClick={() => setShowAddProjectDialog(true)}
                        className="add-project-inline-btn"
                        title="Add new project"
                      >
                        Add Project
                      </button>
                      <button
                        onClick={() => setShowProjectModal(true)}
                        className="manage-projects-btn"
                      >
                        Manage
                      </button>
                    </div>
                  </div>
                </>
              )}

              {selectedProject && currentProject && (
                <>
                  <div className="sidebar-project-buttons">
                    <button
                      onClick={handleGenerateZip}
                      disabled={generatingZip || isGeneratingZipForProject || currentProject.imageCount === 0}
                      className="sidebar-button primary"
                      title={currentProject.imageCount === 0 ? 'No images in project' : 'Generate ZIP file'}
                    >
                      {generatingZip || isGeneratingZipForProject ? 'Generating...' : 'Create Zip'}
                    </button>
                    <button
                      onClick={() => setShowAddProjectDialog(true)}
                      className="sidebar-button secondary"
                    >
                      + Add
                    </button>
                  </div>

                  {completedZips.length > 0 && (
                    <div className="sidebar-zip-list">
                      <label className="sidebar-label">Downloads</label>
                      {completedZips.map((zipFile) => {
                        const zipId = `${currentProject.projectId}-${zipFile.key}`;
                        const filename = zipFile.key.split('/').pop() || 'download.zip';
                        return (
                          <button
                            key={zipFile.key}
                            type="button"
                            className="sidebar-zip-link"
                            onClick={() => handleDownloadZip(currentProject, zipFile)}
                            disabled={downloadingZip === zipId}
                          >
                            {downloadingZip === zipId ? 'Downloading...' : filename}
                          </button>
                        );
                      })}
                    </div>
                  )}

                  <button
                    onClick={() => setShowProjectModal(true)}
                    className="sidebar-button secondary"
                  >
                    Manage
                  </button>
                </>
              )}
            </>
          )}
        </div>

        <div className="sidebar-bottom">
          {!sidebarCollapsed && (
            <>
              <button
                className="settings-btn"
                onClick={() => setShowStats(true)}
                title="System stats"
              >
                Stats
              </button>
              <button
                className="settings-btn"
                onClick={() => setShowThemeSettings(true)}
                title="Appearance settings"
              >
                Appearance
              </button>
            </>
          )}
          <button onClick={handleLogout} className="logout-button">
            {sidebarCollapsed ? 'X' : 'Logout'}
          </button>
        </div>
      </aside>

      <main className="gallery-main" ref={galleryRef}>
        {loading ? (
          <div className="loading-container">
            <PageSkeleton sections={3} />
            {loadingProgress.loaded > 0 && (
              <div className="loading-progress">
                Loading images... {loadingProgress.loaded.toLocaleString()} loaded
                {loadingProgress.loading && <span className="loading-dots">...</span>}
              </div>
            )}
          </div>
        ) : error ? (
          <div className="error-message">{error}</div>
        ) : filteredImages.length === 0 ? (
          <EmptyState
            filter={stateFilter}
            selectedDate={selectedDate}
            onAction={stateFilter === 'unreviewed' ? () => setStateFilter('approved') : undefined}
          />
        ) : (
          <div className={`gallery-sections thumbnail-size-${thumbnailSize}`}>
            {imagesByDate.map((dateGroup) => (
              <div key={dateGroup.date} className="date-section">
                <div className="date-section-header">
                  <div className="date-title-container">
                    <span className="date-section-title">
                      {formatDateForDisplay(dateGroup.date)}
                      <span className="date-section-count">({dateGroup.images.length})</span>
                    </span>
                    <button
                      className="date-select-all-btn"
                      onClick={() => selectAllInDateGroup(dateGroup.images)}
                      title="Select all in this date"
                    >
                      Select All
                    </button>
                  </div>
                  <div className="date-section-line"></div>
                  <div className="date-section-actions">
                    <div className="date-colors">
                      {GROUP_COLORS.slice(1).map((group) => (
                        <button
                          key={group.number}
                          type="button"
                          className="date-color-btn"
                          style={{ backgroundColor: group.color }}
                          onClick={() => handleDateBulkAction(dateGroup.images, 'approve', group.number)}
                          title={`Approve all as ${group.name}`}
                        >
                          {group.number}
                        </button>
                      ))}
                    </div>
                    <div className="date-action-buttons">
                      <button
                        type="button"
                        className="action-btn-mini approve"
                        onClick={() => handleDateBulkAction(dateGroup.images, 'approve', 1)}
                        title="Approve all"
                      >
                        V
                      </button>
                      <button
                        type="button"
                        className="action-btn-mini reject"
                        onClick={() => handleDateBulkAction(dateGroup.images, 'reject')}
                        title="Reject all"
                      >
                        X
                      </button>
                      <button
                        type="button"
                        className="action-btn-mini delete"
                        onClick={() => handleDateBulkAction(dateGroup.images, 'delete')}
                        title="Delete all"
                      >
                        D
                      </button>
                    </div>
                  </div>
                </div>
                <div className="gallery-grid">
                  {dateGroup.images.map((image) => {
                    const isDeleted = image.status === 'deleted';
                    const flatIndex = getFlatIndex(image.imageGUID);
                    const isSelected = selectedImages.has(image.imageGUID);
                    const isFocused = focusedImageIndex === flatIndex;
                    
                    return (
                      <div
                        key={image.imageGUID}
                        data-image-index={flatIndex}
                        className={`gallery-item ${processingIds.has(image.imageGUID) ? 'processing' : ''} ${isDeleted ? 'deleted' : ''} ${isSelected ? 'selected' : ''} ${isFocused ? 'focused' : ''}`}
                        onClick={(e) => {
                          if (e.shiftKey || e.ctrlKey || e.metaKey) {
                            toggleImageSelection(image.imageGUID, flatIndex, e);
                          } else {
                            handleImageClick(image.imageGUID);
                          }
                        }}
                      >
                        <div
                          className={`selection-checkbox ${isSelected ? 'checked' : ''}`}
                          onClick={(e) => {
                            e.stopPropagation();
                            toggleImageSelection(image.imageGUID, flatIndex, e);
                          }}
                        >
                          {isSelected && 'V'}
                        </div>
                        
                        <div
                          className="thumbnail-container"
                          style={{
                            aspectRatio: image.width && image.height
                              ? `${image.width} / ${image.height}`
                              : '1 / 1'
                          }}
                        >
                          <img
                            src={api.getImageUrl(image.bucket, image.thumbnail400)}
                            alt={image.originalFile}
                            className="thumbnail"
                            loading="lazy"
                          />
                          {(image.moveStatus === 'pending' || image.moveStatus === 'moving') && (
                            <div className="move-status-indicator">
                              <span className="spinner"></span>
                              <span className="status-text">
                                {image.moveStatus === 'pending' ? 'Queued' : 'Moving...'}
                              </span>
                            </div>
                          )}
                          {image.moveStatus === 'failed' && (
                            <div className="move-status-indicator error">
                              <span className="status-text">Move Failed</span>
                            </div>
                          )}
                        </div>
                        <div className="image-info">
                          <div className="info-row-1">
                            <span className="thumb-filename">{getDisplayFilename(image)}</span>
                            <span className="thumb-size-info">
                              {image.width}x{image.height} - {formatFileSize(image.fileSize)}
                            </span>
                          </div>
                          {image.projectId && (
                            <div className="info-row-project">
                              <span className="thumb-project-name">
                                {projects.find(p => p.projectId === image.projectId)?.name || 'Unknown Project'}
                              </span>
                            </div>
                          )}
                          <div className="info-row-2">
                            {isDeleted ? (
                              <button
                                type="button"
                                className="undelete-btn"
                                onClick={(e) => handleQuickAction(e, image, 'undelete')}
                                title="Undelete"
                              >
                                Undelete
                              </button>
                            ) : (
                              <>
                                <div className="thumb-colors">
                                  {GROUP_COLORS.slice(1).map((group) => (
                                    <button
                                      key={group.number}
                                      type="button"
                                      className={`color-btn ${image.groupNumber === group.number ? 'selected' : ''}`}
                                      style={{ backgroundColor: group.color }}
                                      onClick={(e) => handleQuickAction(e, image, 'approve', group.number)}
                                      title={`${group.name} (${group.number})`}
                                    >
                                      {group.number}
                                    </button>
                                  ))}
                                </div>
                                <div className="thumb-actions">
                                  <button
                                    type="button"
                                    className="action-btn-mini approve"
                                    onClick={(e) => handleQuickAction(e, image, 'approve', image.groupNumber || 1)}
                                    title="Approve"
                                  >
                                    V
                                  </button>
                                  <button
                                    type="button"
                                    className="action-btn-mini reject"
                                    onClick={(e) => handleQuickAction(e, image, 'reject')}
                                    title="Reject"
                                  >
                                    X
                                  </button>
                                  <button
                                    type="button"
                                    className="action-btn-mini delete"
                                    onClick={(e) => handleQuickAction(e, image, 'delete')}
                                    title="Delete"
                                  >
                                    D
                                  </button>
                                </div>
                                <div
                                  className="thumb-rating"
                                  onMouseLeave={() => setHoverRating(null)}
                                >
                                  {[1, 2, 3, 4, 5].map((star) => {
                                    const currentRating = image.rating ?? 0;
                                    const isHovering = hoverRating?.imageGUID === image.imageGUID;
                                    const displayRating = isHovering ? hoverRating.stars : currentRating;
                                    const isFilled = displayRating >= star;
                                    return (
                                      <button
                                        key={star}
                                        type="button"
                                        className={`thumb-star-btn ${isFilled ? 'filled' : 'empty'} ${isHovering && star <= hoverRating.stars ? 'hover-preview' : ''}`}
                                        onClick={(e) => handleThumbnailRating(e, image, star)}
                                        onMouseEnter={() => setHoverRating({ imageGUID: image.imageGUID, stars: star })}
                                        title={`${star} star${star > 1 ? 's' : ''}`}
                                      >
                                        {isFilled ? '*' : 'o'}
                                      </button>
                                    );
                                  })}
                                </div>
                              </>
                            )}
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>
        )}
      </main>

      <BulkActionBar
        selectedCount={selectedImages.size}
        totalCount={sortedImages.length}
        onApprove={handleBulkApprove}
        onReject={handleBulkReject}
        onDelete={handleBulkDelete}
        onAddToProject={handleBulkAddToProject}
        onClearSelection={clearSelection}
        onSelectAll={selectAllImages}
      />

      {selectedImage && (
        <ImageModal
          image={selectedImage}
          images={sortedImages}
          projects={projects}
          onClose={handleCloseModal}
          onUpdate={handleImageUpdate}
          onNavigate={handleNavigate}
          onPropertyChange={handlePropertyChange}
          onNotify={showNotification}
          onProjectsUpdate={loadProjects}
          hasPrev={selectedImageIndex > 0}
          hasNext={selectedImageIndex < sortedImages.length - 1}
          currentIndex={selectedImageIndex}
          totalImages={sortedImages.length}
        />
      )}

      {showProjectModal && (
        <ProjectModal
          onClose={() => setShowProjectModal(false)}
          onProjectCreated={handleProjectCreated}
          existingProjects={projects}
        />
      )}

      {showAddProjectDialog && (
        <div className="add-dialog-backdrop" onClick={(e) => {
          if (e.target === e.currentTarget) {
            setShowAddProjectDialog(false);
            setNewProjectName('');
          }
        }}>
          <div className="add-dialog">
            <h3>Create New Project</h3>
            <input
              type="text"
              value={newProjectName}
              onChange={(e) => setNewProjectName(e.target.value)}
              placeholder="Enter project name..."
              disabled={creatingProject}
              onKeyDown={(e) => e.key === 'Enter' && handleCreateProjectInline()}
              autoFocus
            />
            <div className="add-dialog-buttons">
              <button
                type="button"
                className="dialog-btn cancel"
                onClick={() => {
                  setShowAddProjectDialog(false);
                  setNewProjectName('');
                }}
                disabled={creatingProject}
              >
                Cancel
              </button>
              <button
                type="button"
                className="dialog-btn create"
                onClick={handleCreateProjectInline}
                disabled={creatingProject || !newProjectName.trim()}
              >
                {creatingProject ? 'Creating...' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
