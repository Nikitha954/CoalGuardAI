/**
 * mines.js — Mine Management module.
 * SUPER_ADMIN can create/edit/deactivate mines; all roles can view.
 */
let allMines = [];
let allSubsidiaries = [];
let mapInstance = null;
let mapMarkers = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('mines.html');

  const user = AUTH.getUser();
  const canManage = user.role_key === 'SUPER_ADMIN';

  if (canManage) {
    document.getElementById('add-mine-btn').classList.remove('hidden');
  }

  document.getElementById('add-mine-btn').addEventListener('click', () => openMineModal());
  document.getElementById('modal-close').addEventListener('click', closeMineModal);
  document.getElementById('modal-cancel').addEventListener('click', closeMineModal);
  document.getElementById('mine-form').addEventListener('submit', handleMineFormSubmit);

  document.getElementById('filter-subsidiary').addEventListener('change', applyFilters);
  document.getElementById('filter-status').addEventListener('change', applyFilters);
  document.getElementById('search-input').addEventListener('input', applyFilters);

  await loadSubsidiaries();
  await loadMines();
  initMap();
});

async function loadSubsidiaries() {
  try {
    allSubsidiaries = await API.get('/subsidiaries');
    const subFilter = document.getElementById('filter-subsidiary');
    const subSelect = document.getElementById('mine-subsidiary');

    const options = allSubsidiaries.map((s) => `<option value="${s.id}">${s.name} (${s.code})</option>`).join('');
    subFilter.innerHTML = `<option value="">All Subsidiaries</option>${options}`;
    subSelect.innerHTML = options;
  } catch (err) {
    showError(err);
  }
}

async function loadMines() {
  const tbody = document.getElementById('mines-table-body');
  tbody.innerHTML = `<tr><td colspan="7" class="state-panel">Loading mines...</td></tr>`;

  try {
    allMines = await API.get('/mines');
    renderMinesTable(allMines);
    renderMapMarkers(allMines);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="7" class="state-panel error">${err.message}</td></tr>`;
  }
}

function applyFilters() {
  const subsidiary = document.getElementById('filter-subsidiary').value;
  const status = document.getElementById('filter-status').value;
  const search = document.getElementById('search-input').value.toLowerCase();

  const filtered = allMines.filter((m) => {
    if (subsidiary && String(m.subsidiary_id) !== subsidiary) return false;
    if (status && m.status !== status) return false;
    if (search && !m.mine_name.toLowerCase().includes(search) && !m.mine_code.toLowerCase().includes(search)) return false;
    return true;
  });

  renderMinesTable(filtered);
  renderMapMarkers(filtered);
}

function renderMinesTable(mines) {
  const tbody = document.getElementById('mines-table-body');
  const user = AUTH.getUser();
  const canManage = user.role_key === 'SUPER_ADMIN';

  if (mines.length === 0) {
    tbody.innerHTML = `<tr><td colspan="7" class="state-panel">No mines match the current filters.</td></tr>`;
    return;
  }

  tbody.innerHTML = mines
    .map(
      (m) => `
    <tr>
      <td><strong>${m.mine_name}</strong><div class="mono">${m.mine_code}</div></td>
      <td>${m.subsidiary_name}</td>
      <td>${m.district ? m.district + ', ' : ''}${m.state}</td>
      <td>${m.mine_type}</td>
      <td>${m.manager_name || '<span class="mono">Unassigned</span>'}</td>
      <td><span class="badge badge-${m.status === 'ACTIVE' ? 'active' : 'inactive'}">${m.status}</span></td>
      <td>
        ${
          canManage
            ? `<button class="btn btn-secondary btn-sm" onclick="openMineModal(${m.id})">Edit</button>
               ${m.status === 'ACTIVE' ? `<button class="btn btn-danger btn-sm" onclick="confirmDeactivate(${m.id})">Deactivate</button>` : ''}`
            : '<span class="mono">View only</span>'
        }
      </td>
    </tr>`
    )
    .join('');
}

function openMineModal(mineId) {
  const form = document.getElementById('mine-form');
  form.reset();
  document.getElementById('mine-id').value = '';
  document.getElementById('modal-title').textContent = 'Add Mine';

  if (mineId) {
    const mine = allMines.find((m) => m.id === mineId);
    if (mine) {
      document.getElementById('modal-title').textContent = 'Edit Mine';
      document.getElementById('mine-id').value = mine.id;
      document.getElementById('mine-name').value = mine.mine_name;
      document.getElementById('mine-code').value = mine.mine_code;
      document.getElementById('mine-subsidiary').value = mine.subsidiary_id;
      document.getElementById('mine-state').value = mine.state || '';
      document.getElementById('mine-district').value = mine.district || '';
      document.getElementById('mine-lat').value = mine.latitude;
      document.getElementById('mine-lng').value = mine.longitude;
      document.getElementById('mine-type').value = mine.mine_type;
      document.getElementById('mine-capacity').value = mine.production_capacity;
      document.getElementById('mine-status').value = mine.status;
    }
  }

  document.getElementById('mine-modal').classList.remove('hidden');
}

function closeMineModal() {
  document.getElementById('mine-modal').classList.add('hidden');
}

async function handleMineFormSubmit(e) {
  e.preventDefault();
  const submitBtn = document.getElementById('modal-save');
  submitBtn.disabled = true;
  submitBtn.textContent = 'Saving...';

  const mineId = document.getElementById('mine-id').value;
  const payload = {
    mine_name: document.getElementById('mine-name').value.trim(),
    mine_code: document.getElementById('mine-code').value.trim(),
    subsidiary_id: parseInt(document.getElementById('mine-subsidiary').value, 10),
    state: document.getElementById('mine-state').value.trim(),
    district: document.getElementById('mine-district').value.trim(),
    latitude: parseFloat(document.getElementById('mine-lat').value) || 0,
    longitude: parseFloat(document.getElementById('mine-lng').value) || 0,
    mine_type: document.getElementById('mine-type').value,
    production_capacity: parseFloat(document.getElementById('mine-capacity').value) || 0,
    status: document.getElementById('mine-status').value,
  };

  try {
    if (mineId) {
      await API.put(`/mines/${mineId}`, payload);
      showToast('Mine updated successfully', 'success');
    } else {
      await API.post('/mines', payload);
      showToast('Mine created successfully', 'success');
    }
    closeMineModal();
    await loadMines();
  } catch (err) {
    showError(err);
  } finally {
    submitBtn.disabled = false;
    submitBtn.textContent = 'Save Mine';
  }
}

function confirmDeactivate(mineId) {
  const mine = allMines.find((m) => m.id === mineId);
  if (!confirm(`Deactivate "${mine.mine_name}"? Historical records will be preserved.`)) return;

  API.del(`/mines/${mineId}`)
    .then(() => {
      showToast('Mine deactivated', 'success');
      loadMines();
    })
    .catch(showError);
}

// ---------------- GIS Map ----------------
function initMap() {
  const mapEl = document.getElementById('mines-map');
  if (!mapEl || typeof L === 'undefined') return;

  mapInstance = L.map('mines-map').setView([22.5, 82.8], 6);
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; OpenStreetMap contributors',
    maxZoom: 18,
  }).addTo(mapInstance);
}

function renderMapMarkers(mines) {
  if (!mapInstance) return;
  mapMarkers.forEach((m) => mapInstance.removeLayer(m));
  mapMarkers = [];

  mines.forEach((mine) => {
    if (!mine.latitude || !mine.longitude) return;

    let color = '#2E7D46'; // low/default
    if (mine.risk_classification === 'MEDIUM') color = '#B8860B';
    if (mine.risk_classification === 'HIGH') color = '#C05621';
    if (mine.risk_classification === 'CRITICAL') color = '#A32424';

    const marker = L.circleMarker([mine.latitude, mine.longitude], {
      radius: 9,
      fillColor: color,
      color: '#fff',
      weight: 2,
      fillOpacity: 0.9,
    }).addTo(mapInstance);

    const riskScoreVal = mine.risk_score !== undefined && mine.risk_score !== null ? mine.risk_score : 0;
    const compScoreVal = mine.compliance_score !== undefined && mine.compliance_score !== null ? mine.compliance_score.toFixed(1) : '100.0';

    marker.bindPopup(`
      <div style="font-size: 13px; line-height: 1.5;">
        <strong style="font-size: 14px;">${mine.mine_name}</strong><br/>
        <span class="hint" style="font-size:11.5px; display:block; margin-bottom:6px;">Code: ${mine.mine_code}</span>
        <strong>Compliance Score:</strong> ${compScoreVal}%<br/>
        <strong>Risk Score:</strong> ${riskScoreVal} (${mine.risk_classification})<br/>
        <strong>Open Violations:</strong> ${mine.open_violations_count ?? 0}<br/>
        <strong>Overdue Actions:</strong> ${mine.overdue_actions_count ?? 0}
      </div>
    `);

    mapMarkers.push(marker);
  });
}
