import React, { useState, useEffect } from 'react';
import { api } from '../services/api';
import { Project } from '../types';
import './ProjectModal.css';

interface ProjectModalProps {
  onClose: () => void;
  onProjectCreated: () => void;
  existingProjects: Project[];
}

const GROUP_COLORS = [
  { number: 1, color: '#e74c3c', name: 'Red' },
  { number: 2, color: '#3498db', name: 'Blue' },
  { number: 3, color: '#2ecc71', name: 'Green' },
  { number: 4, color: '#f1c40f', name: 'Yellow' },
  { number: 5, color: '#9b59b6', name: 'Purple' },
  { number: 6, color: '#e67e22', name: 'Orange' },
  { number: 7, color: '#e91e63', name: 'Pink' },
  { number: 8, color: '#795548', name: 'Brown' },
];

export const ProjectModal: React.FC<ProjectModalProps> = ({
  onClose,
  onProjectCreated,
  existingProjects,
}) => {
  const [mode, setMode] = useState<'create' | 'add'>('create');
  const [projectName, setProjectName] = useState('');
  const [selectedProject, setSelectedProject] = useState<string>('');
  const [imageFilter, setImageFilter] = useState<'all' | number>('all');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ success: boolean; message: string } | null>(null);

  useEffect(() => {
    if (existingProjects.length > 0) {
      setSelectedProject(existingProjects[0].projectId);
    }
  }, [existingProjects]);

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

    setLoading(true);
    setResult(null);
    try {
      const filters = imageFilter === 'all'
        ? { all: true }
        : { group: imageFilter };

      const response = await api.addToProject(selectedProject, filters);
      const projectName = existingProjects.find(p => p.projectId === selectedProject)?.name || 'project';
      setResult({
        success: true,
        message: `Moved ${response.movedCount} image(s) to "${projectName}"`
      });
      onProjectCreated();
    } catch (err) {
      console.error('Failed to add images to project:', err);
      setResult({ success: false, message: 'Failed to add images to project' });
    } finally {
      setLoading(false);
    }
  };

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  return (
    <div className="project-modal-backdrop" onClick={handleBackdropClick}>
      <div className="project-modal-content">
        <button className="project-modal-close" onClick={onClose} disabled={loading}>
          &times;
        </button>

        <h2>Project Management</h2>

        <div className="project-tabs">
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

        {mode === 'create' ? (
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
        ) : (
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
            <select
              value={imageFilter}
              onChange={(e) => setImageFilter(e.target.value === 'all' ? 'all' : parseInt(e.target.value))}
              disabled={loading}
            >
              <option value="all">All Approved Images</option>
              {GROUP_COLORS.map((group) => (
                <option key={group.number} value={group.number}>
                  Group {group.number}: {group.name}
                </option>
              ))}
            </select>

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

        {existingProjects.length > 0 && (
          <div className="existing-projects">
            <h3>Existing Projects</h3>
            <ul>
              {existingProjects.map((project) => (
                <li key={project.projectId}>
                  <span className="project-name">{project.name}</span>
                  <span className="project-count">{project.imageCount} images</span>
                  <span className="project-date">
                    Created: {new Date(project.createdAt).toLocaleDateString()}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </div>
  );
};
