/**
 * compliance.js — Compliance Rules Master Management.
 * Only SUPER_ADMIN (Admin) can create, edit, or deactivate rules.
 * Other roles (Regulatory Officer, Safety Officer, Mine Manager, Inspector) can view rules.
 */
let allRules = [];
let allCategories = [];
let allMines = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('compliance.html');

  const user = AUTH.getUser();
  const canManage = user && user.role_key === 'SUPER_ADMIN';

  // Only show Add button to Admins
  const addBtn = document.getElementById('btn-add-rule');
  if (canManage && addBtn) {
    addBtn.classList.remove('hidden');
    addBtn.addEventListener('click', () => openRuleModal());
  }

  // Filter & Search events
  document.getElementById('search-rule').addEventListener('input', applyFilters);
  document.getElementById('filter-category').addEventListener('change', applyFilters);
  document.getElementById('filter-severity').addEventListener('change', applyFilters);
  document.getElementById('filter-status').addEventListener('change', applyFilters);

  // Modal events
  document.getElementById('rule-modal-close').addEventListener('click', closeRuleModal);
  document.getElementById('rule-modal-cancel').addEventListener('click', closeRuleModal);
  document.getElementById('rule-form').addEventListener('submit', handleRuleSubmit);

  await loadInitialResources();
  await loadRules();
});

async function loadInitialResources() {
  try {
    const [categories, mines] = await Promise.all([
      API.get('/compliance/categories').catch(() => []),
      API.get('/mines').catch(() => [])
    ]);

    allCategories = categories;
    allMines = mines;

    // Populate category dropdown in filter & modal
    const filterCategory = document.getElementById('filter-category');
    const modalCategory = document.getElementById('r-category');

    filterCategory.innerHTML =
      '<option value="">All Categories</option>' +
      allCategories.map((c) => `<option value="${c.name}">${c.name}</option>`).join('');

    modalCategory.innerHTML =
      '<option value="" disabled selected>Select Category</option>' +
      allCategories.map((c) => `<option value="${c.id}">${c.name}</option>`).join('');

    // Populate mine options in modal
    const modalMine = document.getElementById('r-mine');
    modalMine.innerHTML =
      '<option value="">All Mines (Universal Rule)</option>' +
      allMines.map((m) => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');
  } catch (err) {
    showError(err);
  }
}

async function loadRules() {
  const tbody = document.getElementById('rules-table-body');
  tbody.innerHTML = `<tr><td colspan="9" class="state-panel">Loading compliance rules...</td></tr>`;

  try {
    allRules = await API.get('/compliance/rules');
    updateKPIs(allRules);
    renderRules(allRules);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel error">${err.message}</td></tr>`;
  }
}

function updateKPIs(rules) {
  document.getElementById('kpi-total-rules').textContent = rules.length;
  document.getElementById('kpi-active-rules').textContent = rules.filter((r) => r.status === 'ACTIVE').length;
  document.getElementById('kpi-critical-rules').textContent = rules.filter(
    (r) => r.severity === 'CRITICAL' || r.severity === 'HIGH'
  ).length;
  document.getElementById('kpi-categories').textContent = allCategories.length || '--';
}

function applyFilters() {
  const query = document.getElementById('search-rule').value.toLowerCase().trim();
  const category = document.getElementById('filter-category').value;
  const severity = document.getElementById('filter-severity').value;
  const status = document.getElementById('filter-status').value;

  const filtered = allRules.filter((r) => {
    if (category && r.category !== category) return false;
    if (severity && r.severity !== severity) return false;
    if (status && r.status !== status) return false;
    if (query) {
      const matchCode = r.rule_code?.toLowerCase().includes(query);
      const matchTitle = r.title?.toLowerCase().includes(query);
      const matchDesc = r.description?.toLowerCase().includes(query);
      const matchDept = r.responsible_dept?.toLowerCase().includes(query);
      const matchCat = r.category?.toLowerCase().includes(query);
      if (!matchCode && !matchTitle && !matchDesc && !matchDept && !matchCat) return false;
    }
    return true;
  });

  renderRules(filtered);
}

function renderRules(rules) {
  const tbody = document.getElementById('rules-table-body');
  const user = AUTH.getUser();
  const canManage = user && user.role_key === 'SUPER_ADMIN';

  if (!rules || rules.length === 0) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel">No compliance rules match the current filters.</td></tr>`;
    return;
  }

  const severityClass = { LOW: 'low', MEDIUM: 'medium', HIGH: 'high', CRITICAL: 'critical' };

  tbody.innerHTML = rules
    .map((r) => {
      let actionsHtml = '';
      if (canManage) {
        actionsHtml = `
          <div style="display:flex; gap:6px;">
            <button class="btn btn-secondary btn-sm" onclick="openRuleModal(${r.id})">Edit</button>
            ${
              r.status === 'ACTIVE'
                ? `<button class="btn btn-danger btn-sm" onclick="deactivateRule(${r.id})">Deactivate</button>`
                : `<button class="btn btn-secondary btn-sm" onclick="activateRule(${r.id})">Activate</button>`
            }
          </div>
        `;
      } else {
        actionsHtml = `
          <button class="btn btn-secondary btn-sm" onclick="viewRuleDetails(${r.id})">View Details</button>
        `;
      }

      return `
      <tr>
        <td class="mono font-semibold" style="color:var(--color-brand); font-weight:600;">${r.rule_code}</td>
        <td>
          <strong>${r.title}</strong>
          <div style="color:var(--color-ink-muted); font-size:12px; margin-top:2px; max-width:320px; line-height:1.4;">${r.description || '-'}</div>
        </td>
        <td><span class="badge badge-info">${r.category}</span></td>
        <td>${r.frequency}</td>
        <td><span class="badge badge-${severityClass[r.severity] || 'medium'}">${r.severity}</span></td>
        <td>${r.responsible_dept || '-'}</td>
        <td class="mono">${r.due_period_days ? r.due_period_days + ' d' : '-'}</td>
        <td><span class="badge badge-${r.status === 'ACTIVE' ? 'active' : 'inactive'}">${r.status}</span></td>
        <td>${actionsHtml}</td>
      </tr>`;
    })
    .join('');
}

function setFormDisabled(disabled) {
  const form = document.getElementById('rule-form');
  const elements = form.querySelectorAll('input, select, textarea');
  elements.forEach((el) => {
    el.disabled = disabled;
  });
}

window.openRuleModal = function (id) {
  const user = AUTH.getUser();
  if (!user || user.role_key !== 'SUPER_ADMIN') {
    showToast('Permission denied: Only Administrators can create or modify compliance rules.', 'error');
    return;
  }

  const form = document.getElementById('rule-form');
  form.reset();
  setFormDisabled(false);
  document.getElementById('rule-modal-submit').style.display = 'inline-block';

  if (id) {
    const r = allRules.find((item) => item.id === id);
    if (!r) return;
    document.getElementById('rule-modal-title').textContent = 'Edit Compliance Rule — ' + r.rule_code;
    document.getElementById('r-id').value = r.id;
    document.getElementById('r-code').value = r.rule_code;
    document.getElementById('r-title').value = r.title;
    document.getElementById('r-dept').value = r.responsible_dept || '';
    document.getElementById('r-severity').value = r.severity || 'MEDIUM';
    document.getElementById('r-frequency').value = r.frequency || 'MONTHLY';
    document.getElementById('r-due-days').value = r.due_period_days || 30;
    document.getElementById('r-status').value = r.status || 'ACTIVE';
    document.getElementById('r-description').value = r.description || '';

    // Match category ID
    if (r.category_id) {
      document.getElementById('r-category').value = r.category_id;
    } else {
      const matchCat = allCategories.find((c) => c.name === r.category);
      if (matchCat) document.getElementById('r-category').value = matchCat.id;
    }

    document.getElementById('r-mine').value = r.applicable_mine_id || '';
  } else {
    document.getElementById('rule-modal-title').textContent = 'Create Compliance Rule';
    document.getElementById('r-id').value = '';
    document.getElementById('r-severity').value = 'MEDIUM';
    document.getElementById('r-frequency').value = 'MONTHLY';
    document.getElementById('r-due-days').value = '30';
    document.getElementById('r-status').value = 'ACTIVE';
  }

  document.getElementById('rule-modal').classList.remove('hidden');
};

window.viewRuleDetails = function (id) {
  const r = allRules.find((item) => item.id === id);
  if (!r) return;

  const form = document.getElementById('rule-form');
  form.reset();

  document.getElementById('rule-modal-title').textContent = 'Compliance Rule Details — ' + r.rule_code;
  document.getElementById('r-id').value = r.id;
  document.getElementById('r-code').value = r.rule_code;
  document.getElementById('r-title').value = r.title;
  document.getElementById('r-dept').value = r.responsible_dept || '';
  document.getElementById('r-severity').value = r.severity || 'MEDIUM';
  document.getElementById('r-frequency').value = r.frequency || 'MONTHLY';
  document.getElementById('r-due-days').value = r.due_period_days || 30;
  document.getElementById('r-status').value = r.status || 'ACTIVE';
  document.getElementById('r-description').value = r.description || '';

  if (r.category_id) {
    document.getElementById('r-category').value = r.category_id;
  } else {
    const matchCat = allCategories.find((c) => c.name === r.category);
    if (matchCat) document.getElementById('r-category').value = matchCat.id;
  }
  document.getElementById('r-mine').value = r.applicable_mine_id || '';

  setFormDisabled(true);
  document.getElementById('rule-modal-submit').style.display = 'none';
  document.getElementById('rule-modal').classList.remove('hidden');
};

function closeRuleModal() {
  document.getElementById('rule-modal').classList.add('hidden');
  setFormDisabled(false);
}

async function handleRuleSubmit(e) {
  e.preventDefault();
  const user = AUTH.getUser();
  if (!user || user.role_key !== 'SUPER_ADMIN') {
    showToast('Permission denied: Only Administrators can create or edit compliance rules.', 'error');
    return;
  }

  const id = document.getElementById('r-id').value;
  const code = document.getElementById('r-code').value.trim().toUpperCase();
  const title = document.getElementById('r-title').value.trim();
  const catId = parseInt(document.getElementById('r-category').value, 10);
  const dept = document.getElementById('r-dept').value.trim();
  const severity = document.getElementById('r-severity').value;
  const frequency = document.getElementById('r-frequency').value;
  const dueDays = parseInt(document.getElementById('r-due-days').value, 10) || 30;
  const mineVal = document.getElementById('r-mine').value;
  const status = document.getElementById('r-status').value;
  const description = document.getElementById('r-description').value.trim();

  if (!code || !title || !catId) {
    showToast('Rule Code, Title, and Category are required.', 'error');
    return;
  }

  const payload = {
    rule_code: code,
    title: title,
    category_id: catId,
    responsible_dept: dept,
    severity: severity,
    frequency: frequency,
    due_period_days: dueDays,
    applicable_mine_id: mineVal ? parseInt(mineVal, 10) : null,
    status: status,
    description: description
  };

  const submitBtn = document.getElementById('rule-modal-submit');
  submitBtn.disabled = true;
  submitBtn.textContent = 'Saving...';

  try {
    if (id) {
      await API.put(`/compliance/rules/${id}`, payload);
      showToast(`Compliance rule ${code} updated successfully`, 'success');
    } else {
      await API.post('/compliance/rules', payload);
      showToast(`Compliance rule ${code} created successfully`, 'success');
    }
    closeRuleModal();
    await loadRules();
  } catch (err) {
    showError(err);
  } finally {
    submitBtn.disabled = false;
    submitBtn.textContent = 'Save Compliance Rule';
  }
}

window.deactivateRule = async function (id) {
  const user = AUTH.getUser();
  if (!user || user.role_key !== 'SUPER_ADMIN') {
    showToast('Permission denied: Only Administrators can deactivate compliance rules.', 'error');
    return;
  }

  const r = allRules.find((item) => item.id === id);
  const ruleName = r ? r.rule_code : `#${id}`;

  if (!confirm(`Are you sure you want to deactivate compliance rule ${ruleName}?`)) return;

  try {
    await API.delete(`/compliance/rules/${id}`);
    showToast(`Compliance rule ${ruleName} deactivated`, 'warning');
    await loadRules();
  } catch (err) {
    showError(err);
  }
};

window.activateRule = async function (id) {
  const user = AUTH.getUser();
  if (!user || user.role_key !== 'SUPER_ADMIN') {
    showToast('Permission denied: Only Administrators can activate compliance rules.', 'error');
    return;
  }

  const r = allRules.find((item) => item.id === id);
  if (!r) return;

  const payload = {
    rule_code: r.rule_code,
    title: r.title,
    category_id: r.category_id,
    responsible_dept: r.responsible_dept,
    severity: r.severity,
    frequency: r.frequency,
    due_period_days: r.due_period_days,
    applicable_mine_id: r.applicable_mine_id,
    status: 'ACTIVE',
    description: r.description
  };

  try {
    await API.put(`/compliance/rules/${id}`, payload);
    showToast(`Compliance rule ${r.rule_code} activated`, 'success');
    await loadRules();
  } catch (err) {
    showError(err);
  }
};
