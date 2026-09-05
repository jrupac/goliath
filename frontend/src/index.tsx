import React from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { registerSW } from 'virtual:pwa-register';
import App from './App';
import Login from './components/Login';

// Registering through the virtual module rather than letting the plugin inject
// its own script is what makes a new release take effect on the first reload.
// The injected script only calls navigator.serviceWorker.register and stops
// there, so a deploy went: reload serves the old precached bundle while the new
// worker installs and claims the page in the background, and only the *next*
// reload showed the new version. This registration installs the plugin's
// `activated` handler, which reloads the page once the updated worker takes
// over, so the second manual refresh is no longer needed.
//
// Paired with `registerType: 'autoUpdate'` in vite.config.js. The reload fires
// only when a worker activates as an update, never on first install.
registerSW({ immediate: true });

const container = document.getElementById('root');
const root = createRoot(container!);

root.render(
  <BrowserRouter>
    <div className="App">
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/" element={<App />} />
      </Routes>
    </div>
  </BrowserRouter>
);
