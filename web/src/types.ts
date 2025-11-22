export interface Image {
  imageGUID: string;
  originalFile: string;
  thumbnail50: string;
  thumbnail400: string;
  bucket: string;
  width: number;
  height: number;
  fileSize: number;
  reviewed: string;
  groupNumber?: number;
  colorCode?: string;
  promoted?: boolean;
  exifData?: Record<string, string>;
  relatedFiles?: string[];
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
}

export interface UpdateImageRequest {
  groupNumber?: number;
  colorCode?: string;
  promoted?: boolean;
  reviewed?: string;
}
