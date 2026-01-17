// Undo manager service - manages undo/redo stack for image actions
import { Image } from '../types';

export interface UndoAction {
  id: string;
  type: 'update' | 'delete' | 'bulk';
  description: string;
  imageGUIDs: string[];
  previousState: Partial<Image>[];
  timestamp: number;
}

type UndoListener = (action: UndoAction | null) => void;

class UndoManager {
  private stack: UndoAction[] = [];
  private maxSize = 20;
  private listeners: UndoListener[] = [];

  push(action: UndoAction): void {
    this.stack.push(action);
    if (this.stack.length > this.maxSize) {
      this.stack.shift();
    }
    this.notify();
  }

  pop(): UndoAction | null {
    const action = this.stack.pop() || null;
    this.notify();
    return action;
  }

  peek(): UndoAction | null {
    return this.stack[this.stack.length - 1] || null;
  }

  canUndo(): boolean {
    return this.stack.length > 0;
  }

  getStack(): UndoAction[] {
    return [...this.stack];
  }

  clear(): void {
    this.stack = [];
    this.notify();
  }

  subscribe(listener: UndoListener): () => void {
    this.listeners.push(listener);
    return () => {
      this.listeners = this.listeners.filter(l => l !== listener);
    };
  }

  private notify(): void {
    this.listeners.forEach(l => l(this.peek()));
  }
}

export const undoManager = new UndoManager();

// Helper to generate unique IDs
export const generateUndoId = (): string => {
  return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
};
