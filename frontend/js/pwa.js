// pwa.js - Handles Service Worker registration, IndexedDB offline queuing and auto-sync

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/service-worker.js')
      .then((registration) => {
        console.log('[PWA] Service Worker registered with scope:', registration.scope);
      })
      .catch((err) => {
        console.warn('[PWA] Service Worker registration failed:', err);
      });
  });
}

// Global IndexedDB offline store
const OFFLINE_DB_NAME = 'CoalGuardOfflineStore';
const OFFLINE_DB_VERSION = 2;

function openOfflineDB() {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(OFFLINE_DB_NAME, OFFLINE_DB_VERSION);
    req.onupgradeneeded = (e) => {
      const db = e.target.result;
      if (!db.objectStoreNames.contains('inspections')) {
        db.createObjectStore('inspections', { keyPath: 'id', autoIncrement: true });
      }
      if (!db.objectStoreNames.contains('incidents')) {
        db.createObjectStore('incidents', { keyPath: 'id', autoIncrement: true });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

window.OfflineStore = {
  async save(storeName, item) {
    const db = await openOfflineDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(storeName, 'readwrite');
      tx.objectStore(storeName).add({ ...item, saved_offline_at: new Date().toISOString() });
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    });
  },

  async getAll(storeName) {
    const db = await openOfflineDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(storeName, 'readonly');
      const req = tx.objectStore(storeName).getAll();
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  },

  async delete(storeName, id) {
    const db = await openOfflineDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(storeName, 'readwrite');
      tx.objectStore(storeName).delete(id);
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    });
  }
};

window.syncAllOfflineData = async function() {
  if (!navigator.onLine) return;
  const token = localStorage.getItem('cg_token');
  if (!token) return;

  const API_BASE = window.APP_CONFIG?.API_BASE_URL || 'http://localhost:8081/api';

  // 1. Sync Inspections
  try {
    const inspections = await window.OfflineStore.getAll('inspections');
    if (inspections.length > 0) {
      if (window.showToast) window.showToast(`Syncing ${inspections.length} offline inspection(s)...`, 'info');
      for (const item of inspections) {
        const fd = new FormData();
        for (const key in item) {
          if (key !== 'id' && key !== 'saved_offline_at') {
            fd.append(key, item[key]);
          }
        }
        const res = await fetch(`${API_BASE}/inspections`, {
          method: 'POST',
          headers: { 'Authorization': `Bearer ${token}` },
          body: fd
        });
        const text = await res.text();
        let json;
        try { json = (text || '').trim() ? JSON.parse(text) : { success: res.ok }; } catch (e) { json = { success: false }; }
        if (json.success) {
          await window.OfflineStore.delete('inspections', item.id);
        }
      }
      if (window.showToast) window.showToast(`Offline inspections synced successfully!`, 'success');
      if (window.loadInspections) window.loadInspections();
    }
  } catch (e) {
    console.error('Inspections sync error', e);
  }

  // 2. Sync Incidents
  try {
    const incidents = await window.OfflineStore.getAll('incidents');
    if (incidents.length > 0) {
      if (window.showToast) window.showToast(`Syncing ${incidents.length} offline incident report(s)...`, 'info');
      for (const item of incidents) {
        const res = await fetch(`${API_BASE}/incidents`, {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json'
          },
          body: JSON.stringify(item)
        });
        const text = await res.text();
        let json;
        try { json = (text || '').trim() ? JSON.parse(text) : { success: res.ok }; } catch (e) { json = { success: false }; }
        if (json.success) {
          await window.OfflineStore.delete('incidents', item.id);
        }
      }
      if (window.showToast) window.showToast(`Offline incident reports synced successfully!`, 'success');
      if (window.loadIncidents) window.loadIncidents();
    }
  } catch (e) {
    console.error('Incidents sync error', e);
  }
};

function updateNetworkStatusUI() {
  let indicator = document.getElementById('network-status-indicator');
  
  if (!indicator) {
    const topbar = document.querySelector('.topbar');
    if (!topbar) return;
    
    indicator = document.createElement('div');
    indicator.id = 'network-status-indicator';
    indicator.style.display = 'inline-flex';
    indicator.style.alignItems = 'center';
    indicator.style.padding = '4px 10px';
    indicator.style.borderRadius = '4px';
    indicator.style.fontSize = '12px';
    indicator.style.fontWeight = '600';
    indicator.style.marginLeft = 'auto';
    indicator.style.marginRight = '16px';
    
    const userInfo = document.getElementById('topbar-user-info');
    if (userInfo) {
      topbar.insertBefore(indicator, userInfo);
    } else {
      topbar.appendChild(indicator);
    }
  }
  
  if (navigator.onLine) {
    const onlineSvg = window.ICONS ? ICONS.get('cloud_online') : '';
    indicator.innerHTML = `<span style="display:inline-flex; align-items:center; margin-right:4px;">${onlineSvg}</span> Online (Sync Active)`;
    indicator.style.backgroundColor = '#e6f4ea';
    indicator.style.color = '#137333';
    indicator.style.display = 'inline-flex';
    indicator.style.alignItems = 'center';
    window.syncAllOfflineData();
  } else {
    const offlineSvg = window.ICONS ? ICONS.get('cloud_offline') : '';
    indicator.innerHTML = `<span style="display:inline-flex; align-items:center; margin-right:4px;">${offlineSvg}</span> Offline Mode (Queued)`;
    indicator.style.backgroundColor = '#fce8e6';
    indicator.style.color = '#c5221f';
    indicator.style.display = 'inline-flex';
    indicator.style.alignItems = 'center';
  }
}

window.addEventListener('online', () => {
  updateNetworkStatusUI();
  if (window.showToast) {
    window.showToast('Network restored. Syncing offline data...', 'success');
  }
  window.syncAllOfflineData();
});

window.addEventListener('offline', () => {
  updateNetworkStatusUI();
  if (window.showToast) {
    window.showToast('Network connection lost. Offline field reporting is active.', 'warning');
  }
});

document.addEventListener('DOMContentLoaded', updateNetworkStatusUI);

