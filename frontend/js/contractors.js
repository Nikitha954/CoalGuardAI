/**
 * contractors.js — Contractor Lifecycle & Compliance client logic.
 */
let allContractors = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('contractors.html');

  const user = AUTH.getUser();
  const canManage = ['SUPER_ADMIN', 'MINE_MANAGER'].includes(user.role_key);
  if (canManage) {
    document.getElementById('btn-add-contractor').classList.remove('hidden');
  }

  document.getElementById('search-contractor').addEventListener('input', applyFilters);
  document.getElementById('filter-status').addEventListener('change', applyFilters);
  document.getElementById('btn-check-expiries').addEventListener('click', handleCheckExpiries);

  document.getElementById('btn-add-contractor').addEventListener('click', () => openContractorModal());
  document.getElementById('contractor-modal-close').addEventListener('click', closeContractorModal);
  document.getElementById('contractor-modal-cancel').addEventListener('click', closeContractorModal);
  document.getElementById('contractor-form').addEventListener('submit', handleContractorSubmit);

  document.getElementById('blacklist-modal-close').addEventListener('click', closeBlacklistModal);
  document.getElementById('blacklist-modal-cancel').addEventListener('click', closeBlacklistModal);
  document.getElementById('blacklist-form').addEventListener('submit', handleBlacklistSubmit);

  await loadContractors();
});

async function loadContractors() {
  const tbody = document.getElementById('contractors-table-body');
  tbody.innerHTML = `<tr><td colspan="9" class="state-panel">Loading contractors...</td></tr>`;

  try {
    allContractors = await API.get('/contractors');
    updateKPIs(allContractors);
    renderContractorsTable(allContractors);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel error">${err.message}</td></tr>`;
  }
}

function updateKPIs(items) {
  const now = new Date();
  const thirtyDaysOut = new Date(now.getTime() + (30 * 24 * 60 * 60 * 1000));

  let active = 0;
  let expiring = 0;
  let blacklisted = 0;

  items.forEach(c => {
    if (c.status === 'BLACKLISTED') {
      blacklisted++;
    } else if (c.status === 'ACTIVE') {
      active++;
      if (c.contract_end) {
        const endDate = new Date(c.contract_end);
        if (endDate > now && endDate <= thirtyDaysOut) {
          expiring++;
        }
      }
    }
  });

  document.getElementById('kpi-total-c').textContent = items.length;
  document.getElementById('kpi-active-c').textContent = active;
  document.getElementById('kpi-expiring-c').textContent = expiring;
  document.getElementById('kpi-blacklisted-c').textContent = blacklisted;
}

function applyFilters() {
  const query = document.getElementById('search-contractor').value.toLowerCase();
  const status = document.getElementById('filter-status').value;

  const filtered = allContractors.filter(c => {
    if (status && c.status !== status) return false;
    if (query) {
      const matchName = c.company_name?.toLowerCase().includes(query);
      const matchPerson = c.contact_person?.toLowerCase().includes(query);
      const matchEmail = c.email?.toLowerCase().includes(query);
      if (!matchName && !matchPerson && !matchEmail) return false;
    }
    return true;
  });

  renderContractorsTable(filtered);
}

function renderContractorsTable(items) {
  const tbody = document.getElementById('contractors-table-body');
  const user = AUTH.getUser();
  const canManage = ['SUPER_ADMIN', 'MINE_MANAGER', 'SAFETY_OFFICER'].includes(user.role_key);

  if (!items || items.length === 0) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel">No contractors found.</td></tr>`;
    return;
  }

  tbody.innerHTML = items.map(c => {
    let badgeClass = 'badge-active';
    if (c.status === 'EXPIRED') badgeClass = 'badge-warning';
    else if (c.status === 'BLACKLISTED') badgeClass = 'badge-critical';

    const period = `${c.contract_start ? c.contract_start.split('T')[0] : '-'} &rarr; ${c.contract_end ? c.contract_end.split('T')[0] : '-'}`;

    let actions = '-';
    if (canManage) {
      actions = `
        <div style="display:flex; gap:6px;">
          <button class="btn btn-secondary btn-sm" onclick="openContractorModal(${c.id})">Edit</button>
          ${c.status !== 'BLACKLISTED' ? `<button class="btn btn-danger btn-sm" onclick="openBlacklistModal(${c.id})">Blacklist</button>` : ''}
        </div>
      `;
    }

    return `
      <tr>
        <td class="mono font-semibold">#${c.id}</td>
        <td><strong>${c.company_name}</strong></td>
        <td>${c.contact_person || '-'}</td>
        <td>
          <div>${c.email || '-'}</div>
          <div class="mono hint">${c.phone || '-'}</div>
        </td>
        <td class="mono" style="font-size:12px;">${period}</td>
        <td>
          <a href="documents.html" class="mono hint" style="text-decoration:underline;">View Docs</a>
        </td>
        <td><span class="badge ${badgeClass}">${c.status}</span></td>
        <td><div style="max-width:220px; font-size:12px; color:var(--color-critical);">${c.blacklist_reason || '-'}</div></td>
        <td>${actions}</td>
      </tr>
    `;
  }).join('');
}

window.openContractorModal = function(id) {
  const form = document.getElementById('contractor-form');
  form.reset();

  if (id) {
    const c = allContractors.find(item => item.id === id);
    if (!c) return;
    document.getElementById('contractor-modal-title').textContent = 'Edit Contractor Details';
    document.getElementById('c-id').value = c.id;
    document.getElementById('c-name').value = c.company_name;
    document.getElementById('c-person').value = c.contact_person;
    document.getElementById('c-email').value = c.email;
    document.getElementById('c-phone').value = c.phone;
    document.getElementById('c-start').value = c.contract_start ? c.contract_start.split('T')[0] : '';
    document.getElementById('c-end').value = c.contract_end ? c.contract_end.split('T')[0] : '';
  } else {
    document.getElementById('contractor-modal-title').textContent = 'Register Contractor Agency';
    document.getElementById('c-id').value = '';
  }

  document.getElementById('contractor-modal').classList.remove('hidden');
};

function closeContractorModal() {
  document.getElementById('contractor-modal').classList.add('hidden');
}

async function handleContractorSubmit(e) {
  e.preventDefault();
  const id = document.getElementById('c-id').value;
  const payload = {
    company_name: document.getElementById('c-name').value.trim(),
    contact_person: document.getElementById('c-person').value.trim(),
    email: document.getElementById('c-email').value.trim(),
    phone: document.getElementById('c-phone').value.trim(),
    contract_start: document.getElementById('c-start').value,
    contract_end: document.getElementById('c-end').value
  };

  try {
    if (id) {
      await API.put(`/contractors/${id}`, payload);
      showToast('Contractor updated successfully', 'success');
    } else {
      await API.post('/contractors', payload);
      showToast('Contractor registered successfully', 'success');
    }
    closeContractorModal();
    await loadContractors();
  } catch (err) {
    showError(err);
  }
}

window.openBlacklistModal = function(id) {
  document.getElementById('bl-id').value = id;
  document.getElementById('bl-reason').value = '';
  document.getElementById('blacklist-modal').classList.remove('hidden');
};

function closeBlacklistModal() {
  document.getElementById('blacklist-modal').classList.add('hidden');
}

async function handleBlacklistSubmit(e) {
  e.preventDefault();
  const id = document.getElementById('bl-id').value;
  const reason = document.getElementById('bl-reason').value.trim();

  try {
    await API.put(`/contractors/${id}/blacklist`, { blacklist_reason: reason });
    showToast('Contractor blacklisted and compliance alert dispatched.', 'warning');
    closeBlacklistModal();
    await loadContractors();
  } catch (err) {
    showError(err);
  }
}

async function handleCheckExpiries() {
  const btn = document.getElementById('btn-check-expiries');
  btn.disabled = true;
  btn.textContent = 'Scanning Expiries...';

  try {
    const res = await API.post('/contractors/check-expiries', {});
    showToast(res.message || 'Expiries checked successfully', 'info');
    await loadContractors();
  } catch (err) {
    showError(err);
  } finally {
    const alertSvg = window.ICONS ? ICONS.get('alert') : '';
    btn.innerHTML = `${alertSvg} Scan Expiries &amp; Alert`;
  }
}
