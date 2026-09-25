/**
 * api.js - thin fetch wrapper around the Go backend REST API.
 * Every response from the backend follows { success, message, data, error }.
 * This module normalizes that so callers can just await api.get(...).
 */
const API = (() => {
  const BASE_URL = window.APP_CONFIG?.API_BASE_URL || '/api';

  function getToken() {
    return localStorage.getItem('cg_token');
  }

  function buildUrl(path) {
    const normalizedPath = path.startsWith('/') ? path : `/${path}`;
    return `${BASE_URL}${normalizedPath}`;
  }

  async function request(method, path, body) {
    const headers = { 'Content-Type': 'application/json' };
    const token = getToken();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    let res;
    try {
      res = await fetch(buildUrl(path), {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
      });
    } catch (networkErr) {
      throw new Error('Cannot reach the server. Check that the backend is running.');
    }

    let text = '';
    try {
      text = await res.text();
    } catch (textErr) {
      throw new Error(`Failed to read server response (status ${res.status}).`);
    }

    let json;
    const trimmed = (text || '').trim();
    if (!trimmed) {
      json = { success: res.ok, data: null };
    } else {
      try {
        json = JSON.parse(trimmed);
      } catch (parseErr) {
        if (trimmed.startsWith('<')) {
          throw new Error(`Unexpected server response (status ${res.status}): Server returned HTML instead of JSON. Verify backend is running on port 8080.`);
        }
        throw new Error(`Unexpected server response (status ${res.status}): ${trimmed.slice(0, 120)}`);
      }
    }

    if (res.status === 401) {
      localStorage.removeItem('cg_token');
      localStorage.removeItem('cg_user');
      if (!location.pathname.endsWith('login.html')) {
        location.href = 'login.html?expired=1';
      }
      throw new Error((json && json.message) || 'Session expired');
    }

    if (!json || !json.success) {
      throw new Error((json && json.message) || 'Request failed');
    }

    return json.data;
  }

  return {
    get: (path) => request('GET', path),
    post: (path, body) => request('POST', path, body),
    put: (path, body) => request('PUT', path, body),
    delete: (path, body) => request('DELETE', path, body),
    del: (path, body) => request('DELETE', path, body),
  };
})();
