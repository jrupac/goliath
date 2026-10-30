import React from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Route, Routes } from 'react-router';
import { registerSW } from 'virtual:pwa-register';
import { markUpdatePending, setUpdateApplier } from './utils/swUpdate';
import App from './App';
import Login from './components/Login';

// How often a tab asks whether a newer build has been deployed. The browser
// only re-checks the worker script on navigation and roughly once a day, and
// these tabs stay open for far longer than that, so without a check of their
// own a release would go unnoticed until the page was reloaded -- which is the
// very thing the offer exists to make unnecessary.
const UPDATE_CHECK_INTERVAL_MS = 60 * 60 * 1000;

// How long to wait for the waiting worker to take over before reloading
// anyway.
const HANDOVER_TIMEOUT_MS = 3000;

// Registering through the virtual module rather than letting the plugin inject
// its own script is what gives the page a say in when a new release takes
// effect. The injected script only calls navigator.serviceWorker.register and
// stops there.
//
// The new worker installs and waits; the page it would replace is left alone
// until the reader asks for it. Paired with `registerType: 'prompt'` in
// vite.config.js.
const updateSW = registerSW({
  immediate: true,
  onNeedRefresh: markUpdatePending,
  // A worker taking control is not by itself a reason to reload. Every open
  // tab sees that happen when any one of them accepts the update, and a tab
  // sitting on an older build is usually there on purpose. The tab whose
  // button was pressed reloads itself below; the rest stay as they are.
  onNeedReload: () => {},
  onRegisteredSW: (
    _swUrl: string,
    registration?: ServiceWorkerRegistration
  ) => {
    if (!registration) {
      return;
    }
    setInterval(() => {
      // A hidden or offline tab would only be asking the cache, or failing.
      if (document.visibilityState !== 'visible' || !navigator.onLine) {
        return;
      }
      registration.update().catch((e: unknown) => {
        console.log('Update check failed: ' + e);
      });
    }, UPDATE_CHECK_INTERVAL_MS);
  },
});

setUpdateApplier((): void => {
  // Reload once the waiting worker has taken over, so that the page comes back
  // served by the new build rather than racing the handover and being served
  // the old one again. The timeout is the backstop: a reload button that does
  // nothing is worse than one that reloads a moment early.
  let reloaded = false;
  const reload = (): void => {
    if (reloaded) {
      return;
    }
    reloaded = true;
    window.location.reload();
  };

  navigator.serviceWorker?.addEventListener('controllerchange', reload, {
    once: true,
  });
  setTimeout(reload, HANDOVER_TIMEOUT_MS);

  updateSW(true);
});

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
