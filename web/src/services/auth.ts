import axios from 'axios';
import { API_BASE_URL } from '../config';
import { LoginRequest, LoginResponse } from '../types';

const AUTH_TOKEN_KEY = 'authToken';

export const authService = {
  async login(credentials: LoginRequest): Promise<string> {
    const response = await axios.post<LoginResponse>(
      `${API_BASE_URL}/api/login`,
      credentials
    );
    const token = response.data.token;
    localStorage.setItem(AUTH_TOKEN_KEY, token);
    return token;
  },

  logout(): void {
    localStorage.removeItem(AUTH_TOKEN_KEY);
  },

  getToken(): string | null {
    return localStorage.getItem(AUTH_TOKEN_KEY);
  },

  isAuthenticated(): boolean {
    return !!this.getToken();
  },

  getAuthHeader(): Record<string, string> {
    const token = this.getToken();
    return token ? { Authorization: `Bearer ${token}` } : {};
  },
};
