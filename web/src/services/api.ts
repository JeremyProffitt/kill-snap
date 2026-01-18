import axios, { AxiosError } from 'axios';
import { API_BASE_URL, IMAGE_CDN_URL } from '../config';
import { authService } from './auth';
import { Image, UpdateImageRequest, Project, AddToProjectRequest, LogsResponse } from '../types';

const RETRY_DELAYS = [300, 600, 1200, 2400]; // Exponential backoff for throttling

async function withRetry<T>(fn: () => Promise<T>): Promise<T> {
  let lastError: Error | undefined;

  for (let attempt = 0; attempt <= RETRY_DELAYS.length; attempt++) {
    try {
      return await fn();
    } catch (error) {
      lastError = error as Error;

      // Don't retry on client errors (4xx) except 429 (rate limit)
      if (error instanceof AxiosError && error.response) {
        const status = error.response.status;
        if (status >= 400 && status < 500 && status !== 429) {
          throw error;
        }
      }

      // If we've exhausted all retries, throw
      if (attempt >= RETRY_DELAYS.length) {
        throw error;
      }

      // Wait before retrying
      await new Promise(resolve => setTimeout(resolve, RETRY_DELAYS[attempt]));
    }
  }

  throw lastError;
}

export interface TransferProgressCallback {
  (currentFile: string, currentIndex: number, totalCount: number): void;
}

export interface ImageFilters {
  state?: 'unreviewed' | 'approved' | 'rejected' | 'deleted' | 'all';
  group?: number | 'all';
}

export interface SystemStats {
  incomingCount: number;
  processedCount: number;
  unreviewedCount: number;
  reviewedCount: number;
  approvedCount: number;
  rejectedCount: number;
  deletedCount: number;
  sqsQueueDepth: number;
  sqsDlqDepth: number;
  lastUpdated: string;
}

export const api = {
  async getImages(filters?: ImageFilters): Promise<Image[]> {
    const params = new URLSearchParams();
    if (filters?.state) {
      params.append('state', filters.state);
    }
    if (filters?.group !== undefined && filters.group !== 'all') {
      params.append('group', String(filters.group));
    }
    const queryString = params.toString();
    const url = queryString
      ? `${API_BASE_URL}/api/images?${queryString}`
      : `${API_BASE_URL}/api/images`;

    const response = await withRetry(() =>
      axios.get<Image[]>(url, {
        headers: authService.getAuthHeader(),
      })
    );
    return response.data;
  },

  async updateImage(imageId: string, update: UpdateImageRequest): Promise<void> {
    await withRetry(() =>
      axios.put(
        `${API_BASE_URL}/api/images/${imageId}`,
        update,
        { headers: authService.getAuthHeader() }
      )
    );
  },

  async deleteImage(imageId: string): Promise<void> {
    await withRetry(() =>
      axios.delete(
        `${API_BASE_URL}/api/images/${imageId}`,
        { headers: authService.getAuthHeader() }
      )
    );
  },

  async undeleteImage(imageId: string): Promise<void> {
    await withRetry(() =>
      axios.post(
        `${API_BASE_URL}/api/images/${imageId}/undelete`,
        {},
        { headers: authService.getAuthHeader() }
      )
    );
  },

  async getDownloadUrl(imageId: string): Promise<string> {
    const response = await withRetry(() =>
      axios.get<{ url: string }>(
        `${API_BASE_URL}/api/images/${imageId}/download`,
        { headers: authService.getAuthHeader() }
      )
    );
    return response.data.url;
  },

  getImageUrl(bucket: string, key: string): string {
    // Use CloudFront CDN URL if configured, otherwise fall back to S3 direct URL
    if (IMAGE_CDN_URL) {
      return `${IMAGE_CDN_URL}/${key}`;
    }
    return `https://${bucket}.s3.amazonaws.com/${key}`;
  },

  async getProjects(includeArchived?: boolean): Promise<Project[]> {
    const params = includeArchived ? '?includeArchived=true' : '';
    const response = await withRetry(() =>
      axios.get<Project[]>(
        `${API_BASE_URL}/api/projects${params}`,
        { headers: authService.getAuthHeader() }
      )
    );
    return response.data;
  },

  async createProject(name: string): Promise<Project> {
    const response = await withRetry(() =>
      axios.post<Project>(
        `${API_BASE_URL}/api/projects`,
        { name },
        { headers: authService.getAuthHeader() }
      )
    );
    return response.data;
  },

  async addToProject(projectId: string, filters: AddToProjectRequest): Promise<{ movedCount: number }> {
    const response = await withRetry(() =>
      axios.post<{ movedCount: number }>(
        `${API_BASE_URL}/api/projects/${projectId}/images`,
        filters,
        { headers: authService.getAuthHeader() }
      )
    );
    return response.data;
  },

  async getApprovedImages(filters: AddToProjectRequest): Promise<Image[]> {
    // Get approved images that match the filter criteria
    const params = new URLSearchParams();
    params.append('state', 'approved');
    if (!filters.all && filters.group !== undefined) {
      params.append('group', String(filters.group));
    }
    const response = await withRetry(() =>
      axios.get<Image[]>(
        `${API_BASE_URL}/api/images?${params.toString()}`,
        { headers: authService.getAuthHeader() }
      )
    );
    return response.data;
  },

  async addToProjectWithProgress(
    projectId: string,
    filters: AddToProjectRequest,
    onProgress: TransferProgressCallback
  ): Promise<{ movedCount: number; failedCount: number }> {
    // First, get the list of images to be moved for progress display
    const images = await this.getApprovedImages(filters);
    const totalCount = images.length;

    if (totalCount === 0) {
      return { movedCount: 0, failedCount: 0 };
    }

    // Start showing progress with file names
    // Simulate progress during the bulk transfer
    let progressIndex = 0;
    const progressInterval = setInterval(() => {
      if (progressIndex < images.length) {
        onProgress(images[progressIndex].originalFile, progressIndex + 1, totalCount);
        progressIndex++;
      }
    }, 100); // Show each file for 100ms minimum

    try {
      // Perform the actual bulk transfer
      const result = await this.addToProject(projectId, filters);

      // Clear progress interval
      clearInterval(progressInterval);

      // Show final progress
      onProgress(images[images.length - 1]?.originalFile || '', totalCount, totalCount);

      return { movedCount: result.movedCount, failedCount: 0 };
    } catch (err) {
      clearInterval(progressInterval);
      throw err;
    }
  },

  async getProjectImages(projectId: string): Promise<Image[]> {
    const response = await withRetry(() =>
      axios.get<Image[]>(
        `${API_BASE_URL}/api/projects/${projectId}/images`,
        { headers: authService.getAuthHeader() }
      )
    );
    return response.data;
  },

  async regenerateAI(imageId: string): Promise<{ keywords: string[]; description: string }> {
    const response = await withRetry(() =>
      axios.post<{ success: boolean; keywords: string[]; description: string }>(
        `${API_BASE_URL}/api/images/${imageId}/regenerate-ai`,
        {},
        { headers: authService.getAuthHeader() }
      )
    );
    return { keywords: response.data.keywords, description: response.data.description };
  },

  async generateZip(projectId: string): Promise<void> {
    await withRetry(() =>
      axios.post(
        `${API_BASE_URL}/api/projects/${projectId}/generate-zip`,
        {},
        { headers: authService.getAuthHeader() }
      )
    );
  },

  async getZipDownload(projectId: string, zipKey: string): Promise<{ url: string; filename: string; size: number }> {
    const response = await withRetry(() =>
      axios.get<{ url: string; filename: string; size: number }>(
        `${API_BASE_URL}/api/projects/${projectId}/zips/${encodeURIComponent(zipKey)}/download`,
        { headers: authService.getAuthHeader() }
      )
    );
    return response.data;
  },

  async getZipLogs(projectId: string): Promise<{ status: string; message: string; errorMessages?: string[]; elapsedMins?: number }> {
    const response = await withRetry(() =>
      axios.get<{ status: string; message: string; errorMessages?: string[]; elapsedMins?: number }>(
        `${API_BASE_URL}/api/projects/${projectId}/zip-logs`,
        { headers: authService.getAuthHeader() }
      )
    );
    return response.data;
  },

  async deleteZip(projectId: string, zipKey: string): Promise<void> {
    await withRetry(() =>
      axios.delete(
        `${API_BASE_URL}/api/projects/${projectId}/zips/${encodeURIComponent(zipKey)}`,
        { headers: authService.getAuthHeader() }
      )
    );
  },

  async deleteAllZips(projectId: string): Promise<void> {
    await withRetry(() =>
      axios.delete(
        `${API_BASE_URL}/api/projects/${projectId}/zips`,
        { headers: authService.getAuthHeader() }
      )
    );
  },

  async deleteProject(projectId: string): Promise<void> {
    await withRetry(() =>
      axios.delete(
        `${API_BASE_URL}/api/projects/${projectId}`,
        { headers: authService.getAuthHeader() }
      )
    );
  },

  async renameProject(projectId: string, name: string): Promise<void> {
    await withRetry(() =>
      axios.put(
        `${API_BASE_URL}/api/projects/${projectId}`,
        { name },
        { headers: authService.getAuthHeader() }
      )
    );
  },

  async updateProjectArchived(projectId: string, archived: boolean): Promise<void> {
    await withRetry(() =>
      axios.put(
        `${API_BASE_URL}/api/projects/${projectId}`,
        { archived },
        { headers: authService.getAuthHeader() }
      )
    );
  },

  async getUserSettings(): Promise<{ themeColor: string; themeStyle: string }> {
    try {
      const response = await withRetry(() =>
        axios.get<{ themeColor: string; themeStyle: string }>(
          `${API_BASE_URL}/api/user/settings`,
          { headers: authService.getAuthHeader() }
        )
      );
      return response.data;
    } catch (error) {
      // Return defaults if endpoint doesn't exist or fails
      return { themeColor: 'ocean-blue', themeStyle: 'rounded-modern' };
    }
  },

  async saveUserSettings(settings: { themeColor: string; themeStyle: string }): Promise<void> {
    try {
      await withRetry(() =>
        axios.put(
          `${API_BASE_URL}/api/user/settings`,
          settings,
          { headers: authService.getAuthHeader() }
        )
      );
    } catch (error) {
      // Silently fail - settings will be stored locally
      console.warn('Failed to save settings to server, using local storage');
    }
  },

  async getStats(): Promise<SystemStats> {
    const response = await withRetry(() =>
      axios.get<SystemStats>(`${API_BASE_URL}/api/stats`, {
        headers: authService.getAuthHeader()
      })
    );
    return response.data;
  },

  async getLogs(
    functionName: string,
    hours: number = 1,
    filter: 'error' | 'all' = 'error'
  ): Promise<LogsResponse> {
    const params = new URLSearchParams();
    params.append('function', functionName);
    params.append('hours', String(hours));
    params.append('filter', filter);
    const response = await axios.get<LogsResponse>(
      `${API_BASE_URL}/api/logs?${params.toString()}`,
      { headers: authService.getAuthHeader() }
    );
    return response.data;
  },
};
