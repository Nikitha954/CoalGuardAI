/**
 * reports.js — Statutory reports client logic.
 * Handles serializing report parameters, requesting blobs, and triggering client-side downloads.
 */
document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('reports.html');

  document.getElementById('report-form').addEventListener('submit', handleReportSubmit);

  await loadMines();
  await loadReportHistory();
});

async function loadMines() {
  try {
    const mines = await API.get('/mines');
    const selectMine = document.getElementById('rep-mine');
    mines.forEach(m => {
      const opt = document.createElement('option');
      opt.value = m.id;
      opt.textContent = m.mine_name;
      selectMine.appendChild(opt);
    });
  } catch (err) {
    showError(err);
  }
}

async function handleReportSubmit(e) {
  e.preventDefault();
  const btn = document.getElementById('btn-generate');
  btn.disabled = true;
  btn.textContent = 'Generating...';

  const type = document.getElementById('rep-type').value;
  const format = document.getElementById('rep-format').value;
  const mineIdVal = document.getElementById('rep-mine').value;

  const payload = {
    report_type: type,
    format: format
  };
  if (mineIdVal) {
    payload.mine_id = parseInt(mineIdVal, 10);
  }

  const token = localStorage.getItem('cg_token');
  const API_BASE = window.APP_CONFIG?.API_BASE_URL || '/api';

  try {
    const res = await fetch(`${API_BASE}/reports/generate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify(payload)
    });

    if (res.status !== 200) {
      const errJSON = await res.json().catch(() => ({}));
      throw new Error(errJSON.message || 'Failed to generate report file.');
    }

    const blob = await res.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `report_${type.toLowerCase()}_${Date.now()}.${format.toLowerCase()}`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    window.URL.revokeObjectURL(url);
    
    showToast('Report generated and downloaded successfully.', 'success');
    await loadReportHistory();
  } catch (err) {
    showError(err);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Generate & Download';
  }
}

async function loadReportHistory() {
  const tbody = document.getElementById('report-history-body');
  if (!tbody) return;

  try {
    const list = await API.get('/reports');
    if (!list || list.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="state-panel">No archived reports yet.</td></tr>';
      return;
    }

    tbody.innerHTML = list.map(r => {
      const dateStr = r.generated_at ? new Date(r.generated_at).toLocaleString('en-IN') : '-';
      return `
        <tr>
          <td><strong>${r.report_title || r.report_type}</strong></td>
          <td><span class="badge ${r.format === 'PDF' ? 'badge-critical' : 'badge-active'}">${r.format}</span></td>
          <td>${r.generated_by_name || 'System'}</td>
          <td class="mono" style="font-size:12px;">${dateStr}</td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="state-panel error">${err.message}</td></tr>`;
  }
}

