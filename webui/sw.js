// NAS-OS Service Worker v2.0
// Enhanced PWA with Offline Support, Background Sync, and Push Notifications

const CACHE_NAME = 'nas-os-v2';
const RUNTIME_CACHE = 'nas-os-runtime-v2';

// Static assets to cache on install
const STATIC_ASSETS = [
    '/',
    '/index.html',
    '/css/design-system.css',
    '/css/mobile.css',
    '/js/app.js',
    '/manifest.json'
];

// API endpoints to cache (network-first)
const API_CACHE_PATTERNS = [
    /\/api\/v1\/system\/info/,
    /\/api\/v1\/volumes$/,
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
// Background Sync
// ============================================
self.addEventListener('sync', (event) => {
    console.log('[SW] Background sync:', event.tag);
    
    if (event.tag === 'sync-data') {
        event.waitUntil(syncOfflineData());
    }
    
    if (event.tag === 'sync-settings') {
        event.waitUntil(syncSettings());
    }
});

async function syncOfflineData() {
    try {
        // Get offline data from IndexedDB
        const offlineData = await getOfflineData();
        
        if (offlineData && offlineData.length > 0) {
            for (const item of offlineData) {
                await fetch(item.url, {
                    method: item.method,
                    headers: item.headers,
                    body: JSON.stringify(item.body)
                });
            }
            
            // Clear synced data
            await clearOfflineData();
            
            // Notify clients
            const clients = await self.clients.matchAll();
            clients.forEach(client => {
                client.postMessage({
                    type: 'SYNC_COMPLETE',
                    message: '离线数据已同步'
                });
            });
        }
    } catch (error) {
        console.error('[SW] Sync failed:', error);
    }
}

async function syncSettings() {
    // Sync user settings
    console.log('[SW] Syncing settings...');
}

// Simple IndexedDB helpers (for offline data)
function openDatabase() {
    return new Promise((resolve, reject) => {
        const request = indexedDB.open('nas-os-offline', 1);
        request.onerror = () => reject(request.error);
        request.onsuccess = () => resolve(request.result);
        request.onupgradeneeded = (event) => {
            const db = event.target.result;
            if (!db.objectStoreNames.contains('pending')) {
                db.createObjectStore('pending', { keyPath: 'id', autoIncrement: true });
            }
        };
    });
}

async function getOfflineData() {
    const db = await openDatabase();
    return new Promise((resolve, reject) => {
        const transaction = db.transaction('pending', 'readonly');
        const store = transaction.objectStore('pending');
        const request = store.getAll();
        request.onerror = () => reject(request.error);
        request.onsuccess = () => resolve(request.result);
    });
}

async function clearOfflineData() {
    const db = await openDatabase();
    return new Promise((resolve, reject) => {
        const transaction = db.transaction('pending', 'readwrite');
        const store = transaction.objectStore('pending');
        const request = store.clear();
        request.onerror = () => reject(request.error);
        request.onsuccess = () => resolve();
    });
}

// ============================================
// Push Notifications
// ============================================
self.addEventListener('push', (event) => {
    console.log('[SW] Push received');
    
    let data = {};
    try {
        data = event.data?.json() || {};
    } catch (e) {
        data = { title: 'NAS-OS', body: event.data?.text() || '新通知' };
    }
    
    const title = data.title || 'NAS-OS 通知';
    const options = {
        body: data.body || data.message || '有新消息',
        icon: data.icon || '/brand/logo/logo-192.png',
        badge: '/brand/logo/logo-72.png',
        tag: data.tag || 'default',
        data: {
            url: data.url || '/',
            id: data.id
        },
        actions: data.actions || [],
        vibrate: [100, 50, 100],
        requireInteraction: data.requireInteraction || false
    };
    
    event.waitUntil(
        self.registration.showNotification(title, options)
    );
});

// Notification Click Handler
self.addEventListener('notificationclick', (event) => {
    console.log('[SW] Notification clicked:', event.notification.tag);
    event.notification.close();
    
    const urlToOpen = event.notification.data?.url || '/';
    
    event.waitUntil(
        self.clients.matchAll({ type: 'window', includeUncontrolled: true })
            .then((clientList) => {
                // Check if a window is already open
                for (const client of clientList) {
                    if (client.url.includes(self.location.origin) && 'focus' in client) {
                        client.postMessage({
                            type: 'NOTIFICATION_CLICK',
                            data: event.notification.data
                        });
                        return client.focus();
                    }
                }
                
                // Open new window
                if (self.clients.openWindow) {
                    return self.clients.openWindow(urlToOpen);
                }
            })
    );
});

// Notification Close Handler
self.addEventListener('notificationclose', (event) => {
    console.log('[SW] Notification closed:', event.notification.tag);
    
    // Track notification dismissal if needed
    event.waitUntil(
        fetch('/api/v1/notifications/dismiss', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tag: event.notification.tag })
        }).catch(() => {})
    );
});

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
// Periodic Background Sync (if supported)
// ============================================
self.addEventListener('periodicsync', (event) => {
    console.log('[SW] Periodic sync:', event.tag);
    
    if (event.tag === 'check-updates') {
        event.waitUntil(checkForUpdates());
    }
});

async function checkForUpdates() {
    try {
        const response = await fetch('/api/v1/system/updates');
        const data = await response.json();
        
        if (data.hasUpdates) {
            await self.registration.showNotification('NAS-OS 更新', {
                body: `有 ${data.updateCount} 个更新可用`,
                icon: '/brand/logo/logo-192.png',
                tag: 'system-updates'
            });
        }
    } catch (error) {
        console.error('[SW] Check updates failed:', error);
    }
}

// ============================================
// Connection Status
// ============================================
self.addEventListener('online', () => {
    console.log('[SW] Back online');
    // Trigger background sync
    self.registration.sync.register('sync-data').catch(() => {});
});

self.addEventListener('offline', () => {
    console.log('[SW] Gone offline');
});