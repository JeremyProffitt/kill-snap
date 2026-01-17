// Session storage service - manages scroll position and session state

const SCROLL_KEY = 'kill-snap-scroll';
const LAST_IMAGE_KEY = 'kill-snap-last-image';
const FILTER_STATE_KEY = 'kill-snap-filter-state';

export interface FilterState {
  stateFilter: string;
  groupFilter: number | 'all';
  selectedDate: string;
  selectedProject: string;
}

// Scroll position management
export const saveScrollPosition = (): void => {
  try {
    sessionStorage.setItem(SCROLL_KEY, String(window.scrollY));
  } catch (e) {
    console.error('Failed to save scroll position:', e);
  }
};

export const restoreScrollPosition = (): void => {
  try {
    const saved = sessionStorage.getItem(SCROLL_KEY);
    if (saved) {
      setTimeout(() => window.scrollTo(0, parseInt(saved)), 100);
      sessionStorage.removeItem(SCROLL_KEY);
    }
  } catch (e) {
    console.error('Failed to restore scroll position:', e);
  }
};

// Last viewed image management
export const saveLastImage = (imageGUID: string | null): void => {
  try {
    if (imageGUID) {
      sessionStorage.setItem(LAST_IMAGE_KEY, imageGUID);
    } else {
      sessionStorage.removeItem(LAST_IMAGE_KEY);
    }
  } catch (e) {
    console.error('Failed to save last image:', e);
  }
};

export const getLastImage = (): string | null => {
  try {
    return sessionStorage.getItem(LAST_IMAGE_KEY);
  } catch (e) {
    console.error('Failed to get last image:', e);
    return null;
  }
};

// Filter state management
export const saveFilterState = (state: FilterState): void => {
  try {
    sessionStorage.setItem(FILTER_STATE_KEY, JSON.stringify(state));
  } catch (e) {
    console.error('Failed to save filter state:', e);
  }
};

export const getFilterState = (): FilterState | null => {
  try {
    const saved = sessionStorage.getItem(FILTER_STATE_KEY);
    return saved ? JSON.parse(saved) : null;
  } catch (e) {
    console.error('Failed to get filter state:', e);
    return null;
  }
};

// URL state management for deep linking
export const updateURLState = (params: Record<string, string | null>): void => {
  try {
    const url = new URL(window.location.href);
    Object.entries(params).forEach(([key, value]) => {
      if (value) {
        url.searchParams.set(key, value);
      } else {
        url.searchParams.delete(key);
      }
    });
    window.history.replaceState({}, '', url.toString());
  } catch (e) {
    console.error('Failed to update URL state:', e);
  }
};

export const getURLParam = (key: string): string | null => {
  try {
    const params = new URLSearchParams(window.location.search);
    return params.get(key);
  } catch (e) {
    console.error('Failed to get URL param:', e);
    return null;
  }
};
