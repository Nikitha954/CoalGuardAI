/**
 * violations.js — Violations Management client logic.
 * Supports manual violation reporting and corrective action assignments.
 */
let allViolations = [];
let allMines = [];
let categories = [];
let assignableUsers = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('violations.html');

  const user = AUTH.getUser();
  const canReport = ['SAFETY_OFFICER', 'SUPER_ADMIN'].includes(user.role_key);

  if (canReport) {
    document.getElementById('report-violation-btn').classList.remove('hidden');
  }

  // Bind UI Events
  document.getElementById('report-violation-btn').addEventListener('click', () => openReportModal());
  document.getElementById('modal-close').addEventListener('click', closeReportModal);
  document.getElementById('modal-cancel').addEventListener('click', closeReportModal);
  document.getElementById('violation-form').addEventListener('submit', handleReportSubmit);

  document.getElementById('assign-close').addEventListener('click', closeAssignModal);
  document.getElementById('assign-cancel').addEventListener('click', closeAssignModal);
  document.getElementById('assign-form').addEventListener('submit', handleAssignSubmit);

  document.getElementById('filter-mine').addEventListener('change', applyFilters);
  document.getElementById('filter-severity').addEventListener('change', applyFilters);
  document.getElementById('filter-status').addEventListener('change', applyFilters);

  // Load resources
  await loadMines();
  await loadCategories();
  await loadAssignableUsers();
  await loadViolations();
});

async function loadMines() {
  try {
    allMines = await API.get('/mines');
    const filterMine = document.getElementById('filter-mine');
    const selectMine = document.getElementById('vio-mine');

    const options = allMines.map(m => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');
    filterMine.innerHTML = `<option value="">All Mines</option>${options}`;
    selectMine.innerHTML = `<option value="" disabled selected>Select Mine...</option>${options}`;
  } catch (err) {
    showError(err);
  }
}

async function loadCategories() {
  try {
    categories = await API.get('/compliance/categories');
    const selectCat = document.getElementById('vio-category');
    selectCat.innerHTML = `<option value="" disabled selected>Select Category...</option>` + 
      categories.map(c => `<option value="${c.id}">${c.name}</option>`).join('');
  } catch (err) {
    showError(err);
  }
}

async function loadAssignableUsers() {
  try {
    // Fetch users (workers / supervisors)
    assignableUsers = await API.get('/users');
    const selectUser = document.getElementById('assign-user');
    
    // Allow assignment to MINE_MANAGER, SAFETY_OFFICER, or INSPECTOR for simple hackathon simulation
    selectUser.innerHTML = `<option value="" disabled selected>Select Assignee...</option>` +
      assignableUsers.map(u => `<option value="${u.id}">${u.full_name} (${u.role_name})</option>`).join('');
  } catch (err) {
    showError(err);
  }
}

async function loadViolations() {
  const tbody = document.getElementById('violations-table-body');
  tbody.innerHTML = `<tr><td colspan="10" class="state-panel">Loading violations...</td></tr>`;

  try {
    allViolations = await API.get('/violations');
    renderViolationsTable(allViolations);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="10" class="state-panel error">${err.message}</td></tr>`;
  }
}

function applyFilters() {
  const mine = document.getElementById('filter-mine').value;
  const severity = document.getElementById('filter-severity').value;
  const status = document.getElementById('filter-status').value;

  const filtered = allViolations.filter(v => {
    if (mine && String(v.mine_id) !== mine) return false;
    if (severity && v.severity !== severity) return false;
    if (status && v.status !== status) return false;
    return true;
  });

  renderViolationsTable(filtered);
}

function renderViolationsTable(violations) {
  const tbody = document.getElementById('violations-table-body');
  if (violations.length === 0) {
    tbody.innerHTML = `<tr><td colspan="10" class="state-panel">No violations found.</td></tr>`;
    return;
  }

  const user = AUTH.getUser();
  const canAssign = ['MINE_MANAGER', 'SAFETY_OFFICER', 'SUPER_ADMIN'].includes(user.role_key);

  tbody.innerHTML = violations.map(v => {
    let severityClass = 'badge-inactive';
    if (v.severity === 'CRITICAL') severityClass = 'badge-danger';
    else if (v.severity === 'HIGH') severityClass = 'badge-danger'; // or badge-orange
    else if (v.severity === 'MEDIUM') severityClass = 'badge-warning';

    let statusClass = 'inactive';
    if (v.status === 'CLOSED' || v.status === 'VERIFIED') statusClass = 'active';
    else if (v.status === 'OPEN' || v.status === 'OVERDUE') statusClass = 'danger';
    else if (v.status === 'IN_PROGRESS' || v.status === 'RESOLVED') statusClass = 'warning';

    let actionsHtml = '';
    if (v.status === 'OPEN' && canAssign) {
      actionsHtml = `<button class="btn btn-primary btn-sm" onclick="openAssignModal(${v.id}, '${v.violation_code}')">Assign Action</button>`;
    } else {
      actionsHtml = `<span class="hint">No action required</span>`;
    }

    return `
      <tr>
        <td class="mono"><strong>${v.violation_code}</strong></td>
        <td>${v.mine_name}</td>
        <td>${v.category_name}</td>
        <td style="max-width:240px; font-size:12.5px; line-height:1.4;">${v.description}</td>
        <td><span class="badge ${severityClass}">${v.severity}</span></td>
        <td>${v.reported_by_name}</td>
        <td>${v.responsible_name || '<span class="hint">Unassigned</span>'}</td>
        <td>${v.deadline || '&mdash;'}</td>
        <td><span class="badge badge-${statusClass}">${v.status}</span></td>
        <td>${actionsHtml}</td>
      </tr>
    `;
  }).join('');
}

function openReportModal() {
  const form = document.getElementById('violation-form');
  form.reset();
  document.getElementById('violation-modal').classList.remove('hidden');
}

function closeReportModal() {
  document.getElementById('violation-modal').classList.add('hidden');
}

async function handleReportSubmit(e) {
  e.preventDefault();
  const saveBtn = document.getElementById('modal-save');
  saveBtn.disabled = true;
  saveBtn.textContent = 'Reporting...';

  const payload = {
    mine_id: parseInt(document.getElementById('vio-mine').value, 10),
    category_id: parseInt(document.getElementById('vio-category').value, 10),
    description: document.getElementById('vio-description').value.trim(),
    severity: document.getElementById('vio-severity').value,
    deadline: document.getElementById('vio-deadline').value
  };

  try {
    await API.post('/violations', payload);
    showToast('Violation logged successfully', 'success');
    closeReportModal();
    await loadViolations();
  } catch (err) {
    showError(err);
  } finally {
    saveBtn.disabled = false;
    saveBtn.textContent = 'Report Violation';
  }
}

function openAssignModal(id, code) {
  document.getElementById('assign-violation-id').value = id;
  document.getElementById('assign-violation-code').value = code;
  document.getElementById('assign-desc').value = '';
  
  // Pre-fill deadline to +3 days from now
  const defaultDeadline = new Date();
  defaultDeadline.setDate(defaultDeadline.getDate() + 3);
  document.getElementById('assign-deadline').value = defaultDeadline.toISOString().split('T')[0];

  document.getElementById('assign-modal').classList.remove('hidden');
}

function closeAssignModal() {
  document.getElementById('assign-modal').classList.add('hidden');
}

async function handleAssignSubmit(e) {
  e.preventDefault();
  const saveBtn = document.getElementById('assign-save');
  saveBtn.disabled = true;
  saveBtn.textContent = 'Assigning...';

  const payload = {
    violation_id: parseInt(document.getElementById('assign-violation-id').value, 10),
    assigned_to: parseInt(document.getElementById('assign-user').value, 10),
    action_description: document.getElementById('assign-desc').value.trim(),
    deadline: document.getElementById('assign-deadline').value
  };

  try {
    await API.post('/corrective-actions', payload);
    showToast('Corrective action assigned successfully', 'success');
    closeAssignModal();
    await loadViolations();
  } catch (err) {
    showError(err);
  } finally {
    saveBtn.disabled = false;
    saveBtn.textContent = 'Assign Action';
  }
}
