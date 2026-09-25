/**
 * grievances.js — Worker Grievance Redressal portal client logic.
 */
let allGrievances = [];
let allMines = [];
let allOfficers = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('grievances.html');

  // Filter Listeners
  document.getElementById('filter-mine').addEventListener('change', loadGrievances);
  document.getElementById('filter-category').addEventListener('change', loadGrievances);
  document.getElementById('filter-status').addEventListener('change', loadGrievances);

  // File Modal
  document.getElementById('btn-file-grievance').addEventListener('click', openGrievanceModal);
  document.getElementById('grievance-modal-close').addEventListener('click', closeGrievanceModal);
  document.getElementById('grievance-modal-cancel').addEventListener('click', closeGrievanceModal);
  document.getElementById('grievance-form').addEventListener('submit', handleGrievanceSubmit);

  // Assign Modal
  document.getElementById('assign-modal-close').addEventListener('click', closeAssignModal);
  document.getElementById('assign-modal-cancel').addEventListener('click', closeAssignModal);
  document.getElementById('assign-form').addEventListener('submit', handleAssignSubmit);

  // Resolve Modal
  document.getElementById('resolve-modal-close').addEventListener('click', closeResolveModal);
  document.getElementById('resolve-modal-cancel').addEventListener('click', closeResolveModal);
  document.getElementById('resolve-form').addEventListener('submit', handleResolveSubmit);

  // Escalate Modal
  document.getElementById('escalate-modal-close').addEventListener('click', closeEscalateModal);
  document.getElementById('escalate-modal-cancel').addEventListener('click', closeEscalateModal);
  document.getElementById('escalate-form').addEventListener('submit', handleEscalateSubmit);

  document.getElementById('g-mine').addEventListener('change', (e) => loadWorkersForMine(e.target.value));

  await loadInitialData();
  await loadGrievances();
});

async function loadInitialData() {
  try {
    const [mines, users] = await Promise.all([
      API.get('/mines'),
      API.get('/users').catch(() => [])
    ]);

    allMines = mines;
    allOfficers = users.filter(u => ['MINE_MANAGER', 'SAFETY_OFFICER', 'SUPER_ADMIN'].includes(u.role_key));

    // Mine options
    const mineFilter = document.getElementById('filter-mine');
    const gMine = document.getElementById('g-mine');
    const mineOpts = allMines.map(m => `<option value="${m.id}">${m.mine_name}</option>`).join('');
    mineFilter.innerHTML = `<option value="">All Mines</option>${mineOpts}`;
    gMine.innerHTML = mineOpts;

    // Assign officers options
    const assignSelect = document.getElementById('assign-officer');
    assignSelect.innerHTML = allOfficers.map(u => `<option value="${u.id}">${u.full_name} (${u.role_name})</option>`).join('');

    if (allMines.length > 0) {
      await loadWorkersForMine(allMines[0].id);
    }
  } catch (err) {
    showError(err);
  }
}

async function loadWorkersForMine(mineId) {
  try {
    const workers = await API.get(`/workers?mine_id=${mineId}`).catch(() => []);
    const gWorker = document.getElementById('g-worker');
    const opts = workers.map(w => `<option value="${w.id}">${w.full_name} (${w.worker_code})</option>`).join('');
    gWorker.innerHTML = `<option value="">Anonymous Worker Submission</option>${opts}`;
  } catch (e) {
    console.error(e);
  }
}

async function loadGrievances() {
  const tbody = document.getElementById('grievances-table-body');
  tbody.innerHTML = `<tr><td colspan="9" class="state-panel">Loading grievances...</td></tr>`;

  try {
    const mineId = document.getElementById('filter-mine').value;
    const category = document.getElementById('filter-category').value;
    const status = document.getElementById('filter-status').value;

    let url = `/grievances?`;
    if (mineId) url += `mine_id=${mineId}&`;
    if (category) url += `category=${category}&`;
    if (status) url += `status=${status}&`;

    allGrievances = await API.get(url);
    updateKPIs(allGrievances);
    renderGrievancesTable(allGrievances);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel error">${err.message}</td></tr>`;
  }
}

function updateKPIs(items) {
  document.getElementById('kpi-total-g').textContent = items.length;
  document.getElementById('kpi-submitted-g').textContent = items.filter(g => g.status === 'SUBMITTED').length;
  document.getElementById('kpi-inreview-g').textContent = items.filter(g => g.status === 'IN_REVIEW').length;
  document.getElementById('kpi-resolved-g').textContent = items.filter(g => ['RESOLVED', 'CLOSED'].includes(g.status)).length;
}

function renderGrievancesTable(items) {
  const tbody = document.getElementById('grievances-table-body');
  const user = AUTH.getUser();
  const canAct = ['SUPER_ADMIN', 'MINE_MANAGER', 'SAFETY_OFFICER'].includes(user.role_key);

  if (!items || items.length === 0) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel">No grievances match current filters.</td></tr>`;
    return;
  }

  tbody.innerHTML = items.map(g => {
    let badgeClass = 'badge-draft';
    if (g.status === 'IN_REVIEW') badgeClass = 'badge-warning';
    else if (g.status === 'RESOLVED') badgeClass = 'badge-active';
    else if (g.status === 'ESCALATED') badgeClass = 'badge-critical';
    else if (g.status === 'CLOSED') badgeClass = 'badge-inactive';

    const filedDate = g.created_at ? new Date(g.created_at).toLocaleDateString('en-IN') : '-';

    let actionButtons = '<span class="hint">-</span>';
    if (canAct && g.status !== 'RESOLVED' && g.status !== 'CLOSED') {
      const userAssignSvg = window.ICONS ? ICONS.get('user_assign') : '';
      const checkSvg = window.ICONS ? ICONS.get('check') : '';
      const alertSvg = window.ICONS ? ICONS.get('alert') : '';
      actionButtons = `
        <div style="display:flex; gap:4px; flex-wrap:wrap; justify-content:flex-end;">
          <button class="btn btn-secondary btn-sm" style="padding:4px 8px; font-size:12px;" onclick="openAssignModal(${g.id})">${userAssignSvg} Assign</button>
          <button class="btn btn-primary btn-sm" style="padding:4px 8px; font-size:12px;" onclick="openResolveModal(${g.id})">${checkSvg} Resolve</button>
          <button class="btn btn-danger btn-sm" style="padding:4px 8px; font-size:12px;" onclick="openEscalateModal(${g.id})">${alertSvg} Escalate</button>
        </div>
      `;
    }

    return `
      <tr>
        <td class="mono font-semibold" style="color:var(--color-brand);">#${g.id}</td>
        <td>
          <div style="font-weight:600; color:var(--color-ink);">${g.worker_name}</div>
          ${g.worker_code ? `<div class="mono hint" style="font-size:11px;">${g.worker_code}</div>` : ''}
        </td>
        <td><strong>${g.mine_name}</strong></td>
        <td><span class="badge badge-info">${g.category}</span></td>
        <td>
          <div style="max-width:300px; line-height:1.4; font-size:13px;">${g.description}</div>
          ${g.resolution_notes ? `<div style="margin-top:6px; font-size:11.5px; color:var(--color-ink-muted); background:#f1f5f9; padding:4px 8px; border-radius:4px; border-left:3px solid var(--color-brand);"><strong>Notes:</strong> ${g.resolution_notes}</div>` : ''}
        </td>
        <td><span class="badge ${badgeClass}">${g.status}</span></td>
        <td style="font-size:12.5px;">${g.assigned_to_name}</td>
        <td class="mono" style="font-size:12px;">${filedDate}</td>
        <td style="text-align: right;">${actionButtons}</td>
      </tr>
    `;
  }).join('');
}

function openGrievanceModal() {
  document.getElementById('grievance-modal').classList.remove('hidden');
}

function closeGrievanceModal() {
  document.getElementById('grievance-modal').classList.add('hidden');
  document.getElementById('grievance-form').reset();
}

async function handleGrievanceSubmit(e) {
  e.preventDefault();
  const mineId = parseInt(document.getElementById('g-mine').value);
  const workerStr = document.getElementById('g-worker').value;
  const workerId = workerStr ? parseInt(workerStr) : null;
  const category = document.getElementById('g-category').value;
  const desc = document.getElementById('g-desc').value.trim();

  try {
    await API.post('/grievances', {
      mine_id: mineId,
      worker_id: workerId,
      category: category,
      description: desc
    });

    showToast('Grievance filed successfully and safety officer alerted', 'success');
    closeGrievanceModal();
    await loadGrievances();
  } catch (err) {
    showError(err);
  }
}

window.openAssignModal = function(id) {
  document.getElementById('assign-g-id').value = id;
  document.getElementById('assign-modal').classList.remove('hidden');
};

function closeAssignModal() {
  document.getElementById('assign-modal').classList.add('hidden');
  document.getElementById('assign-form').reset();
}

async function handleAssignSubmit(e) {
  e.preventDefault();
  const id = document.getElementById('assign-g-id').value;
  const assignedTo = parseInt(document.getElementById('assign-officer').value);

  try {
    await API.put(`/grievances/${id}/assign`, { assigned_to: assignedTo });
    showToast('Grievance assigned for investigation', 'success');
    closeAssignModal();
    await loadGrievances();
  } catch (err) {
    showError(err);
  }
}

window.openResolveModal = function(id) {
  document.getElementById('resolve-g-id').value = id;
  document.getElementById('resolve-modal').classList.remove('hidden');
};

function closeResolveModal() {
  document.getElementById('resolve-modal').classList.add('hidden');
  document.getElementById('resolve-form').reset();
}

async function handleResolveSubmit(e) {
  e.preventDefault();
  const id = document.getElementById('resolve-g-id').value;
  const notes = document.getElementById('resolve-notes').value.trim();

  try {
    await API.put(`/grievances/${id}/resolve`, { resolution_notes: notes });
    showToast('Grievance resolved successfully', 'success');
    closeResolveModal();
    await loadGrievances();
  } catch (err) {
    showError(err);
  }
}

window.openEscalateModal = function(id) {
  document.getElementById('escalate-g-id').value = id;
  document.getElementById('escalate-modal').classList.remove('hidden');
};

function closeEscalateModal() {
  document.getElementById('escalate-modal').classList.add('hidden');
  document.getElementById('escalate-form').reset();
}

async function handleEscalateSubmit(e) {
  e.preventDefault();
  const id = document.getElementById('escalate-g-id').value;
  const reason = document.getElementById('escalate-reason').value.trim();

  try {
    await API.put(`/grievances/${id}/escalate`, { reason: reason });
    showToast('Grievance escalated to Corporate & Ministry oversight', 'warning');
    closeEscalateModal();
    await loadGrievances();
  } catch (err) {
    showError(err);
  }
}
