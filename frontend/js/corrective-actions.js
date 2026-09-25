/**
 * corrective-actions.js — Corrective actions module logic.
 * Handles task execution, verification review, and escalation level tracking.
 */
let allActions = [];
let myTasksOnly = false;

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('corrective-actions.html');

  const user = AUTH.getUser();
  const isSupervisor = ['MINE_MANAGER', 'SAFETY_OFFICER', 'SUPER_ADMIN'].includes(user.role_key);

  if (isSupervisor) {
    document.getElementById('run-escalation-btn').classList.remove('hidden');
  }

  // Bind UI Events
  document.getElementById('run-escalation-btn').addEventListener('click', runEscalationCheck);
  document.getElementById('filter-my-tasks').addEventListener('click', toggleMyTasks);
  document.getElementById('filter-status').addEventListener('change', applyFilters);

  document.getElementById('verify-close').addEventListener('click', closeVerifyModal);
  document.getElementById('verify-cancel').addEventListener('click', closeVerifyModal);
  document.getElementById('verify-form').addEventListener('submit', handleVerifySubmit);

  await loadActions();
});

async function loadActions() {
  const tbody = document.getElementById('actions-table-body');
  tbody.innerHTML = `<tr><td colspan="8" class="state-panel">Loading corrective actions...</td></tr>`;

  try {
    allActions = await API.get('/corrective-actions');
    applyFilters();
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="8" class="state-panel error">${err.message}</td></tr>`;
  }
}

function toggleMyTasks() {
  myTasksOnly = !myTasksOnly;
  const btn = document.getElementById('filter-my-tasks');
  if (myTasksOnly) {
    btn.textContent = 'Show All Tasks';
    btn.classList.add('btn-primary');
    btn.classList.remove('btn-secondary');
  } else {
    btn.textContent = 'Show My Tasks Only';
    btn.classList.add('btn-secondary');
    btn.classList.remove('btn-primary');
  }
  applyFilters();
}

function applyFilters() {
  const status = document.getElementById('filter-status').value;
  const user = AUTH.getUser();

  const filtered = allActions.filter(ca => {
    if (status && ca.status !== status) return false;
    if (myTasksOnly && ca.assigned_to !== user.id) return false;
    return true;
  });

  renderActionsTable(filtered);
}

function renderActionsTable(actions) {
  const tbody = document.getElementById('actions-table-body');
  if (actions.length === 0) {
    tbody.innerHTML = `<tr><td colspan="8" class="state-panel">No corrective actions found.</td></tr>`;
    return;
  }

  const currentUser = AUTH.getUser();
  const isSupervisor = ['MINE_MANAGER', 'SAFETY_OFFICER', 'SUPER_ADMIN'].includes(currentUser.role_key);

  tbody.innerHTML = actions.map(ca => {
    let statusClass = 'inactive';
    if (ca.status === 'CLOSED' || ca.status === 'VERIFIED') statusClass = 'active';
    else if (ca.status === 'ASSIGNED') statusClass = 'info';
    else if (ca.status === 'SUBMITTED') statusClass = 'warning';
    else if (ca.status === 'OVERDUE') statusClass = 'danger';

    let escLabel = '&mdash;';
    if (ca.escalation_level > 0) {
      let escRole = 'Safety Officer';
      if (ca.escalation_level === 2) escRole = 'Mine Manager';
      if (ca.escalation_level === 3) escRole = 'Corporate Management';
      escLabel = `<span class="badge badge-danger">L${ca.escalation_level}: ${escRole}</span>`;
    }

    let actionButtonHtml = '';
    if ((ca.status === 'ASSIGNED' || ca.status === 'OVERDUE') && ca.assigned_to === currentUser.id) {
      actionButtonHtml = `<button class="btn btn-primary btn-sm" onclick="submitResolution(${ca.id})">Submit Work</button>`;
    } else if (ca.status === 'SUBMITTED' && isSupervisor) {
      actionButtonHtml = `<button class="btn btn-warning btn-sm" onclick="openVerifyModal(${ca.id})">Verify Work</button>`;
    } else {
      actionButtonHtml = `<span class="hint">Waiting...</span>`;
    }

    return `
      <tr>
        <td>
          <span class="mono"><strong>${ca.violation_code}</strong></span><br/>
          <span class="hint" style="font-size:11.5px;">${ca.severity} Severity</span>
        </td>
        <td>${ca.mine_name}</td>
        <td style="max-width:280px; font-size:12.5px; line-height:1.4;">
          <strong>Req:</strong> ${ca.action_description}<br/>
          <span class="hint" style="font-size:11.5px; display:block; margin-top:2px;">Vio: ${ca.violation_desc}</span>
        </td>
        <td>${ca.assigned_to_name}</td>
        <td>${ca.deadline}</td>
        <td><span class="badge badge-${statusClass}">${ca.status}</span></td>
        <td>${escLabel}</td>
        <td>${actionButtonHtml}</td>
      </tr>
    `;
  }).join('');
}

async function submitResolution(id) {
  if (!confirm('Mark this corrective action as complete and submit for supervisor audit?')) return;
  try {
    await API.put(`/corrective-actions/${id}/submit`);
    showToast('Work submitted successfully for review', 'success');
    await loadActions();
  } catch (err) {
    showError(err);
  }
}

function openVerifyModal(id) {
  document.getElementById('verify-action-id').value = id;
  document.getElementById('verify-remarks').value = '';
  document.getElementById('verify-decision').value = 'APPROVE';
  document.getElementById('verify-modal').classList.remove('hidden');
}

function closeVerifyModal() {
  document.getElementById('verify-modal').classList.add('hidden');
}

async function handleVerifySubmit(e) {
  e.preventDefault();
  const saveBtn = document.getElementById('verify-save');
  saveBtn.disabled = true;
  saveBtn.textContent = 'Submitting...';

  const actionId = document.getElementById('verify-action-id').value;
  const remarks = document.getElementById('verify-remarks').value.trim();
  const decision = document.getElementById('verify-decision').value;

  const payload = {
    approved: decision === 'APPROVE',
    remarks: remarks
  };

  try {
    await API.put(`/corrective-actions/${actionId}/verify`, payload);
    showToast('Verification outcome submitted successfully', 'success');
    closeVerifyModal();
    await loadActions();
  } catch (err) {
    showError(err);
  } finally {
    saveBtn.disabled = false;
    saveBtn.textContent = 'Submit Verdict';
  }
}

async function runEscalationCheck() {
  const btn = document.getElementById('run-escalation-btn');
  btn.disabled = true;
  btn.textContent = 'Checking...';

  try {
    const data = await API.post('/corrective-actions/check-escalations');
    showToast(data.message || 'Deadline & escalation checks completed', 'success');
    await loadActions();
  } catch (err) {
    showError(err);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Check Deadlines & Escalations';
  }
}
