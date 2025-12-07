import React, { useEffect, useState, useCallback } from 'react';
import { api } from '../services/api';
import { Project } from '../types';
import './ZipProgressBanner.css';

interface ZipProgressBannerProps {
  projects: Project[];
  onComplete: () => void;
}

interface ZipStatus {
  projectId: string;
  projectName: string;
  status: 'generating' | 'complete' | 'failed';
  elapsedMins?: number;
  message?: string;
  errorMessages?: string[];
}

export const ZipProgressBanner: React.FC<ZipProgressBannerProps> = ({ projects, onComplete }) => {
  const [zipStatuses, setZipStatuses] = useState<ZipStatus[]>([]);

  const checkZipStatus = useCallback(async (project: Project): Promise<ZipStatus | null> => {
    try {
      const response = await api.getZipLogs(project.projectId);
      return {
        projectId: project.projectId,
        projectName: project.name,
        status: response.status as 'generating' | 'complete' | 'failed',
        elapsedMins: response.elapsedMins,
        message: response.message,
        errorMessages: response.errorMessages,
      };
    } catch (err) {
      console.error('Failed to check zip status:', err);
      return null;
    }
  }, []);

  useEffect(() => {
    // Find projects with generating zips
    const generatingProjects = projects.filter(
      p => p.zipFiles?.some(z => z.status === 'generating')
    );

    if (generatingProjects.length === 0) {
      setZipStatuses([]);
      return;
    }

    // Initial check
    const checkAll = async () => {
      const statuses = await Promise.all(
        generatingProjects.map(p => checkZipStatus(p))
      );
      const validStatuses = statuses.filter((s): s is ZipStatus => s !== null);
      setZipStatuses(validStatuses);

      // If any completed or failed, trigger refresh
      if (validStatuses.some(s => s.status === 'complete' || s.status === 'failed')) {
        onComplete();
      }
    };

    checkAll();

    // Poll every 5 seconds
    const interval = setInterval(checkAll, 5000);
    return () => clearInterval(interval);
  }, [projects, checkZipStatus, onComplete]);

  if (zipStatuses.length === 0) return null;

  return (
    <div className="zip-progress-banner">
      {zipStatuses.map(status => (
        <div key={status.projectId} className={`zip-progress-item ${status.status}`}>
          <div className="zip-progress-icon">
            {status.status === 'generating' && (
              <div className="zip-spinner" />
            )}
            {status.status === 'complete' && (
              <span className="zip-check">&#10003;</span>
            )}
            {status.status === 'failed' && (
              <span className="zip-error">&#10005;</span>
            )}
          </div>
          <div className="zip-progress-text">
            <span className="zip-progress-title">
              {status.status === 'generating' && `Generating zip for "${status.projectName}"...`}
              {status.status === 'complete' && `Zip complete for "${status.projectName}"`}
              {status.status === 'failed' && `Zip failed for "${status.projectName}"`}
            </span>
            {status.status === 'generating' && status.elapsedMins !== undefined && (
              <span className="zip-progress-time">
                {status.elapsedMins} min elapsed
              </span>
            )}
            {status.status === 'failed' && status.errorMessages && (
              <span className="zip-progress-error">
                {status.errorMessages[0]}
              </span>
            )}
          </div>
        </div>
      ))}
    </div>
  );
};
