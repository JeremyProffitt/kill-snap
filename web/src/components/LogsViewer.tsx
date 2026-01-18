import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../services/api';
import { LogEntry } from '../types';
import './LogsViewer.css';

type FunctionName = 'ImageThumbnailGenerator' | 'ImageReviewApi' | 'ProjectZipGenerator';
type TimeRange = 1 | 24 | 48 | 168;
type FilterMode = 'error' | 'all';

interface LogsViewerProps {
  onLogout: () => void;
}

const functionLabels: Record<FunctionName, string> = {
  ImageThumbnailGenerator: 'Thumbnail Generator',
  ImageReviewApi: 'Review API',
  ProjectZipGenerator: 'Zip Generator',
};

const timeRangeLabels: Record<TimeRange, string> = {
  1: '1 Hour',
  24: '24 Hours',
  48: '48 Hours',
  168: '7 Days',
};

export const LogsViewer: React.FC<LogsViewerProps> = ({ onLogout }) => {
  const [selectedFunction, setSelectedFunction] = useState<FunctionName>('ImageReviewApi');
  const [timeRange, setTimeRange] = useState<TimeRange>(1);
  const [filterMode, setFilterMode] = useState<FilterMode>('error');
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchLogs = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await api.getLogs(selectedFunction, timeRange, filterMode);
      setLogs(response.logs);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch logs');
      setLogs([]);
    } finally {
      setLoading(false);
    }
  }, [selectedFunction, timeRange, filterMode]);

  useEffect(() => {
    fetchLogs();
  }, [fetchLogs]);

  const formatTimestamp = (timestamp: string): string => {
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  return (
    <div className="logs-container">
      <div className="logs-sidebar">
        <div className="logs-sidebar-top">
          <h1 className="logs-sidebar-title">CloudWatch Logs</h1>

          <div className="logs-sidebar-section">
            <span className="logs-sidebar-label">Lambda Function</span>
            {(Object.entries(functionLabels) as [FunctionName, string][]).map(([fn, label]) => (
              <button
                key={fn}
                className={`logs-function-btn ${selectedFunction === fn ? 'active' : ''}`}
                onClick={() => setSelectedFunction(fn)}
              >
                {label}
              </button>
            ))}
          </div>

          <div className="logs-sidebar-divider" />

          <div className="logs-sidebar-section">
            <span className="logs-sidebar-label">Time Range</span>
            <div className="logs-time-grid">
              {(Object.entries(timeRangeLabels) as [string, string][]).map(([hours, label]) => (
                <button
                  key={hours}
                  className={`logs-time-btn ${timeRange === Number(hours) ? 'active' : ''}`}
                  onClick={() => setTimeRange(Number(hours) as TimeRange)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

          <div className="logs-sidebar-divider" />

          <div className="logs-sidebar-section">
            <span className="logs-sidebar-label">Filter</span>
            <div className="logs-filter-toggle">
              <button
                className={`logs-filter-btn ${filterMode === 'error' ? 'active' : ''}`}
                onClick={() => setFilterMode('error')}
              >
                Errors Only
              </button>
              <button
                className={`logs-filter-btn ${filterMode === 'all' ? 'active' : ''}`}
                onClick={() => setFilterMode('all')}
              >
                All Logs
              </button>
            </div>
          </div>

          <div className="logs-sidebar-divider" />

          <button
            className="logs-refresh-btn"
            onClick={fetchLogs}
            disabled={loading}
          >
            {loading ? 'Loading...' : 'Refresh'}
          </button>
        </div>

        <div className="logs-sidebar-bottom">
          <button className="logs-logout-btn" onClick={onLogout}>
            Logout
          </button>
        </div>
      </div>

      <div className="logs-content">
        <div className="logs-header">
          <h2>{functionLabels[selectedFunction]} Logs</h2>
          <span className="logs-count">
            {loading ? 'Loading...' : `${logs.length} entries`}
          </span>
        </div>

        {error && (
          <div className="logs-error">
            {error}
          </div>
        )}

        <div className="logs-list">
          {logs.length === 0 && !loading && !error && (
            <div className="logs-empty">
              No logs found for the selected time range and filter.
            </div>
          )}
          {logs.map((log, index) => (
            <div key={index} className="log-entry">
              <span className="log-timestamp">{formatTimestamp(log.timestamp)}</span>
              <pre className="log-message">{log.message}</pre>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
