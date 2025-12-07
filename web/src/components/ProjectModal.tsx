import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../services/api';
import { Project, ZipFile } from '../types';
import { TransferBanner, TransferProgress } from './TransferBanner';
import './ProjectModal.css';

interface ProjectModalProps {
  onClose: () => void;
  onProjectCreated: (newProjectId?: string) => void;
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
  const [selectedProject, setSelectedProject] = useState<string>('');
  const [imageFilter, setImageFilter] = useState<'all' | number>('all');
  const [loading, setLoading] = useState(false);
  const [generatingZip, setGeneratingZip] = useState<string | null>(null);
  const [downloadingZip, setDownloadingZip] = useState<string | null>(null);
  const [deletingZip, setDeletingZip] = useState<string | null>(null);
  const [zipErrors, setZipErrors] = useState<{ [projectId: string]: string[] }>({});
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [newProjectName, setNewProjectName] = useState('');
  const [creatingProject, setCreatingProject] = useState(false);
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
      setSelectedProject(prev => {
        if (!prev || !existingProjects.find(p => p.projectId === prev)) {
          return existingProjects[0].projectId;
        }
        return prev;
      });
    }
  }, [existingProjects]);

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
        onProjectCreated();
      }
    } catch (err) {
      console.error('Failed to check zip status:', err);
    }
  }, [onProjectCreated]);

  useEffect(() => {
    const projectsWithGeneratingZips = existingProjects.filter(
      p => p.zipFiles?.some(z => z.status === 'generating')
    );

    if (projectsWithGeneratingZips.length === 0) return;

    projectsWithGeneratingZips.forEach(checkZipStatus);

    const refreshInterval = setInterval(() => {
      onProjectCreated();
    }, 10000);

    const errorCheckInterval = setInterval(() => {
      projectsWithGeneratingZips.forEach(checkZipStatus);
    }, 30000);

    return () => {
      clearInterval(refreshInterval);
      clearInterval(errorCheckInterval);
    };
  }, [existingProjects, checkZipStatus, onProjectCreated]);

  const handleCreateProject = async () => {
    if (!newProjectName.trim()) return;

    setCreatingProject(true);
    try {
      const result = await api.createProject(newProjectName.trim());
      setNewProjectName('');
      setShowAddDialog(false);
      // Select the newly created project
      if (result?.projectId) {
        setSelectedProject(result.projectId);
        onProjectCreated(result.projectId);
      } else {
        onProjectCreated();
      }
    } catch (err) {
      console.error('Failed to create project:', err);
    } finally {
      setCreatingProject(false);
    }
  };

  const handleAddToProject = async () => {
    if (!selectedProject) return;

    const projectName = existingProjects.find(p => p.projectId === selectedProject)?.name || 'project';

    setLoading(true);
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

      onProjectCreated();
    } catch (err) {
      console.error('Failed to add images to project:', err);
      setTransferProgress(prev => ({
        ...prev,
        status: 'error',
        errorMessage: 'Failed to add images to project',
      }));
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

  const handleGenerateZip = async (project: Project) => {
    setZipErrors(prev => {
      const newErrors = { ...prev };
      delete newErrors[project.projectId];
      return newErrors;
    });

    setGeneratingZip(project.projectId);
    try {
      await api.generateZip(project.projectId);
      onProjectCreated();
    } catch (err: any) {
      console.error('Failed to generate zip:', err);
    } finally {
      setGeneratingZip(null);
    }
  };

  const handleDownloadZip = async (project: Project, zipFile: ZipFile) => {
    const zipId = `${project.projectId}-${zipFile.key}`;
    setDownloadingZip(zipId);
    try {
      const response = await api.getZipDownload(project.projectId, zipFile.key);
      const link = document.createElement('a');
      link.href = response.url;
      link.download = response.filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    } catch (err: any) {
      console.error('Failed to download zip:', err);
    } finally {
      setDownloadingZip(null);
    }
  };

  const handleDeleteZip = async (project: Project, zipFile: ZipFile) => {
    const zipId = `${project.projectId}-${zipFile.key}`;
    setDeletingZip(zipId);
    try {
      await api.deleteZip(project.projectId, zipFile.key);
      await onProjectCreated();
    } catch (err: any) {
      console.error('Failed to delete zip:', err);
    } finally {
      setDeletingZip(null);
    }
  };

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  const handleAddDialogBackdrop = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      setShowAddDialog(false);
      setNewProjectName('');
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
        <div className="project-actions">
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
                  <div className="zip-file-info">
                    <button
                      type="button"
                      className="zip-file-link"
                      onClick={() => {
                        if (downloadingZip !== zipId && deletingZip !== zipId) {
                          handleDownloadZip(project, zipFile);
                        }
                      }}
                      disabled={downloadingZip === zipId || deletingZip === zipId}
                    >
                      {downloadingZip === zipId ? 'Downloading...' : filename}
                    </button>
                    <span className="zip-file-meta">
                      {formatFileSize(zipFile.size)}, {zipFile.imageCount} images
                    </span>
                  </div>
                  <button
                    type="button"
                    className="delete-zip-btn"
                    onClick={() => handleDeleteZip(project, zipFile)}
                    disabled={downloadingZip === zipId || deletingZip === zipId}
                    title="Delete zip file"
                  >
                    {deletingZip === zipId ? 'Deleting...' : 'Delete'}
                  </button>
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

        <div className="project-form">
          {existingProjects.length === 0 ? (
            <div className="no-projects-section">
              <p className="no-projects-message">No projects yet.</p>
              <button
                className="project-btn primary"
                onClick={() => setShowAddDialog(true)}
              >
                Create Project
              </button>
            </div>
          ) : (
            <>
              <div className="project-select-row">
                <label>Select Project:</label>
                <div className="select-with-button">
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
                  <button
                    type="button"
                    className="add-project-btn"
                    onClick={() => setShowAddDialog(true)}
                    disabled={loading}
                    title="Add new project"
                  >
                    + Add
                  </button>
                </div>
                {currentProject && (
                  <div className="project-info-row">
                    <span className="project-date">
                      Created: {new Date(currentProject.createdAt).toLocaleDateString()}
                    </span>
                    <span className="project-count">{currentProject.imageCount} images</span>
                  </div>
                )}
              </div>

              {currentProject && renderProjectDetails(currentProject)}

              <div className="add-images-section">
                <div className="criteria-header">
                  <span className="criteria-label">Select Image Criteria</span>
                </div>
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
            </>
          )}
        </div>
      </div>

      {/* Add Project Dialog */}
      {showAddDialog && (
        <div className="add-dialog-backdrop" onClick={handleAddDialogBackdrop}>
          <div className="add-dialog">
            <h3>Create New Project</h3>
            <input
              type="text"
              value={newProjectName}
              onChange={(e) => setNewProjectName(e.target.value)}
              placeholder="Enter project name..."
              disabled={creatingProject}
              onKeyDown={(e) => e.key === 'Enter' && handleCreateProject()}
              autoFocus
            />
            <div className="add-dialog-buttons">
              <button
                type="button"
                className="dialog-btn cancel"
                onClick={() => {
                  setShowAddDialog(false);
                  setNewProjectName('');
                }}
                disabled={creatingProject}
              >
                Cancel
              </button>
              <button
                type="button"
                className="dialog-btn create"
                onClick={handleCreateProject}
                disabled={creatingProject || !newProjectName.trim()}
              >
                {creatingProject ? 'Creating...' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
