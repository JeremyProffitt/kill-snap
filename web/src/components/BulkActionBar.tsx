import React from 'react';
import './BulkActionBar.css';

interface BulkActionBarProps {
  selectedCount: number;
  totalCount: number;
  onApprove: (groupNumber: number) => void;
  onReject: () => void;
  onDelete: () => void;
  onAddToProject: () => void;
  onClearSelection: () => void;
  onSelectAll: () => void;
  disabled?: boolean;
}

// Lightroom color labels
const GROUP_COLORS = [
  { number: 1, color: '#e74c3c', name: 'Red' },
  { number: 2, color: '#f1c40f', name: 'Yellow' },
  { number: 3, color: '#2ecc71', name: 'Green' },
  { number: 4, color: '#3498db', name: 'Blue' },
  { number: 5, color: '#9b59b6', name: 'Purple' },
];

export const BulkActionBar: React.FC<BulkActionBarProps> = ({
  selectedCount,
  totalCount,
  onApprove,
  onReject,
  onDelete,
  onAddToProject,
  onClearSelection,
  onSelectAll,
  disabled = false,
}) => {
  if (selectedCount === 0) return null;

  return (
    <div className="bulk-action-bar">
      <div className="bulk-selection-info">
        <span className="selection-count">{selectedCount} selected</span>
        <button
          className="bulk-select-all-btn"
          onClick={onSelectAll}
          disabled={disabled || selectedCount === totalCount}
        >
          Select All ({totalCount})
        </button>
      </div>

      <div className="bulk-divider" />

      <div className="bulk-color-buttons">
        {GROUP_COLORS.map((group) => (
          <button
            key={group.number}
            className="bulk-color-btn"
            style={{ backgroundColor: group.color }}
            onClick={() => onApprove(group.number)}
            disabled={disabled}
            title={`Approve as ${group.name}`}
          >
            {group.number}
          </button>
        ))}
      </div>

      <div className="bulk-divider" />

      <div className="bulk-action-buttons">
        <button
          className="bulk-btn approve"
          onClick={() => onApprove(1)}
          disabled={disabled}
          title="Approve selected"
        >
          Approve
        </button>
        <button
          className="bulk-btn reject"
          onClick={onReject}
          disabled={disabled}
          title="Reject selected"
        >
          Reject
        </button>
        <button
          className="bulk-btn delete"
          onClick={onDelete}
          disabled={disabled}
          title="Delete selected"
        >
          Delete
        </button>
        <button
          className="bulk-btn project"
          onClick={onAddToProject}
          disabled={disabled}
          title="Add to project"
        >
          Add to Project
        </button>
      </div>

      <div className="bulk-divider" />

      <button
        className="bulk-clear-btn"
        onClick={onClearSelection}
        disabled={disabled}
        title="Clear selection (Esc)"
      >
        Clear
      </button>
    </div>
  );
};
