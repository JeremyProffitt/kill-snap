import axios from 'axios';
import { API_BASE_URL, IMAGE_CDN_URL } from '../config';
import { authService } from './auth';
import { Image, UpdateImageRequest, Project, AddToProjectRequest } from '../types';

export interface TransferProgressCallback {
  (currentFile: string, currentIndex: number, totalCount: number): void;
}

export interface ImageFilters {
  state?: 'unreviewed' | 'approved' | 'rejected' | 'deleted' | 'all';
  group?: number | 'all';
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

    const response = await axios.get<Image[]>(url, {
      headers: authService.getAuthHeader(),
    });
    return response.data;
  },

  async updateImage(imageId: string, update: UpdateImageRequest): Promise<void> {
    await axios.put(
      `${API_BASE_URL}/api/images/${imageId}`,
      update,
      { headers: authService.getAuthHeader() }
    );
  },

  async deleteImage(imageId: string): Promise<void> {
    await axios.delete(
      `${API_BASE_URL}/api/images/${imageId}`,
      { headers: authService.getAuthHeader() }
    );
  },

  async undeleteImage(imageId: string): Promise<void> {
    await axios.post(
      `${API_BASE_URL}/api/images/${imageId}/undelete`,
      {},
      { headers: authService.getAuthHeader() }
    );
  },

  async getDownloadUrl(imageId: string): Promise<string> {
    const response = await axios.get<{ url: string }>(
      `${API_BASE_URL}/api/images/${imageId}/download`,
      { headers: authService.getAuthHeader() }
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

  async getProjects(): Promise<Project[]> {
    const response = await axios.get<Project[]>(
      `${API_BASE_URL}/api/projects`,
      { headers: authService.getAuthHeader() }
    );
    return response.data;
  },

  async createProject(name: string): Promise<Project> {
    const response = await axios.post<Project>(
      `${API_BASE_URL}/api/projects`,
      { name },
      { headers: authService.getAuthHeader() }
    );
    return response.data;
  },

  async addToProject(projectId: string, filters: AddToProjectRequest): Promise<{ movedCount: number }> {
    const response = await axios.post<{ movedCount: number }>(
      `${API_BASE_URL}/api/projects/${projectId}/images`,
      filters,
      { headers: authService.getAuthHeader() }
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
    const response = await axios.get<Image[]>(
      `${API_BASE_URL}/api/images?${params.toString()}`,
      { headers: authService.getAuthHeader() }
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
    const response = await axios.get<Image[]>(
      `${API_BASE_URL}/api/projects/${projectId}/images`,
      { headers: authService.getAuthHeader() }
    );
    return response.data;
  },

  async regenerateAI(imageId: string): Promise<{ keywords: string[]; description: string }> {
    const response = await axios.post<{ success: boolean; keywords: string[]; description: string }>(
      `${API_BASE_URL}/api/images/${imageId}/regenerate-ai`,
      {},
      { headers: authService.getAuthHeader() }
    );
    return { keywords: response.data.keywords, description: response.data.description };
  },

  async generateZip(projectId: string): Promise<void> {
    await axios.post(
      `${API_BASE_URL}/api/projects/${projectId}/generate-zip`,
      {},
      { headers: authService.getAuthHeader() }
    );
  },

  async getZipDownload(projectId: string, zipKey: string): Promise<{ url: string; filename: string; size: number }> {
    const response = await axios.get<{ url: string; filename: string; size: number }>(
      `${API_BASE_URL}/api/projects/${projectId}/zips/${encodeURIComponent(zipKey)}/download`,
      { headers: authService.getAuthHeader() }
    );
    return response.data;
  },

  async getZipLogs(projectId: string): Promise<{ status: string; message: string; errorMessages?: string[]; elapsedMins?: number }> {
    const response = await axios.get<{ status: string; message: string; errorMessages?: string[]; elapsedMins?: number }>(
      `${API_BASE_URL}/api/projects/${projectId}/zip-logs`,
      { headers: authService.getAuthHeader() }
    );
    return response.data;
  },

  async deleteZip(projectId: string, zipKey: string): Promise<void> {
    await axios.delete(
      `${API_BASE_URL}/api/projects/${projectId}/zips/${encodeURIComponent(zipKey)}`,
      { headers: authService.getAuthHeader() }
    );
  },
};
