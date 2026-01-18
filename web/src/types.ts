export interface Image {
  imageGUID: string;
  originalFile: string;           // S3 key (UUID-based: images/{uuid}.jpg)
  originalFilename?: string;      // Original base filename without extension (e.g., "IMG_0001")
  rawFile?: string;               // S3 key of linked RAW file (e.g., images/{uuid}.CR2)
  thumbnail50: string;
  thumbnail400: string;
  bucket: string;
  width: number;
  height: number;
  fileSize: number;
  reviewed: string;
  groupNumber?: number;
  colorCode?: string;
  rating?: number;
  promoted?: boolean;
  keywords?: string[];
  description?: string;
  exifData?: Record<string, string>;
  relatedFiles?: string[];
  status?: 'inbox' | 'approved' | 'rejected' | 'deleted' | 'project';
  projectId?: string;
  moveStatus?: 'pending' | 'moving' | 'complete' | 'failed';
  insertedDateTime?: string;
  updatedDateTime?: string;
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
  rating?: number;
  promoted?: boolean;
  reviewed?: string;
  keywords?: string[];
}

export interface Project {
  projectId: string;
  name: string;
  s3Prefix?: string;
  createdAt: string;
  imageCount: number;
  zipFiles?: ZipFile[];
  archived?: boolean;
}

export interface ZipFile {
  key: string;
  size: number;
  imageCount: number;
  createdAt: string;
  status: 'generating' | 'complete' | 'failed';
}

export interface AddToProjectRequest {
  all?: boolean;
  group?: number;
  imageGUID?: string;
}

export interface LogEntry {
  timestamp: string;
  message: string;
}

export interface LogsResponse {
  logs: LogEntry[];
  count: number;
}
