/**
 * notifications-page.js — client side notification listing and dismissing.
 */
let allNotifications = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('notifications.html');

  document.getElementById('filter-read-status').addEventListener('change', applyFilters);

  await loadNotifications();
});

async function loadNotifications() {
  const container = document.getElementById('notifications-list');
  container.innerHTML = '<p class="state-panel">Loading notifications...</p>';

  try {
    allNotifications = await API.get('/notifications');
    applyFilters();
  } catch (err) {
    container.innerHTML = `<p class="state-panel error">${err.message}</p>`;
  }
}

function applyFilters() {
  const filter = document.getElementById('filter-read-status').value;
  const filtered = allNotifications.filter(n => {
    if (filter === 'UNREAD') return !n.is_read;
    if (filter === 'READ') return n.is_read;
    return true;
  });

  renderNotifications(filtered);
}

function renderNotifications(items) {
  const container = document.getElementById('notifications-list');
  if (items.length === 0) {
    container.innerHTML = '<p class="state-panel">No notifications found matching your selection.</p>';
    return;
  }

  container.innerHTML = items.map(n => {
    let sevColor = '#2B6CB0'; // INFO
    if (n.severity === 'WARNING') sevColor = '#ED8936';
    else if (n.severity === 'CRITICAL') sevColor = '#E53E3E';

    let cardBg = n.is_read ? 'var(--color-bg-light)' : 'rgba(43, 108, 176, 0.05)';
    let dateStr = new Date(n.created_at).toLocaleString();

    let actionButton = '';
    if (!n.is_read) {
      actionButton = `
        <button class="btn btn-secondary btn-sm" onclick="markAsRead(${n.id})" style="margin-left: 16px;">
          Mark as Read
        </button>
      `;
    }

    return `
      <div class="notification-card" style="display: flex; align-items: center; justify-content: space-between; padding: 16px; margin-bottom: 10px; border-radius: 6px; background: ${cardBg}; border-left: 5px solid ${sevColor}; box-shadow: var(--shadow-sm);">
        <div style="flex: 1;">
          <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
            <strong style="font-size: 14.5px; color: var(--color-ink);">${n.title}</strong>
            <span class="badge" style="background: ${sevColor}20; color: ${sevColor}; border: 1px solid ${sevColor}40; font-size:11px;">
              ${n.severity}
            </span>
            <span class="badge badge-inactive" style="font-size:11px;">${n.type}</span>
          </div>
          <p style="margin: 6px 0 4px 0; font-size: 13.5px; line-height: 1.5; color: var(--color-ink-muted);">${n.message}</p>
          <span class="hint" style="font-size: 11.5px;">${dateStr}</span>
        </div>
        <div>
          ${actionButton}
        </div>
      </div>
    `;
  }).join('');
}

async function markAsRead(id) {
  try {
    await API.put(`/notifications/${id}/read`);
    showToast('Notification marked as read', 'success');
    await loadNotifications();
  } catch (err) {
    showError(err);
  }
}
