/**
 * environmental.js — Environmental Telemetry & CPCB Compliance client logic.
 */
let allEnvData = [];
let allMines = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('environmental.html');

  const user = AUTH.getUser();
  const canLog = ['SAFETY_OFFICER', 'INSPECTOR', 'SUPER_ADMIN'].includes(user.role_key);
  if (canLog) {
    document.getElementById('btn-log-env').classList.remove('hidden');
  }

  const todayStr = new Date().toISOString().split('T')[0];
  document.getElementById('env-date').value = todayStr;

  document.getElementById('filter-mine').addEventListener('change', loadEnvData);
  document.getElementById('btn-log-env').addEventListener('click', openEnvModal);
  document.getElementById('env-modal-close').addEventListener('click', closeEnvModal);
  document.getElementById('env-modal-cancel').addEventListener('click', closeEnvModal);
  document.getElementById('env-form').addEventListener('submit', handleEnvSubmit);

  await loadMines();
  await loadEnvData();
});

async function loadMines() {
  try {
    allMines = await API.get('/mines');
    const filterMine = document.getElementById('filter-mine');
    const envMine = document.getElementById('env-mine');

    const opts = allMines.map(m => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');
    filterMine.innerHTML = `<option value="">All Mines</option>${opts}`;
    envMine.innerHTML = opts;
  } catch (err) {
    showError(err);
  }
}

async function loadEnvData() {
  const tbody = document.getElementById('env-table-body');
  tbody.innerHTML = `<tr><td colspan="10" class="state-panel">Loading environmental telemetry...</td></tr>`;

  try {
    const mineId = document.getElementById('filter-mine').value;
    let url = `/environmental?`;
    if (mineId) url += `mine_id=${mineId}&`;

    allEnvData = await API.get(url);
    updateKPIs(allEnvData);
    renderEnvTable(allEnvData);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="10" class="state-panel error">${err.message}</td></tr>`;
  }
}

function updateKPIs(items) {
  if (!items || items.length === 0) return;

  const avgPM10 = (items.reduce((acc, i) => acc + (i.air_quality_pm10 || 0), 0) / items.length).toFixed(1);
  const avgPH = (items.reduce((acc, i) => acc + (i.water_ph || 7.0), 0) / items.length).toFixed(1);
  const maxCH4 = Math.max(...items.map(i => i.methane_level || 0)).toFixed(2);
  const avgNoise = (items.reduce((acc, i) => acc + (i.noise_level || 0), 0) / items.length).toFixed(1);

  document.getElementById('kpi-pm10').textContent = avgPM10;
  document.getElementById('kpi-ph').textContent = avgPH;
  document.getElementById('kpi-ch4').textContent = `${maxCH4}%`;
  document.getElementById('kpi-noise').textContent = `${avgNoise} dB`;

  // Update color warnings
  if (parseFloat(avgPM10) > 100) document.getElementById('kpi-pm10').className = 'kpi-value warning';
  if (parseFloat(avgPH) < 6.5 || parseFloat(avgPH) > 8.5) document.getElementById('kpi-ph').className = 'kpi-value danger';
  if (parseFloat(maxCH4) > 1.25) document.getElementById('kpi-ch4').className = 'kpi-value danger';
}

function renderEnvTable(items) {
  const tbody = document.getElementById('env-table-body');
  if (!items || items.length === 0) {
    tbody.innerHTML = `<tr><td colspan="10" class="state-panel">No telemetry logs found.</td></tr>`;
    return;
  }

  tbody.innerHTML = items.map(d => {
    const isBreach = d.air_quality_pm10 > 100 || d.air_quality_pm25 > 60 || d.water_ph < 6.5 || d.water_ph > 8.5 || d.noise_level > 75 || d.methane_level > 1.25;

    return `
      <tr>
        <td class="mono font-semibold">#${d.id}</td>
        <td><strong>${d.mine_name}</strong></td>
        <td class="mono">${d.record_date}</td>
        <td class="mono ${d.air_quality_pm10 > 100 ? 'text-critical' : ''}">${d.air_quality_pm10?.toFixed(1) || '-'}</td>
        <td class="mono ${d.air_quality_pm25 > 60 ? 'text-critical' : ''}">${d.air_quality_pm25?.toFixed(1) || '-'}</td>
        <td class="mono">${d.air_quality_so2?.toFixed(1) || '-'}/${d.air_quality_no2?.toFixed(1) || '-'}</td>
        <td class="mono ${(d.water_ph < 6.5 || d.water_ph > 8.5) ? 'text-critical' : ''}">${d.water_ph?.toFixed(1) || '-'}</td>
        <td class="mono ${d.noise_level > 75 ? 'text-critical' : ''}">${d.noise_level?.toFixed(1) || '-'}</td>
        <td class="mono ${d.methane_level > 1.25 ? 'text-critical' : ''}">${d.methane_level ? d.methane_level.toFixed(2) + '%' : '0.00%'}</td>
        <td>
          <span class="badge ${isBreach ? 'badge-critical' : 'badge-active'}" style="display:inline-flex; align-items:center; gap:3px;">
            ${isBreach ? (window.ICONS ? ICONS.get('alert') : '') + ' BREACH' : (window.ICONS ? ICONS.get('check') : '') + ' COMPLIANT'}
          </span>
        </td>
      </tr>
    `;
  }).join('');
}

function openEnvModal() {
  document.getElementById('env-modal').classList.remove('hidden');
}

function closeEnvModal() {
  document.getElementById('env-modal').classList.add('hidden');
  document.getElementById('env-form').reset();
}

async function handleEnvSubmit(e) {
  e.preventDefault();
  const mineId = parseInt(document.getElementById('env-mine').value);
  const date = document.getElementById('env-date').value;
  const pm10 = parseFloat(document.getElementById('env-pm10').value);
  const pm25 = parseFloat(document.getElementById('env-pm25').value);
  const so2 = parseFloat(document.getElementById('env-so2').value) || 0;
  const no2 = parseFloat(document.getElementById('env-no2').value) || 0;
  const ph = parseFloat(document.getElementById('env-ph').value);
  const noise = parseFloat(document.getElementById('env-noise').value);
  const ch4 = parseFloat(document.getElementById('env-ch4').value) || 0;

  try {
    await API.post('/environmental', {
      mine_id: mineId,
      record_date: date,
      air_quality_pm10: pm10,
      air_quality_pm25: pm25,
      air_quality_so2: so2,
      air_quality_no2: no2,
      water_ph: ph,
      noise_level: noise,
      methane_level: ch4
    });

    showToast('Environmental reading logged successfully', 'success');
    closeEnvModal();
    await loadEnvData();
  } catch (err) {
    showError(err);
  }
}
