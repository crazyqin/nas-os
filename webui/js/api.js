/**
 * NAS-OS WebUI shared API helper.
 * - Attaches Authorization Bearer from localStorage
 * - Auto-injects Bearer on any window.fetch to /api/v1 (covers legacy pages)
 * - Normalizes token storage keys used across pages
 *
 * CSRF: backend skips CSRF when Authorization: Bearer is present (see middleware).
 * Login is CSRF-exempt; use apiLogin() which does not require a token yet.
 */
(function (global) {
  'use strict';

  const API_BASE = '/api/v1';
  const TOKEN_KEYS = ['token', 'nas_token', 'nasos_token', 'auth_token'];

  function getAuthToken() {
    for (const key of TOKEN_KEYS) {
      const raw = localStorage.getItem(key);
      if (!raw) continue;
      const t = String(raw).trim();
      if (!t) continue;
      return t.startsWith('Bearer ') || t.startsWith('bearer ') ? t : 'Bearer ' + t;
    }
    return '';
  }

  function setAuthToken(token) {
    if (!token) return;
    const bare = String(token).replace(/^Bearer\s+/i, '').trim();
    if (!bare) return;
    localStorage.setItem('nasos_token', bare);
    localStorage.setItem('token', 'Bearer ' + bare);
    localStorage.setItem('nas_token', bare);
  }

  function clearAuthToken() {
    TOKEN_KEYS.forEach((k) => localStorage.removeItem(k));
  }

  function isAPIURL(url) {
    if (!url) return false;
    const s = String(url);
    if (s.indexOf('/api/v1') !== -1) return true;
    // relative api paths used by some pages
    if (s.indexOf('/api/') === 0) return true;
    return false;
  }

  /**
   * Install once: every fetch to /api/* gets Authorization when available.
   * Opt out per-request with header X-Nas-No-Auth: 1
   */
  function installFetchInterceptor() {
    if (global.__nasFetchPatched || typeof global.fetch !== 'function') return;
    const rawFetch = global.fetch.bind(global);
    global.fetch = function (input, init) {
      init = init || {};
      let url = '';
      if (typeof input === 'string') url = input;
      else if (input && typeof input.url === 'string') url = input.url;

      if (isAPIURL(url)) {
        const headers = new Headers(init.headers || (input && input.headers) || {});
        if (!headers.has('X-Nas-No-Auth')) {
          const token = getAuthToken();
          if (token) {
            // Normalize double "Bearer Bearer …" and bare tokens from legacy pages.
            let auth = headers.get('Authorization') || token;
            auth = String(auth).replace(/^(Bearer\s+)+/i, 'Bearer ').trim();
            if (!/^Bearer\s+\S+/i.test(auth)) {
              auth = token;
            }
            headers.set('Authorization', auth);
          }
        } else {
          headers.delete('X-Nas-No-Auth');
        }
        init = Object.assign({}, init, { headers: headers });
      }
      return rawFetch(input, init);
    };
    global.__nasFetchPatched = true;
  }

  /**
   * @param {string} path - absolute path or path under /api/v1 (e.g. "/storage/volumes")
   * @param {RequestInit & { json?: any, auth?: boolean, raw?: boolean }} [opts]
   * @returns {Promise<{ ok: boolean, status: number, data: any, response: Response }>}
   */
  async function apiFetch(path, opts) {
    opts = opts || {};
    let url = path;
    if (path.startsWith('/api/')) {
      url = path;
    } else if (path.startsWith('http://') || path.startsWith('https://')) {
      url = path;
    } else {
      const p = path.startsWith('/') ? path : '/' + path;
      url = API_BASE + p;
    }

    const headers = new Headers(opts.headers || {});
    if (opts.json !== undefined && !headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json');
    }
    const useAuth = opts.auth !== false;
    if (useAuth) {
      const token = getAuthToken();
      if (token && !headers.has('Authorization')) {
        headers.set('Authorization', token);
      }
    } else {
      headers.set('X-Nas-No-Auth', '1');
    }

    const init = Object.assign({}, opts);
    delete init.json;
    delete init.auth;
    delete init.raw;
    init.headers = headers;
    if (opts.json !== undefined) {
      init.body = JSON.stringify(opts.json);
    }

    const response = await fetch(url, init);
    if (opts.raw) {
      return { ok: response.ok, status: response.status, data: null, response };
    }

    let data = null;
    const text = await response.text();
    if (text) {
      try {
        data = JSON.parse(text);
      } catch (_) {
        data = { code: response.status, message: text };
      }
    } else {
      data = { code: response.ok ? 0 : response.status, message: response.statusText };
    }

    const apiOk = response.ok && (data == null || data.code === undefined || data.code === 0);
    return { ok: apiOk, status: response.status, data, response };
  }

  async function apiLogin(username, password, remember) {
    return apiFetch('/auth/login', {
      method: 'POST',
      auth: false,
      json: { username, password, remember: !!remember },
    });
  }

  installFetchInterceptor();

  global.NasAPI = {
    API_BASE,
    getAuthToken,
    setAuthToken,
    clearAuthToken,
    apiFetch,
    apiLogin,
    installFetchInterceptor,
  };
  global.apiFetch = apiFetch;
  global.apiLogin = apiLogin;
  global.getAuthToken = getAuthToken;
  global.setAuthToken = setAuthToken;
  global.clearAuthToken = clearAuthToken;
})(typeof window !== 'undefined' ? window : globalThis);
