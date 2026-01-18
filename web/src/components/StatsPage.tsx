import React, { useState, useEffect, useCallback } from 'react';
import { api, SystemStats } from '../services/api';
import './StatsPage.css';

interface StatsPageProps {
  onClose: () => void;
}

const StatCard: React.FC<{
  title: string;
  value: number;
  status?: 'healthy' | 'warning' | 'alert';
  subtitle?: string;
}> = ({ title, value, status = 'healthy', subtitle }) => (
  <div className={`stat-card stat-${status}`}>
    <div className="stat-card-header">
      <span className="stat-card-title">{title}</span>
      {status !== 'healthy' && <span className={`stat-indicator ${status}`} />}
    </div>
    <div className="stat-card-value">{value.toLocaleString()}</div>
    {subtitle && <div className="stat-card-subtitle">{subtitle}</div>}
  </div>
);

const StatsSkeleton: React.FC = () => (
  <div className="stats-grid">
    {Array.from({ length: 9 }).map((_, i) => (
      <div key={i} className="stat-card skeleton">
        <div className="skeleton-title" />
        <div className="skeleton-value" />
      </div>
    ))}
  </div>
);

export const StatsPage: React.FC<StatsPageProps> = ({ onClose }) => {
  const [stats, setStats] = useState<SystemStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date());

  const fetchStats = useCallback(async () => {
    try {
      setError(null);
      const data = await api.getStats();
      setStats(data);
      setLastRefresh(new Date());
    } catch (err) {
      console.error('Failed to fetch stats:', err);
      setError('Failed to load stats');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStats();
    // Auto-refresh every 30 seconds
    const interval = setInterval(fetchStats, 30000);
    return () => clearInterval(interval);
  }, [fetchStats]);

  const getQueueStatus = (depth: number): 'healthy' | 'warning' | 'alert' => {
    if (depth === 0) return 'healthy';
    if (depth < 10) return 'warning';
    return 'alert';
  };

  const getDLQStatus = (depth: number): 'healthy' | 'warning' | 'alert' => {
    if (depth === 0) return 'healthy';
    if (depth < 5) return 'warning';
    return 'alert';
  };

  const getIncomingStatus = (count: number): 'healthy' | 'warning' | 'alert' => {
    if (count < 50) return 'healthy';
    if (count < 200) return 'warning';
    return 'alert';
  };

  const formatTime = (date: Date): string => {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  };

  return (
    <div className="stats-overlay" onClick={(e) => e.target === e.currentTarget && onClose()}>
      <div className="stats-modal">
        <div className="stats-header">
          <h2>System Stats</h2>
          <div className="stats-header-actions">
            <span className="last-refresh">
              Last refresh: {formatTime(lastRefresh)}
            </span>
            <button
              className="refresh-btn"
              onClick={fetchStats}
              disabled={loading}
              title="Refresh stats"
            >
              {loading ? 'Loading...' : 'Refresh'}
            </button>
            <button className="close-btn" onClick={onClose}>
              Close
            </button>
          </div>
        </div>

        {error ? (
          <div className="stats-error">
            <p>{error}</p>
            <button onClick={fetchStats}>Retry</button>
          </div>
        ) : loading && !stats ? (
          <StatsSkeleton />
        ) : stats ? (
          <>
            <div className="stats-section">
              <h3>Processing Pipeline</h3>
              <div className="stats-grid">
                <StatCard
                  title="Incoming"
                  value={stats.incomingCount}
                  status={getIncomingStatus(stats.incomingCount)}
                  subtitle="Files in S3 incoming/"
                />
                <StatCard
                  title="Queue Depth"
                  value={stats.sqsQueueDepth}
                  status={getQueueStatus(stats.sqsQueueDepth)}
                  subtitle="SQS pending messages"
                />
                <StatCard
                  title="DLQ Depth"
                  value={stats.sqsDlqDepth}
                  status={getDLQStatus(stats.sqsDlqDepth)}
                  subtitle="Failed processing"
                />
              </div>
            </div>

            <div className="stats-section">
              <h3>Image Counts</h3>
              <div className="stats-grid">
                <StatCard
                  title="Total Processed"
                  value={stats.processedCount}
                  subtitle="All images in system"
                />
                <StatCard
                  title="Unreviewed"
                  value={stats.unreviewedCount}
                  status={stats.unreviewedCount > 100 ? 'warning' : 'healthy'}
                  subtitle="Awaiting review"
                />
                <StatCard
                  title="Reviewed"
                  value={stats.reviewedCount}
                  subtitle="Total reviewed"
                />
              </div>
            </div>

            <div className="stats-section">
              <h3>Review Status</h3>
              <div className="stats-grid">
                <StatCard
                  title="Approved"
                  value={stats.approvedCount}
                  subtitle="Ready for projects"
                />
                <StatCard
                  title="Rejected"
                  value={stats.rejectedCount}
                  subtitle="Marked as rejected"
                />
                <StatCard
                  title="Deleted"
                  value={stats.deletedCount}
                  subtitle="Soft deleted"
                />
              </div>
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
};
