// User preferences service - stores and retrieves user preferences from localStorage

export interface UserPreferences {
  thumbnailSize: number;
  sidebarCollapsed: boolean;
  defaultStatusFilter: string;
  defaultColorFilter: number | 'all';
  sortOrder: 'asc' | 'desc';
  showFilmstrip: boolean;
  confirmOnDelete: boolean;
  showKeyboardShortcuts: boolean;
  hoverPreviewEnabled: boolean;
  hoverPreviewDelay: number;
}

const PREFERENCES_KEY = 'kill-snap-preferences';

const defaultPreferences: UserPreferences = {
  thumbnailSize: 3,
  sidebarCollapsed: false,
  defaultStatusFilter: 'unreviewed',
  defaultColorFilter: 0,
  sortOrder: 'desc',
  showFilmstrip: false,
  confirmOnDelete: true,
  showKeyboardShortcuts: true,
  hoverPreviewEnabled: true,
  hoverPreviewDelay: 500,
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

export const resetPreferences = (): void => {
  try {
    localStorage.removeItem(PREFERENCES_KEY);
  } catch (e) {
    console.error('Failed to reset preferences:', e);
  }
};
