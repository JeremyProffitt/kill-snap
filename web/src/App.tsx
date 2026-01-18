import React, { useState } from 'react';
import { authService } from './services/auth';
import { Login } from './components/Login';
import { ImageGallery } from './components/ImageGallery';
import { LogsViewer } from './components/LogsViewer';
import './App.css';

type ActiveView = 'images' | 'logs';

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(authService.isAuthenticated());
  const [activeView, setActiveView] = useState<ActiveView>('images');

  const handleLoginSuccess = () => {
    setIsAuthenticated(true);
  };

  const handleLogout = () => {
    authService.logout();
    setIsAuthenticated(false);
  };

  if (!isAuthenticated) {
    return (
      <div className="App">
        <Login onLoginSuccess={handleLoginSuccess} />
      </div>
    );
  }

  return (
    <div className="App">
      <div className="app-nav">
        <button
          className={`app-nav-btn ${activeView === 'images' ? 'active' : ''}`}
          onClick={() => setActiveView('images')}
        >
          Image Review
        </button>
        <button
          className={`app-nav-btn ${activeView === 'logs' ? 'active' : ''}`}
          onClick={() => setActiveView('logs')}
        >
          Logs
        </button>
      </div>
      {activeView === 'images' ? (
        <ImageGallery onLogout={handleLogout} />
      ) : (
        <LogsViewer onLogout={handleLogout} />
      )}
    </div>
  );
}

export default App;
