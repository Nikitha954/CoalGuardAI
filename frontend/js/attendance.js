/**
 * attendance.js — Attendance & Workforce Management client logic.
 */
let allMines = [];
let allContractors = [];
let allWorkers = [];
let allAttendance = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('attendance.html');

  const user = AUTH.getUser();
  const canManage = ['SUPER_ADMIN', 'MINE_MANAGER', 'SAFETY_OFFICER'].includes(user.role_key);

  if (canManage) {
    document.getElementById('btn-mark-attendance').classList.remove('hidden');
    document.getElementById('btn-add-worker').classList.remove('hidden');
  }

  // Set default date to today
  const todayStr = new Date().toISOString().split('T')[0];
  document.getElementById('filter-date').value = todayStr;
  document.getElementById('att-date').value = todayStr;

  // Event Listeners
  document.getElementById('filter-mine').addEventListener('change', () => { loadWorkers(); applyFilters(); });
  document.getElementById('filter-contractor').addEventListener('change', applyFilters);
  document.getElementById('filter-date').addEventListener('change', applyFilters);
  document.getElementById('filter-status').addEventListener('change', applyFilters);

  document.getElementById('btn-mark-attendance').addEventListener('click', openMarkModal);
  document.getElementById('mark-modal-close').addEventListener('click', closeMarkModal);
  document.getElementById('mark-modal-cancel').addEventListener('click', closeMarkModal);
  document.getElementById('mark-form').addEventListener('submit', handleMarkSubmit);

  document.getElementById('btn-add-worker').addEventListener('click', openWorkerModal);
  document.getElementById('worker-modal-close').addEventListener('click', closeWorkerModal);
  document.getElementById('worker-modal-cancel').addEventListener('click', closeWorkerModal);
  document.getElementById('worker-form').addEventListener('submit', handleWorkerSubmit);

  document.getElementById('att-mine').addEventListener('change', (e) => populateWorkersDropdown(e.target.value));

  await loadInitialDropdowns();
  await loadAttendance();
  await loadReport();
});

async function loadInitialDropdowns() {
  try {
    const [mines, contractors] = await Promise.all([
      API.get('/mines'),
      API.get('/contractors').catch(() => [])
    ]);
    allMines = mines;
    allContractors = contractors;

    // Populate Mine filters & modals
    const mineFilter = document.getElementById('filter-mine');
    const attMine = document.getElementById('att-mine');
    const wMine = document.getElementById('w-mine');

    const mineOpts = allMines.map(m => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');
    mineFilter.innerHTML = `<option value="">All Mines</option>${mineOpts}`;
    attMine.innerHTML = mineOpts;
    wMine.innerHTML = mineOpts;

    // Populate Contractor filters & modals
    const contFilter = document.getElementById('filter-contractor');
    const wCont = document.getElementById('w-contractor');
    const contOpts = allContractors.map(c => `<option value="${c.id}">${c.company_name}</option>`).join('');
    contFilter.innerHTML = `<option value="">All Contractors / Direct</option>${contOpts}`;
    wCont.innerHTML = `<option value="">Direct CIL Employee</option>${contOpts}`;

    if (allMines.length > 0) {
      await populateWorkersDropdown(allMines[0].id);
    }
  } catch (err) {
    showError(err);
  }
}

async function populateWorkersDropdown(mineId) {
  try {
    const workers = await API.get(`/workers?mine_id=${mineId}`);
    allWorkers = workers;
    const attWorker = document.getElementById('att-worker');
    if (workers.length === 0) {
      attWorker.innerHTML = `<option value="">No active workers found</option>`;
    } else {
      attWorker.innerHTML = workers.map(w => `<option value="${w.id}">${w.full_name} (${w.worker_code} - ${w.designation})</option>`).join('');
    }
  } catch (e) {
    console.error('Failed to populate workers', e);
  }
}

async function loadAttendance() {
  const tbody = document.getElementById('attendance-table-body');
  tbody.innerHTML = `<tr><td colspan="10" class="state-panel">Loading attendance roster...</td></tr>`;

  try {
    const mineId = document.getElementById('filter-mine').value;
    const contId = document.getElementById('filter-contractor').value;
    const date = document.getElementById('filter-date').value;
    const status = document.getElementById('filter-status').value;

    let url = `/attendance?`;
    if (mineId) url += `mine_id=${mineId}&`;
    if (contId) url += `contractor_id=${contId}&`;
    if (date) url += `date=${date}&`;
    if (status) url += `status=${status}&`;

    allAttendance = await API.get(url);
    renderAttendanceTable(allAttendance);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="10" class="state-panel error">${err.message}</td></tr>`;
  }
}

async function loadReport() {
  try {
    const mineId = document.getElementById('filter-mine').value;
    const contId = document.getElementById('filter-contractor').value;
    let url = `/attendance/report?`;
    if (mineId) url += `mine_id=${mineId}&`;
    if (contId) url += `contractor_id=${contId}&`;

    const report = await API.get(url);
    document.getElementById('kpi-total-workers').textContent = report.total_workers || '0';
    document.getElementById('kpi-present-count').textContent = report.status_breakdown?.PRESENT || '0';
    
    const absentTotal = (report.status_breakdown?.ABSENT || 0) + (report.status_breakdown?.LEAVE || 0);
    document.getElementById('kpi-absent-count').textContent = absentTotal;
    
    const rate = Math.round(report.attendance_rate || 0);
    document.getElementById('kpi-rate').textContent = `${rate}%`;
  } catch (e) {
    console.error('Failed to load attendance report', e);
  }
}

function applyFilters() {
  loadAttendance();
  loadReport();
}

function renderAttendanceTable(records) {
  const tbody = document.getElementById('attendance-table-body');
  if (!records || records.length === 0) {
    tbody.innerHTML = `<tr><td colspan="10" class="state-panel">No attendance records match current filters.</td></tr>`;
    return;
  }

  tbody.innerHTML = records.map(r => {
    let badgeClass = 'badge-active';
    if (r.status === 'ABSENT') badgeClass = 'badge-critical';
    else if (r.status === 'LEAVE') badgeClass = 'badge-info';
    else if (r.status === 'HALF_DAY') badgeClass = 'badge-warning';

    return `
      <tr>
        <td class="mono font-semibold">${r.worker_code || 'AGGREGATE'}</td>
        <td><strong>${r.worker_name || 'Mine-wide Headcount'}</strong></td>
        <td>${r.designation || 'All Shifts'}</td>
        <td>${r.mine_name}</td>
        <td>${r.contractor_name || '<span class="text-muted">CIL Direct</span>'}</td>
        <td><span class="mono">${r.shift || 'GENERAL'}</span></td>
        <td>${r.overtime_hours > 0 ? `<strong>+${r.overtime_hours} hrs</strong>` : '-'}</td>
        <td class="mono">${r.record_date}</td>
        <td><span class="badge ${badgeClass}">${r.status}</span></td>
        <td>${r.marked_by_name || 'System / Auto'}</td>
      </tr>
    `;
  }).join('');
}

function openMarkModal() {
  document.getElementById('mark-modal').classList.remove('hidden');
}

function closeMarkModal() {
  document.getElementById('mark-modal').classList.add('hidden');
  document.getElementById('mark-form').reset();
}

async function handleMarkSubmit(e) {
  e.preventDefault();
  const mineId = parseInt(document.getElementById('att-mine').value);
  const workerIdStr = document.getElementById('att-worker').value;
  const workerId = workerIdStr ? parseInt(workerIdStr) : null;
  const date = document.getElementById('att-date').value;
  const shift = document.getElementById('att-shift').value;
  const status = document.getElementById('att-status').value;
  const ot = parseFloat(document.getElementById('att-ot').value) || 0.0;

  try {
    await API.post('/attendance', {
      mine_id: mineId,
      worker_id: workerId,
      record_date: date,
      shift: shift,
      status: status,
      overtime_hours: ot
    });

    showToast('Attendance recorded successfully', 'success');
    closeMarkModal();
    await applyFilters();
  } catch (err) {
    showError(err);
  }
}

function openWorkerModal() {
  document.getElementById('worker-modal').classList.remove('hidden');
}

function closeWorkerModal() {
  document.getElementById('worker-modal').classList.add('hidden');
  document.getElementById('worker-form').reset();
}

async function handleWorkerSubmit(e) {
  e.preventDefault();
  const mineId = parseInt(document.getElementById('w-mine').value);
  const code = document.getElementById('w-code').value.trim();
  const name = document.getElementById('w-name').value.trim();
  const desig = document.getElementById('w-desig').value.trim();
  const contStr = document.getElementById('w-contractor').value;
  const contractorId = contStr ? parseInt(contStr) : null;

  try {
    await API.post('/workers', {
      mine_id: mineId,
      worker_code: code,
      full_name: name,
      designation: desig,
      contractor_id: contractorId
    });

    showToast(`Worker ${name} registered successfully`, 'success');
    closeWorkerModal();
    await loadInitialDropdowns();
  } catch (err) {
    showError(err);
  }
}
