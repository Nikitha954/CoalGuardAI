/**
 * production.js — Production & Heavy Machinery (HEMM) client logic.
 */
let allProdData = [];
let allMines = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('production.html');

  const user = AUTH.getUser();
  const canLog = ['MINE_MANAGER', 'SUPER_ADMIN'].includes(user.role_key);
  if (canLog) {
    document.getElementById('btn-log-prod').classList.remove('hidden');
  }

  const todayStr = new Date().toISOString().split('T')[0];
  document.getElementById('prod-date').value = todayStr;

  document.getElementById('filter-mine').addEventListener('change', loadProdData);
  document.getElementById('btn-log-prod').addEventListener('click', openProdModal);
  document.getElementById('prod-modal-close').addEventListener('click', closeProdModal);
  document.getElementById('prod-modal-cancel').addEventListener('click', closeProdModal);
  document.getElementById('prod-form').addEventListener('submit', handleProdSubmit);

  await loadMines();
  await loadProdData();
});

async function loadMines() {
  try {
    allMines = await API.get('/mines');
    const filterMine = document.getElementById('filter-mine');
    const prodMine = document.getElementById('prod-mine');

    const opts = allMines.map(m => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');
    filterMine.innerHTML = `<option value="">All Mines</option>${opts}`;
    prodMine.innerHTML = opts;
  } catch (err) {
    showError(err);
  }
}

async function loadProdData() {
  const tbody = document.getElementById('prod-table-body');
  tbody.innerHTML = `<tr><td colspan="10" class="state-panel">Loading production telemetry...</td></tr>`;

  try {
    const mineId = document.getElementById('filter-mine').value;
    let url = `/operational?`;
    if (mineId) url += `mine_id=${mineId}&`;

    allProdData = await API.get(url);
    updateKPIs(allProdData);
    renderProdTable(allProdData);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="10" class="state-panel error">${err.message}</td></tr>`;
  }
}

function updateKPIs(items) {
  if (!items || items.length === 0) return;

  const totalActual = items.reduce((acc, i) => acc + (i.production_actual || 0), 0);
  const totalTarget = items.reduce((acc, i) => acc + (i.production_target || 0), 0);
  const totalOB = items.reduce((acc, i) => acc + (i.overburden_stripped || 0), 0);
  const avgFleet = (items.reduce((acc, i) => acc + (i.equipment_availability || 0), 0) / items.length).toFixed(1);

  const fulfillPct = totalTarget > 0 ? Math.round((totalActual / totalTarget) * 100) : 100;

  document.getElementById('kpi-prod-total').textContent = `${(totalActual / 1000).toFixed(1)}k T`;
  document.getElementById('kpi-target-pct').textContent = `${fulfillPct}%`;
  document.getElementById('kpi-ob-total').textContent = `${(totalOB / 1000).toFixed(1)}k m³`;
  document.getElementById('kpi-hemm-avail').textContent = `${avgFleet}%`;

  if (fulfillPct < 85) document.getElementById('kpi-target-pct').className = 'kpi-value warning';
  if (parseFloat(avgFleet) < 75) document.getElementById('kpi-hemm-avail').className = 'kpi-value danger';
}

function renderProdTable(items) {
  const tbody = document.getElementById('prod-table-body');
  if (!items || items.length === 0) {
    tbody.innerHTML = `<tr><td colspan="10" class="state-panel">No operational records found.</td></tr>`;
    return;
  }

  tbody.innerHTML = items.map(p => {
    const fulfill = p.production_target > 0 ? Math.round((p.production_actual / p.production_target) * 100) : 100;
    const isUnderperforming = fulfill < 80;

    return `
      <tr>
        <td class="mono font-semibold">#${p.id}</td>
        <td><strong>${p.mine_name}</strong></td>
        <td class="mono">${p.record_date}</td>
        <td class="mono font-semibold">${p.production_actual?.toLocaleString()} T</td>
        <td class="mono hint">${p.production_target?.toLocaleString()} T</td>
        <td>
          <span class="badge ${isUnderperforming ? 'badge-warning' : 'badge-active'}">
            ${fulfill}%
          </span>
        </td>
        <td class="mono">${p.overburden_stripped?.toLocaleString() || '-'} m&sup3;</td>
        <td>${p.active_workers || '-'}</td>
        <td class="mono">${p.equipment_availability?.toFixed(1) || '-'}%</td>
        <td>
          <span class="badge ${isUnderperforming ? 'badge-critical' : 'badge-info'}" style="display:inline-flex; align-items:center; gap:3px;">
            ${isUnderperforming ? (window.ICONS ? ICONS.get('alert') : '') + ' DEFICIT' : (window.ICONS ? ICONS.get('check') : '') + ' OPTIMAL'}
          </span>
        </td>
      </tr>
    `;
  }).join('');
}

function openProdModal() {
  document.getElementById('prod-modal').classList.remove('hidden');
}

function closeProdModal() {
  document.getElementById('prod-modal').classList.add('hidden');
  document.getElementById('prod-form').reset();
}

async function handleProdSubmit(e) {
  e.preventDefault();
  const mineId = parseInt(document.getElementById('prod-mine').value);
  const date = document.getElementById('prod-date').value;
  const actual = parseFloat(document.getElementById('prod-actual').value);
  const target = parseFloat(document.getElementById('prod-target').value);
  const ob = parseFloat(document.getElementById('prod-ob').value) || 0;
  const workers = parseInt(document.getElementById('prod-workers').value) || 0;
  const fleet = parseFloat(document.getElementById('prod-fleet').value) || 0;

  try {
    await API.post('/operational', {
      mine_id: mineId,
      record_date: date,
      production_actual: actual,
      production_target: target,
      overburden_stripped: ob,
      active_workers: workers,
      equipment_availability: fleet
    });

    showToast('Operational telemetry logged successfully', 'success');
    closeProdModal();
    await loadProdData();
  } catch (err) {
    showError(err);
  }
}
