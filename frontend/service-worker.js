const CACHE_NAME = 'coalguard-shell-v1';
const DATA_CACHE_NAME = 'coalguard-data-v1';

// Static assets to cache for offline availability
const FILES_TO_CACHE = [
  '/',
  '/index.html',
  '/login.html',
  '/dashboard.html',
  '/inspections.html',
  '/mines.html',
  '/attendance.html',
  '/grievances.html',
  '/contractors.html',
  '/environmental.html',
  '/production.html',
  '/compliance.html',
  '/violations.html',
  '/corrective-actions.html',
  '/incidents.html',
  '/documents.html',
  '/analytics.html',
  '/reports.html',
  '/notifications.html',
  '/simulation-control.html',
  '/audit-logs.html',
  '/users.html',
  '/css/dashboard.css',
  '/css/forms.css',
  '/css/responsive.css',
  '/css/style.css',
  '/js/analytics.js',
  '/js/api.js',
  '/js/attendance.js',
  '/js/auth.js',
  '/js/compliance.js',
  '/js/config.js',
  '/js/contractors.js',
  '/js/corrective-actions.js',
  '/js/dashboard.js',
  '/js/documents.js',
  '/js/environmental.js',
  '/js/grievances.js',
  '/js/incidents.js',
  '/js/inspections.js',
  '/js/mines.js',
  '/js/notifications-page.js',
  '/js/notifications.js',
  '/js/production.js',
  '/js/pwa.js',
  '/js/reports.js',
  '/js/simulation-control.js',
  '/js/audit-logs.js',
  '/js/users.js',
  '/js/violations.js',
  '/js/voice-assistant.js',
  '/icons/icon-192x192.png',
  '/icons/icon-512x512.png',
  'https://cdn.jsdelivr.net/npm/chart.js',
  'https://unpkg.com/leaflet@1.9.4/dist/leaflet.css',
  'https://unpkg.com/leaflet@1.9.4/dist/leaflet.js'
];

self.addEventListener('install', (evt) => {
  evt.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      console.log('[ServiceWorker] Pre-caching offline page');
      return cache.addAll(FILES_TO_CACHE);
    })
  );
  self.skipWaiting();
});

self.addEventListener('activate', (evt) => {
  evt.waitUntil(
    caches.keys().then((keyList) => {
      return Promise.all(keyList.map((key) => {
        if (key !== CACHE_NAME && key !== DATA_CACHE_NAME) {
          console.log('[ServiceWorker] Removing old cache', key);
          return caches.delete(key);
        }
      }));
    })
  );
  self.clients.claim();
});

self.addEventListener('fetch', (evt) => {
  const url = new URL(evt.request.url);

  // Cache API GET requests for mines and compliance rules (Stale-While-Revalidate)
  if (evt.request.method === 'GET' && url.pathname.includes('/api/')) {
    if (url.pathname.endsWith('/mines') || url.pathname.endsWith('/compliance/rules') || url.pathname.endsWith('/compliance/categories')) {
      evt.respondWith(
        caches.open(DATA_CACHE_NAME).then((cache) => {
          return cache.match(evt.request).then((response) => {
            const fetchPromise = fetch(evt.request).then((networkResponse) => {
              if (networkResponse.ok) {
                cache.put(evt.request, networkResponse.clone());
              }
              return networkResponse;
            }).catch(() => {
              // Ignore fetch error, we might be offline
            });
            // Return cached response immediately if available, otherwise wait for network
            return response || fetchPromise;
          });
        })
      );
      return;
    }
  }

  // Fallback to network first, then cache for other requests
  if (evt.request.mode !== 'navigate') {
    evt.respondWith(
      caches.match(evt.request).then((response) => {
        return response || fetch(evt.request).catch(() => {
          return caches.match('/index.html');
        });
      })
    );
    return;
  }

  evt.respondWith(
    fetch(evt.request).catch(() => {
      return caches.match('/index.html');
    })
  );
});
