/**
 * incidents.js — Incidents tracking & offline reporting logic.
 */
let allIncidents = [];
let allMines = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('incidents.html');

  // Set default datetime to now
  const now = new Date();
  now.setMinutes(now.getMinutes() - now.getTimezoneOffset());
  document.getElementById('inc-date').value = now.toISOString().slice(0, 16);

  document.getElementById('filter-mine').addEventListener('change', loadIncidents);
  document.getElementById('filter-severity').addEventListener('change', loadIncidents);
  document.getElementById('filter-status').addEventListener('change', loadIncidents);

  document.getElementById('btn-report-incident').addEventListener('click', openIncidentModal);
  document.getElementById('incident-modal-close').addEventListener('click', closeIncidentModal);
  document.getElementById('incident-modal-cancel').addEventListener('click', closeIncidentModal);
  document.getElementById('incident-form').addEventListener('submit', handleIncidentSubmit);

  await loadMines();
  await loadIncidents();
});

async function loadMines() {
  try {
    allMines = await API.get('/mines');
    const filterMine = document.getElementById('filter-mine');
    const incMine = document.getElementById('inc-mine');

    const opts = allMines.map(m => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');
    filterMine.innerHTML = `<option value="">All Mines</option>${opts}`;
    incMine.innerHTML = opts;
  } catch (err) {
    showError(err);
  }
}

async function loadIncidents() {
  const tbody = document.getElementById('incidents-table-body');
  tbody.innerHTML = `<tr><td colspan="9" class="state-panel">Loading incidents...</td></tr>`;

  try {
    const mineId = document.getElementById('filter-mine').value;
    const severity = document.getElementById('filter-severity').value;
    const status = document.getElementById('filter-status').value;

    let url = `/incidents?`;
    if (mineId) url += `mine_id=${mineId}&`;
    if (severity) url += `severity=${severity}&`;
    if (status) url += `status=${status}&`;

    allIncidents = await API.get(url);
    updateKPIs(allIncidents);
    renderIncidentsTable(allIncidents);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel error">${err.message}</td></tr>`;
  }
}

function updateKPIs(items) {
  document.getElementById('kpi-total-inc').textContent = items.length;
  document.getElementById('kpi-critical-inc').textContent = items.filter(i => i.severity === 'CRITICAL').length;
  document.getElementById('kpi-open-inc').textContent = items.filter(i => i.status === 'OPEN' || i.status === 'UNDER_REVIEW').length;
  document.getElementById('kpi-closed-inc').textContent = items.filter(i => i.status === 'CLOSED').length;
}

function renderIncidentsTable(items) {
  const tbody = document.getElementById('incidents-table-body');
  const user = AUTH.getUser();
  const canManage = ['MINE_MANAGER', 'SAFETY_OFFICER', 'SUPER_ADMIN'].includes(user.role_key);

  if (!items || items.length === 0) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel">No incidents match current filters.</td></tr>`;
    return;
  }

  tbody.innerHTML = items.map(i => {
    let sevBadge = 'badge-info';
    if (i.severity === 'CRITICAL') sevBadge = 'badge-critical';
    else if (i.severity === 'HIGH') sevBadge = 'badge-warning';

    let statusBadge = 'badge-draft';
    if (i.status === 'UNDER_REVIEW') statusBadge = 'badge-warning';
    else if (i.status === 'CLOSED') statusBadge = 'badge-active';

    const incDate = i.incident_date ? new Date(i.incident_date).toLocaleString('en-IN') : '-';

    let actions = '-';
    if (canManage && i.status !== 'CLOSED') {
      actions = `
        <div style="display:flex; gap:6px;">
          ${i.status === 'OPEN' ? `<button class="btn btn-secondary btn-sm" onclick="updateIncidentStatus(${i.id}, 'UNDER_REVIEW')">Review</button>` : ''}
          <button class="btn btn-primary btn-sm" onclick="updateIncidentStatus(${i.id}, 'CLOSED')">Close</button>
        </div>
      `;
    }

    return `
      <tr>
        <td class="mono font-semibold">#${i.id}</td>
        <td><strong>${i.mine_name}</strong></td>
        <td>${i.incident_type}</td>
        <td><span class="badge ${sevBadge}">${i.severity}</span></td>
        <td><div style="max-width:320px; line-height:1.4;">${i.description}</div></td>
        <td>${i.reported_by_name}</td>
        <td class="mono">${incDate}</td>
        <td><span class="badge ${statusBadge}">${i.status}</span></td>
        <td>${actions}</td>
      </tr>
    `;
  }).join('');
}

function openIncidentModal() {
  document.getElementById('incident-modal').classList.remove('hidden');
}

function closeIncidentModal() {
  document.getElementById('incident-modal').classList.add('hidden');
  document.getElementById('incident-form').reset();
}

async function handleIncidentSubmit(e) {
  e.preventDefault();
  const mineId = parseInt(document.getElementById('inc-mine').value);
  const incType = document.getElementById('inc-type').value;
  const severity = document.getElementById('inc-sev').value;
  const incDate = document.getElementById('inc-date').value.replace('T', ' ') + ':00';
  const desc = document.getElementById('inc-desc').value.trim();

  const payload = {
    mine_id: mineId,
    incident_type: incType,
    severity: severity,
    incident_date: incDate,
    description: desc
  };

  // Offline Check
  if (!navigator.onLine) {
    if (window.OfflineStore) {
      await window.OfflineStore.save('incidents', payload);
      showToast('Offline Mode: Incident report saved to local queue. Will sync when online.', 'warning');
      closeIncidentModal();
      return;
    }
  }

  try {
    await API.post('/incidents', payload);
    showToast('Incident report filed successfully and notifications sent.', 'success');
    closeIncidentModal();
    await loadIncidents();
  } catch (err) {
    if (window.OfflineStore && !navigator.onLine) {
      await window.OfflineStore.save('incidents', payload);
      showToast('Incident saved locally in offline queue.', 'warning');
      closeIncidentModal();
    } else {
      showError(err);
    }
  }
}

window.updateIncidentStatus = async function(id, newStatus) {
  if (!confirm(`Update incident #${id} to status ${newStatus}?`)) return;
  try {
    await API.put(`/incidents/${id}/status`, { status: newStatus });
    showToast(`Incident marked as ${newStatus}`, 'success');
    await loadIncidents();
  } catch (err) {
    showError(err);
  }
};
