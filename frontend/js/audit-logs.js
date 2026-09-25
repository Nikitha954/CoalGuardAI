/**
 * audit-logs.js — Cryptographic Audit Trail client logic.
 */
let allAuditLogs = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('audit-logs.html');

  document.getElementById('filter-action').addEventListener('change', loadAuditLogs);
  document.getElementById('filter-entity').addEventListener('change', loadAuditLogs);
  document.getElementById('btn-refresh').addEventListener('click', loadAuditLogs);
  document.getElementById('btn-verify-chain').addEventListener('click', verifyLedgerChain);

  document.getElementById('block-modal-close').addEventListener('click', closeBlockModal);
  document.getElementById('block-modal-cancel').addEventListener('click', closeBlockModal);

  await loadAuditLogs();
});

async function loadAuditLogs() {
  const tbody = document.getElementById('audit-table-body');
  tbody.innerHTML = `<tr><td colspan="8" class="state-panel">Loading cryptographic audit trail...</td></tr>`;

  try {
    const action = document.getElementById('filter-action').value;
    const entity = document.getElementById('filter-entity').value;

    let url = `/audit?limit=100&`;
    if (action) url += `action=${action}&`;
    if (entity) url += `entity_type=${entity}&`;

    allAuditLogs = await API.get(url);
    updateKPIs(allAuditLogs);
    renderAuditTable(allAuditLogs);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="8" class="state-panel error">${err.message}</td></tr>`;
  }
}

function updateKPIs(items) {
  document.getElementById('kpi-total-blocks').textContent = items.length;
  document.getElementById('kpi-auth-blocks').textContent = items.filter(l => l.action.includes('LOGIN') || l.entity_type === 'users').length;
  document.getElementById('kpi-comp-blocks').textContent = items.filter(l => l.entity_type === 'inspections' || l.entity_type === 'violations' || l.entity_type === 'contractors').length;
}

function renderAuditTable(items) {
  const tbody = document.getElementById('audit-table-body');
  if (!items || items.length === 0) {
    tbody.innerHTML = `<tr><td colspan="8" class="state-panel">No audit records found.</td></tr>`;
    return;
  }

  tbody.innerHTML = items.map((l) => {
    const timeStr = l.created_at ? new Date(l.created_at).toLocaleString('en-IN') : '-';
    const shortHash = l.hash ? l.hash.substring(0, 10) + '...' + l.hash.substring(58) : '0000000000...';

    let actionBadge = 'badge-info';
    if (l.action.includes('CRITICAL') || l.action.includes('BLACKLIST') || l.action.includes('EMERGENCY')) {
      actionBadge = 'badge-critical';
    } else if (l.action.includes('CREATE') || l.action.includes('RECORD')) {
      actionBadge = 'badge-active';
    } else if (l.action.includes('UPDATE') || l.action.includes('ASSIGN')) {
      actionBadge = 'badge-warning';
    }

    return `
      <tr>
        <td class="mono font-semibold">#${l.id}</td>
        <td class="mono" style="font-size:12px;">${timeStr}</td>
        <td><strong>${l.user_name || 'System / Auto'}</strong></td>
        <td><span class="badge ${actionBadge}">${l.action}</span></td>
        <td><span class="mono hint">${l.entity_type}${l.entity_id ? ` #${l.entity_id}` : ''}</span></td>
        <td class="mono" style="font-size:12px;">${l.ip_address || '127.0.0.1'}</td>
        <td style="max-width:240px; font-size:12px;">${l.details || '-'}</td>
        <td>
          <button class="btn btn-secondary btn-sm mono" style="font-size:11px; padding:3px 6px; display:inline-flex; align-items:center; gap:4px;" onclick="openBlockModal(${l.id})">
            ${window.ICONS ? ICONS.get('link') : ''} ${shortHash}
          </button>
        </td>
      </tr>
    `;
  }).join('');
}

async function verifyLedgerChain() {
  const btn = document.getElementById('btn-verify-chain');
  const resultsDiv = document.getElementById('verification-results');
  const badge = document.getElementById('verify-status-badge');
  const meta = document.getElementById('verify-meta');
  const details = document.getElementById('verify-details');

  btn.disabled = true;
  btn.innerHTML = 'Verifying Merkle Hashes...';

  try {
    const res = await API.get('/audit/verify');
    resultsDiv.style.display = 'block';

    if (res.valid) {
      const shieldCheckSvg = window.ICONS ? ICONS.get('shield_check') : '';
      badge.innerHTML = `<span style="color:#10b981; display:inline-flex; align-items:center; gap:6px;">${shieldCheckSvg} CHAIN INTEGRITY VERIFIED (TAMPER-FREE)</span>`;
      meta.textContent = `${res.total_records} chained blocks validated`;
      details.innerHTML = `All block SHA-256 signatures are intact and sequentially valid from genesis block. Latest chain block hash: <span class="mono">${res.latest_hash}</span>`;
      showToast('SHA-256 audit ledger is 100% cryptographically verified.', 'success');
    } else {
      const alertSvg = window.ICONS ? ICONS.get('alert') : '';
      badge.innerHTML = `<span style="color:#ef4444; display:inline-flex; align-items:center; gap:6px;">${alertSvg} CRITICAL: LEDGER TAMPER DETECTED AT BLOCK #${res.broken_at_id}</span>`;
      meta.textContent = `Verification halted at ID ${res.broken_at_id}`;
      details.innerHTML = `Computed hash does not match stored block signature: <br/><strong>Expected:</strong> <span class="mono">${res.expected_hash}</span><br/><strong>Found:</strong> <span class="mono">${res.stored_hash}</span>`;
      showToast('Warning: Cryptographic ledger signature mismatch detected!', 'error');
    }
  } catch (err) {
    showError(err);
  } finally {
    btn.disabled = false;
    const lockSvg = window.ICONS ? ICONS.get('lock') : '';
    btn.innerHTML = `${lockSvg} Verify Ledger Integrity`;
  }
}

window.openBlockModal = function(id) {
  const block = allAuditLogs.find(l => l.id === id);
  if (!block) return;

  document.getElementById('modal-prev-hash').textContent = block.prev_hash || '0000000000000000000000000000000000000000000000000000000000000000';
  document.getElementById('modal-curr-hash').textContent = block.hash || 'N/A';
  
  const payload = {
    id: block.id,
    user_id: block.user_id,
    user_name: block.user_name,
    action: block.action,
    entity_type: block.entity_type,
    entity_id: block.entity_id,
    ip_address: block.ip_address,
    details: block.details,
    created_at: block.created_at
  };

  document.getElementById('modal-block-payload').textContent = JSON.stringify(payload, null, 2);
  document.getElementById('block-modal').classList.remove('hidden');
};

function closeBlockModal() {
  document.getElementById('block-modal').classList.add('hidden');
}
