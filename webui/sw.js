// NAS-OS Service Worker v2.0
// Enhanced PWA with Offline Support

const CACHE_NAME = 'nas-os-v3.24.6';
const RUNTIME_CACHE = 'nas-os-runtime-v3.24.6';

// Static assets to cache on install
const STATIC_ASSETS = [
    '/',
    '/index.html',
    '/css/design-system.css',
    '/css/mobile.css',
    '/js/app.js',
    '/js/vendor/echarts.min.js',
    '/manifest.json'
];

// API endpoints to cache (network-first)
const API_CACHE_PATTERNS = [
    /\/api\/v1\/system\/info/,
    /\/api\/v1\/storage\/volumes$/,
    /\/api\/v1\/system\/stats/
];

// ============================================
// Install Event - Cache Static Assets
// ============================================
self.addEventListener('install', (event) => {
    console.log('[SW] Installing Service Worker v2');
    event.waitUntil(
        caches.open(CACHE_NAME)
            .then((cache) => {
                console.log('[SW] Caching static assets');
                return cache.addAll(STATIC_ASSETS);
            })
            .then(() => self.skipWaiting())
            .catch((error) => {
                console.error('[SW] Cache failed:', error);
            })
    );
});

// ============================================
// Activate Event - Clean Up Old Caches
// ============================================
self.addEventListener('activate', (event) => {
    console.log('[SW] Activating Service Worker v2');
    event.waitUntil(
        caches.keys()
            .then((cacheNames) => {
                return Promise.all(
                    cacheNames
                        .filter((name) => {
                            // Delete old version caches
                            return name.startsWith('nas-os-') && 
                                   name !== CACHE_NAME && 
                                   name !== RUNTIME_CACHE;
                        })
                        .map((name) => {
                            console.log('[SW] Deleting old cache:', name);
                            return caches.delete(name);
                        })
                );
            })
            .then(() => {
                console.log('[SW] Service Worker activated');
                return self.clients.claim();
            })
    );
});

// ============================================
// Fetch Event - Network First with Cache Fallback
// ============================================
self.addEventListener('fetch', (event) => {
    const { request } = event;
    const url = new URL(request.url);

    // Skip non-GET requests
    if (request.method !== 'GET') {
        return;
    }

    // Skip cross-origin requests
    if (url.origin !== location.origin) {
        return;
    }

    // API requests - Network First with Cache Fallback
    if (url.pathname.startsWith('/api/')) {
        event.respondWith(networkFirstWithCache(request));
        return;
    }

    // Static assets - Cache First with Network Update
    if (isStaticAsset(url.pathname)) {
        event.respondWith(cacheFirstWithNetworkUpdate(request));
        return;
    }

    // HTML pages - Network First with Cache Fallback
    if (request.headers.get('accept')?.includes('text/html')) {
        event.respondWith(networkFirstWithCache(request, '/index.html'));
        return;
    }

    // Default: Network First
    event.respondWith(networkFirstWithCache(request));
});

// ============================================
// Caching Strategies
// ============================================

// Network First with Cache Fallback
async function networkFirstWithCache(request, fallbackUrl = null) {
    const cache = await caches.open(RUNTIME_CACHE);
    
    try {
        const networkResponse = await fetch(request);
        
        // Cache successful responses
        if (networkResponse.ok) {
            const responseClone = networkResponse.clone();
            
            // Only cache API responses that match patterns
            const url = new URL(request.url);
            if (API_CACHE_PATTERNS.some(pattern => pattern.test(url.pathname))) {
                cache.put(request, responseClone);
            }
        }
        
        return networkResponse;
    } catch (error) {
        console.log('[SW] Network request failed, trying cache:', request.url);
        
        // Try cache
        const cachedResponse = await cache.match(request);
        if (cachedResponse) {
            return cachedResponse;
        }
        
        // Return fallback for HTML requests
        if (fallbackUrl && request.headers.get('accept')?.includes('text/html')) {
            const fallbackResponse = await cache.match(fallbackUrl);
            if (fallbackResponse) {
                return fallbackResponse;
            }
        }
        
        // Return offline response
        return new Response(
            JSON.stringify({ 
                error: 'offline', 
                message: '您当前处于离线状态' 
            }),
            {
                status: 503,
                statusText: 'Service Unavailable',
                headers: { 'Content-Type': 'application/json' }
            }
        );
    }
}

// Cache First with Network Update (stale-while-revalidate)
async function cacheFirstWithNetworkUpdate(request) {
    const cache = await caches.open(CACHE_NAME);
    
    // Try cache first
    const cachedResponse = await cache.match(request);
    
    // Start network fetch in background
    const networkPromise = fetch(request)
        .then((networkResponse) => {
            if (networkResponse.ok) {
                cache.put(request, networkResponse.clone());
            }
            return networkResponse;
        })
        .catch(() => null);
    
    // Return cached or wait for network
    if (cachedResponse) {
        return cachedResponse;
    }
    
    return networkPromise.then(response => {
        if (response) return response;
        return new Response('Offline', { status: 503 });
    });
}

function isStaticAsset(pathname) {
    return /\.(css|js|png|jpg|jpeg|gif|svg|ico|woff|woff2|ttf|eot)$/.test(pathname) ||
           pathname === '/' ||
           pathname === '/index.html';
}

// ============================================
// Message Handling
// ============================================
self.addEventListener('message', (event) => {
    console.log('[SW] Message received:', event.data);
    
    if (event.data && event.data.type === 'SKIP_WAITING') {
        self.skipWaiting();
    }
    
    if (event.data && event.data.type === 'GET_VERSION') {
        event.ports[0].postMessage({ version: CACHE_NAME });
    }
    
    if (event.data && event.data.type === 'CACHE_URLS') {
        event.waitUntil(
            caches.open(CACHE_NAME)
                .then((cache) => cache.addAll(event.data.urls))
        );
    }
});

// ============================================
// Push Notifications (Web Push / VAPID)
// ============================================
self.addEventListener('push', (event) => {
    let data = {};
    try {
        data = event.data ? event.data.json() : {};
    } catch (e) {
        data = { body: event.data ? event.data.text() : '' };
    }
    const title = data.title || 'NAS-OS';
    const options = {
        body: data.body || '',
        icon: data.icon || '/brand/logo/logo-192.png',
        badge: data.badge || '/brand/logo/logo-72.png',
        tag: data.tag || 'nas-os',
        data: { url: data.url || '/' }
    };
    event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (event) => {
    event.notification.close();
    const url = (event.notification.data && event.notification.data.url) || '/';
    event.waitUntil(
        clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
            for (const client of clientList) {
                if ('focus' in client && client.url && client.url.includes(url)) {
                    return client.focus();
                }
            }
            for (const client of clientList) {
                if ('focus' in client && 'navigate' in client) {
                    return client.navigate(url).then(() => client.focus());
                }
            }
            return clients.openWindow(url);
        })
    );
});
