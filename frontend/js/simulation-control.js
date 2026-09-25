/**
 * simulation-control.js — Live demonstration simulation manager.
 */
document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('simulation-control.html');

  document.getElementById('btn-reset-demo').addEventListener('click', resetDemoDatabase);

  await loadCurrentMode();
});

async function loadCurrentMode() {
  const statusEl = document.getElementById('current-mode-status');
  statusEl.textContent = 'FETCHING...';

  try {
    const data = await API.get('/simulator/mode');
    const mode = data.mode || 'NORMAL_MODE';
    
    statusEl.textContent = mode.replace('_', ' ');

    // Reset card highlight classes
    document.getElementById('card-normal').classList.remove('active');
    document.getElementById('card-prod').classList.remove('active');
    document.getElementById('card-env').classList.remove('active');
    document.getElementById('card-safety').classList.remove('active');
    document.getElementById('card-high').classList.remove('active');

    // Highlight active card
    if (mode === 'NORMAL_MODE') document.getElementById('card-normal').classList.add('active');
    else if (mode === 'PRODUCTION_ANOMALY') document.getElementById('card-prod').classList.add('active');
    else if (mode === 'ENVIRONMENTAL_ALERT') document.getElementById('card-env').classList.add('active');
    else if (mode === 'SAFETY_INCIDENT') document.getElementById('card-safety').classList.add('active');
    else if (mode === 'HIGH_RISK_MODE') document.getElementById('card-high').classList.add('active');

  } catch (err) {
    statusEl.textContent = 'OFFLINE';
    showError(err);
  }
}

async function setSimMode(mode) {
  try {
    await API.post('/simulator/mode', { mode });
    showToast(`Simulation mode changed to ${mode.replace('_', ' ')}`, 'success');
    await loadCurrentMode();
  } catch (err) {
    showError(err);
  }
}

async function resetDemoDatabase() {
  if (!confirm('Are you sure you want to reset the demonstration database? This will purge all dynamically generated trial violations, incidents, and anomalies.')) return;
  
  const btn = document.getElementById('btn-reset-demo');
  btn.disabled = true;
  btn.textContent = 'Resetting Database...';

  try {
    await API.post('/simulator/reset');
    showToast('Database successfully restored to baseline seed data.', 'success');
    await loadCurrentMode();
  } catch (err) {
    showError(err);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Reset Demo Database';
  }
}
