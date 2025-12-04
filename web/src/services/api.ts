import axios from 'axios';
import { API_BASE_URL, IMAGE_CDN_URL } from '../config';
import { authService } from './auth';
import { Image, UpdateImageRequest } from '../types';

export interface ImageFilters {
  state?: 'unreviewed' | 'approved' | 'rejected' | 'all';
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
};
