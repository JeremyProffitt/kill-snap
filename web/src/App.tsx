import React, { useState } from 'react';
import { authService } from './services/auth';
import { Login } from './components/Login';
import { ImageGallery } from './components/ImageGallery';
import './App.css';

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(authService.isAuthenticated());

  const handleLoginSuccess = () => {
    setIsAuthenticated(true);
  };

  const handleLogout = () => {
    setIsAuthenticated(false);
  };

  return (
    <div className="App">
      {isAuthenticated ? (
        <ImageGallery onLogout={handleLogout} />
      ) : (
        <Login onLoginSuccess={handleLoginSuccess} />
      )}
    </div>
  );
}

export default App;
