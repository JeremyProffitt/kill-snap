import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../services/api';
import { Project, ZipFile } from '../types';
import { TransferBanner, TransferProgress } from './TransferBanner';
import './ProjectModal.css';

interface ProjectModalProps {
  onClose: () => void;
  onProjectCreated: () => void;
  existingProjects: Project[];
}

// Lightroom color labels: Red, Yellow, Green, Blue, Purple
const GROUP_COLORS = [
  { number: 1, color: '#e74c3c', name: 'Red' },
  { number: 2, color: '#f1c40f', name: 'Yellow' },
  { number: 3, color: '#2ecc71', name: 'Green' },
  { number: 4, color: '#3498db', name: 'Blue' },
  { number: 5, color: '#9b59b6', name: 'Purple' },
];

const formatFileSize = (bytes: number): string => {
  if (bytes >= 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
  }
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
  return `${(bytes / 1024).toFixed(1)} KB`;
};

export const ProjectModal: React.FC<ProjectModalProps> = ({
  onClose,
  onProjectCreated,
  existingProjects,
}) => {
  const [mode, setMode] = useState<'existing' | 'create' | 'add'>('existing');
  const [projectName, setProjectName] = useState('');
  const [selectedProject, setSelectedProject] = useState<string>('');
  const [imageFilter, setImageFilter] = useState<'all' | number>('all');
  const [loading, setLoading] = useState(false);
  const [downloadingProject, setDownloadingProject] = useState<string | null>(null);
  const [generatingZip, setGeneratingZip] = useState<string | null>(null);
  const [downloadingZip, setDownloadingZip] = useState<string | null>(null);
  const [deletingZip, setDeletingZip] = useState<string | null>(null);
  const [result, setResult] = useState<{ success: boolean; message: string } | null>(null);
  const [zipErrors, setZipErrors] = useState<{ [projectId: string]: string[] }>({});
  const [transferProgress, setTransferProgress] = useState<TransferProgress>({
    isActive: false,
    currentFile: '',
    currentIndex: 0,
    totalCount: 0,
    projectName: '',
    status: 'transferring',
  });

  useEffect(() => {
    if (existingProjects.length > 0) {
      setSelectedProject(existingProjects[0].projectId);
    }
  }, [existingProjects]);

  // Check for zip generation timeout
  const checkZipStatus = useCallback(async (project: Project) => {
    const generatingZipFile = project.zipFiles?.find(z => z.status === 'generating');
    if (!generatingZipFile) return;

    try {
      const response = await api.getZipLogs(project.projectId);
      if (response.status === 'failed') {
        setZipErrors(prev => ({
          ...prev,
          [project.projectId]: response.errorMessages || ['Zip generation failed']
        }));
        onProjectCreated(); // Refresh projects to get updated status
      }
    } catch (err) {
      console.error('Failed to check zip status:', err);
    }
  }, [onProjectCreated]);

  // Periodically check for generating zips - refresh every 10 seconds to catch completion
  useEffect(() => {
    const projectsWithGeneratingZips = existingProjects.filter(
      p => p.zipFiles?.some(z => z.status === 'generating')
    );

    if (projectsWithGeneratingZips.length === 0) return;

    // Check for timeout/errors immediately
    projectsWithGeneratingZips.forEach(checkZipStatus);

    // Refresh projects every 10 seconds to check for completion
    const refreshInterval = setInterval(() => {
      onProjectCreated(); // This refreshes the projects list
    }, 10000);

    // Check for errors every 30 seconds
    const errorCheckInterval = setInterval(() => {
      projectsWithGeneratingZips.forEach(checkZipStatus);
    }, 30000);

    return () => {
      clearInterval(refreshInterval);
      clearInterval(errorCheckInterval);
    };
  }, [existingProjects, checkZipStatus, onProjectCreated]);

  const handleCreateProject = async () => {
    if (!projectName.trim()) {
      setResult({ success: false, message: 'Please enter a project name' });
      return;
    }

    setLoading(true);
    setResult(null);
    try {
      await api.createProject(projectName.trim());
      setResult({ success: true, message: `Project "${projectName}" created successfully!` });
      setProjectName('');
      onProjectCreated();
    } catch (err) {
      console.error('Failed to create project:', err);
      setResult({ success: false, message: 'Failed to create project' });
    } finally {
      setLoading(false);
    }
  };

  const handleAddToProject = async () => {
    if (!selectedProject) {
      setResult({ success: false, message: 'Please select a project' });
      return;
    }

    const projectName = existingProjects.find(p => p.projectId === selectedProject)?.name || 'project';

    setLoading(true);
    setResult(null);
    setTransferProgress({
      isActive: true,
      currentFile: '',
      currentIndex: 0,
      totalCount: 0,
      projectName,
      status: 'transferring',
    });

    try {
      const filters = imageFilter === 'all'
        ? { all: true }
        : { group: imageFilter };

      const response = await api.addToProjectWithProgress(
        selectedProject,
        filters,
        (currentFile, currentIndex, totalCount) => {
          setTransferProgress(prev => ({
            ...prev,
            currentFile,
            currentIndex,
            totalCount,
          }));
        }
      );

      setTransferProgress(prev => ({
        ...prev,
        currentIndex: response.movedCount,
        totalCount: response.movedCount,
        status: 'complete',
      }));

      setResult({
        success: true,
        message: `Moved ${response.movedCount} image(s) to "${projectName}"`
      });
      onProjectCreated();
    } catch (err) {
      console.error('Failed to add images to project:', err);
      setTransferProgress(prev => ({
        ...prev,
        status: 'error',
        errorMessage: 'Failed to add images to project',
      }));
      setResult({ success: false, message: 'Failed to add images to project' });
    } finally {
      setLoading(false);
    }
  };

  const handleDismissTransfer = () => {
    setTransferProgress(prev => ({
      ...prev,
      isActive: false,
    }));
  };

  const handleDownloadCatalog = async (project: Project) => {
    setDownloadingProject(project.projectId);
    setResult(null);
    try {
      const response = await api.getProjectCatalog(project.projectId);
      // Trigger download by opening the presigned URL
      const link = document.createElement('a');
      link.href = response.url;
      link.download = response.filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      setResult({
        success: true,
        message: `Downloading Lightroom catalog for "${project.name}"`
      });
    } catch (err: any) {
      console.error('Failed to download catalog:', err);
      const errorMsg = err.response?.status === 404
        ? 'No catalog found for this project'
        : 'Failed to download catalog';
      setResult({ success: false, message: errorMsg });
    } finally {
      setDownloadingProject(null);
    }
  };

  const handleGenerateZip = async (project: Project) => {
    // Clear any previous errors for this project
    setZipErrors(prev => {
      const newErrors = { ...prev };
      delete newErrors[project.projectId];
      return newErrors;
    });

    setGeneratingZip(project.projectId);
    setResult(null);
    try {
      await api.generateZip(project.projectId);
      setResult({
        success: true,
        message: `Zip generation started for "${project.name}". This may take several minutes.`
      });
      onProjectCreated(); // Refresh projects to see updated status
    } catch (err: any) {
      console.error('Failed to generate zip:', err);
      const errorMsg = err.response?.data?.error || 'Failed to start zip generation';
      setResult({ success: false, message: errorMsg });
    } finally {
      setGeneratingZip(null);
    }
  };

  const handleDownloadZip = async (project: Project, zipFile: ZipFile) => {
    const zipId = `${project.projectId}-${zipFile.key}`;
    setDownloadingZip(zipId);
    setResult(null);
    try {
      const response = await api.getZipDownload(project.projectId, zipFile.key);
      const link = document.createElement('a');
      link.href = response.url;
      link.download = response.filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      setResult({
        success: true,
        message: `Downloading ${response.filename}`
      });
    } catch (err: any) {
      console.error('Failed to download zip:', err);
      setResult({ success: false, message: 'Failed to download zip file' });
    } finally {
      setDownloadingZip(null);
    }
  };

  const handleDeleteZip = async (project: Project, zipFile: ZipFile) => {
    const zipId = `${project.projectId}-${zipFile.key}`;
    setDeletingZip(zipId);
    setResult(null);
    try {
      await api.deleteZip(project.projectId, zipFile.key);
      setResult({
        success: true,
        message: `Zip file deleted`
      });
      onProjectCreated(); // Refresh projects to see updated status
    } catch (err: any) {
      console.error('Failed to delete zip:', err);
      setResult({ success: false, message: 'Failed to delete zip file' });
    } finally {
      setDeletingZip(null);
    }
  };

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  const currentProject = existingProjects.find(p => p.projectId === selectedProject);

  const renderProjectDetails = (project: Project) => {
    const isGenerating = project.zipFiles?.some(z => z.status === 'generating');
    const hasFailed = project.zipFiles?.some(z => z.status === 'failed');
    const completedZips = project.zipFiles?.filter(z => z.status === 'complete') || [];
    const projectErrors = zipErrors[project.projectId];

    return (
      <div className="project-details">
        <div className="project-info-row">
          <span className="project-count">{project.imageCount} images</span>
          <span className="project-date">
            Created: {new Date(project.createdAt).toLocaleDateString()}
          </span>
        </div>
        <div className="project-actions">
          <button
            className="download-catalog-btn"
            onClick={() => handleDownloadCatalog(project)}
            disabled={downloadingProject === project.projectId || project.imageCount === 0}
            title={project.imageCount === 0 ? 'No images in project' : 'Download Lightroom Classic Catalog'}
          >
            {downloadingProject === project.projectId ? 'Downloading...' : 'Download Lightroom Classic Catalog'}
          </button>
          <div className="zip-action-container">
            <button
              className={`generate-zip-btn ${hasFailed || projectErrors ? 'failed' : ''}`}
              onClick={() => handleGenerateZip(project)}
              disabled={generatingZip === project.projectId || isGenerating || project.imageCount === 0}
              title={project.imageCount === 0 ? 'No images in project' : 'Generate ZIP file of original images'}
            >
              {generatingZip === project.projectId || isGenerating ? 'Generating...' : 'Generate ZIP'}
            </button>
            {projectErrors && (
              <div className="zip-error-messages">
                {projectErrors.map((error, idx) => (
                  <div key={idx} className="zip-error-message">{error}</div>
                ))}
              </div>
            )}
          </div>
        </div>
        {completedZips.length > 0 && (
          <div className="zip-files-list">
            <span className="zip-files-label">ZIP Files:</span>
            {completedZips.map((zipFile) => {
              const zipId = `${project.projectId}-${zipFile.key}`;
              const filename = zipFile.key.split('/').pop() || 'download.zip';
              return (
                <div key={zipFile.key} className="zip-file-item">
                  <span className="zip-file-info">
                    {filename} ({formatFileSize(zipFile.size)}, {zipFile.imageCount} images)
                  </span>
                  <div className="zip-file-actions">
                    <button
                      className="download-zip-btn"
                      onClick={() => handleDownloadZip(project, zipFile)}
                      disabled={downloadingZip === zipId || deletingZip === zipId}
                    >
                      {downloadingZip === zipId ? 'Downloading...' : 'Download'}
                    </button>
                    <button
                      className="delete-zip-btn"
                      onClick={() => handleDeleteZip(project, zipFile)}
                      disabled={downloadingZip === zipId || deletingZip === zipId}
                      title="Delete zip file"
                    >
                      {deletingZip === zipId ? 'Deleting...' : 'Delete'}
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="project-modal-backdrop" onClick={handleBackdropClick}>
      <TransferBanner progress={transferProgress} onDismiss={handleDismissTransfer} />
      <div className="project-modal-content">
        <button className="project-modal-close" onClick={onClose} disabled={loading}>
          &times;
        </button>

        <h2>Project Management</h2>

        <div className="project-tabs">
          <button
            className={`tab-btn ${mode === 'existing' ? 'active' : ''}`}
            onClick={() => setMode('existing')}
            disabled={loading || existingProjects.length === 0}
          >
            Existing Projects
          </button>
          <button
            className={`tab-btn ${mode === 'create' ? 'active' : ''}`}
            onClick={() => setMode('create')}
            disabled={loading}
          >
            Create Project
          </button>
          <button
            className={`tab-btn ${mode === 'add' ? 'active' : ''}`}
            onClick={() => setMode('add')}
            disabled={loading || existingProjects.length === 0}
          >
            Add to Project
          </button>
        </div>

        {mode === 'existing' && (
          <div className="project-form">
            {existingProjects.length === 0 ? (
              <p className="no-projects-message">No projects yet. Create one first!</p>
            ) : (
              <>
                <label>Select Project:</label>
                <select
                  value={selectedProject}
                  onChange={(e) => setSelectedProject(e.target.value)}
                  disabled={loading}
                  className="project-select"
                >
                  {existingProjects.map((project) => (
                    <option key={project.projectId} value={project.projectId}>
                      {project.name}
                    </option>
                  ))}
                </select>
                {currentProject && renderProjectDetails(currentProject)}
              </>
            )}
          </div>
        )}

        {mode === 'create' && (
          <div className="project-form">
            <label>Project Name:</label>
            <input
              type="text"
              value={projectName}
              onChange={(e) => setProjectName(e.target.value)}
              placeholder="Enter project name..."
              disabled={loading}
              onKeyDown={(e) => e.key === 'Enter' && handleCreateProject()}
            />
            <button
              className="project-btn primary"
              onClick={handleCreateProject}
              disabled={loading || !projectName.trim()}
            >
              {loading ? 'Creating...' : 'Create Project'}
            </button>
          </div>
        )}

        {mode === 'add' && (
          <div className="project-form">
            <label>Select Project:</label>
            <select
              value={selectedProject}
              onChange={(e) => setSelectedProject(e.target.value)}
              disabled={loading}
            >
              {existingProjects.map((project) => (
                <option key={project.projectId} value={project.projectId}>
                  {project.name} ({project.imageCount} images)
                </option>
              ))}
            </select>

            <label>Images to Add:</label>
            <div className="image-filter-buttons">
              <div className="filter-buttons-row">
                <button
                  type="button"
                  className={`filter-btn filter-all ${imageFilter === 'all' ? 'active' : ''}`}
                  onClick={() => setImageFilter('all')}
                  disabled={loading}
                >
                  All
                </button>
              </div>
              <div className="filter-buttons-row">
                {GROUP_COLORS.map((group) => (
                  <button
                    key={group.number}
                    type="button"
                    className={`filter-btn ${imageFilter === group.number ? 'active' : ''}`}
                    style={{ backgroundColor: group.color }}
                    onClick={() => setImageFilter(group.number)}
                    disabled={loading}
                    title={group.name}
                  >
                    {group.number}
                  </button>
                ))}
              </div>
            </div>

            <button
              className="project-btn primary"
              onClick={handleAddToProject}
              disabled={loading || !selectedProject}
            >
              {loading ? 'Adding...' : 'Add Images to Project'}
            </button>
          </div>
        )}

        {result && (
          <div className={`project-result ${result.success ? 'success' : 'error'}`}>
            {result.message}
          </div>
        )}
      </div>
    </div>
  );
};
