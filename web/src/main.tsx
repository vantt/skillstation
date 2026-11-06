import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { App } from './App';
import { captureTokenFromHash } from './state/session';
import { initAppearance } from './state/appearance';
import './design-system/styles.css';
import './app.css';
import './design-system/fonts/precision';

captureTokenFromHash();
// Pasting a new server URL into an open tab only changes the hash, so the page is not reloaded:
// take the new token and reload to drop everything fetched with the old one.
window.addEventListener('hashchange', () => {
  if (captureTokenFromHash()) window.location.reload();
});
initAppearance();

document.body.classList.add('fg-root');

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

const container = document.getElementById('root');
if (container) {
  ReactDOM.createRoot(container).render(
    <React.StrictMode>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </QueryClientProvider>
    </React.StrictMode>,
  );
}
