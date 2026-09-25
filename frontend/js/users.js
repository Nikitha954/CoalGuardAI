/**
 * users.js — User Management & RBAC client logic.
 */
let allUsers = [];
let allRoles = [];
let allSubsidiaries = [];
let allMines = [];

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('users.html');

  document.getElementById('search-user').addEventListener('input', applyFilters);
  document.getElementById('filter-role').addEventListener('change', applyFilters);
  document.getElementById('filter-status').addEventListener('change', applyFilters);

  document.getElementById('btn-add-user').addEventListener('click', () => openUserModal());
  document.getElementById('user-modal-close').addEventListener('click', closeUserModal);
  document.getElementById('user-modal-cancel').addEventListener('click', closeUserModal);
  document.getElementById('user-form').addEventListener('submit', handleUserSubmit);

  await loadInitialResources();
  await loadUsers();
});

async function loadInitialResources() {
  try {
    const [roles, subsidiaries, mines] = await Promise.all([
      API.get('/roles').catch(() => [
        { id: 1, role_key: 'SUPER_ADMIN', role_name: 'Super Admin' },
        { id: 2, role_key: 'MINE_MANAGER', role_name: 'Mine Manager' },
        { id: 3, role_key: 'SAFETY_OFFICER', role_name: 'Safety Officer' },
        { id: 4, role_key: 'INSPECTOR', role_name: 'Inspector' },
        { id: 5, role_key: 'REGULATORY_OFFICER', role_name: 'Regulatory Officer' },
        { id: 6, role_key: 'CORPORATE_MANAGER', role_name: 'Corporate Manager' }
      ]),
      API.get('/subsidiaries').catch(() => []),
      API.get('/mines').catch(() => [])
    ]);

    allRoles = roles;
    allSubsidiaries = subsidiaries;
    allMines = mines;

    // Populate role options
    const filterRole = document.getElementById('filter-role');
    const uRole = document.getElementById('u-role');
    const roleOpts = allRoles.map(r => `<option value="${r.id}">${r.role_name} (${r.role_key})</option>`).join('');
    filterRole.innerHTML = `<option value="">All Roles</option>${roleOpts}`;
    uRole.innerHTML = roleOpts;

    // Populate subsidiary options
    const uSub = document.getElementById('u-subsidiary');
    const subOpts = allSubsidiaries.map(s => `<option value="${s.id}">${s.subsidiary_name} (${s.code})</option>`).join('');
    uSub.innerHTML = `<option value="">Corporate Headquarters / None</option>${subOpts}`;

    // Populate mine options
    const uMine = document.getElementById('u-mine');
    const mineOpts = allMines.map(m => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');
    uMine.innerHTML = `<option value="">HQ / Multi-Mine Overseer</option>${mineOpts}`;
  } catch (err) {
    showError(err);
  }
}

async function loadUsers() {
  const tbody = document.getElementById('users-table-body');
  tbody.innerHTML = `<tr><td colspan="9" class="state-panel">Loading user accounts...</td></tr>`;

  try {
    allUsers = await API.get('/users');
    updateKPIs(allUsers);
    renderUsersTable(allUsers);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel error">${err.message}</td></tr>`;
  }
}

function updateKPIs(items) {
  document.getElementById('kpi-total-u').textContent = items.length;
  document.getElementById('kpi-active-u').textContent = items.filter(u => u.status === 'ACTIVE').length;
  document.getElementById('kpi-inspectors-u').textContent = items.filter(u => u.role_key === 'INSPECTOR').length;
  document.getElementById('kpi-officers-u').textContent = items.filter(u => u.role_key === 'MINE_MANAGER' || u.role_key === 'SAFETY_OFFICER').length;
}

function applyFilters() {
  const query = document.getElementById('search-user').value.toLowerCase();
  const roleId = document.getElementById('filter-role').value;
  const status = document.getElementById('filter-status').value;

  const filtered = allUsers.filter(u => {
    if (roleId && String(u.role_id) !== roleId) return false;
    if (status && u.status !== status) return false;
    if (query) {
      const matchName = u.full_name?.toLowerCase().includes(query);
      const matchEmail = u.email?.toLowerCase().includes(query);
      const matchDesig = u.designation?.toLowerCase().includes(query);
      if (!matchName && !matchEmail && !matchDesig) return false;
    }
    return true;
  });

  renderUsersTable(filtered);
}

function renderUsersTable(items) {
  const tbody = document.getElementById('users-table-body');
  if (!items || items.length === 0) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel">No user accounts found.</td></tr>`;
    return;
  }

  tbody.innerHTML = items.map(u => {
    let roleBadge = 'badge-info';
    if (u.role_key === 'SUPER_ADMIN') roleBadge = 'badge-critical';
    else if (u.role_key === 'SAFETY_OFFICER' || u.role_key === 'MINE_MANAGER') roleBadge = 'badge-warning';

    return `
      <tr>
        <td class="mono font-semibold">#${u.id}</td>
        <td><strong>${u.full_name}</strong></td>
        <td>${u.email}</td>
        <td><span class="badge ${roleBadge}">${u.role_name}</span></td>
        <td>${u.subsidiary_name || '<span class="text-muted">CIL HQ</span>'}</td>
        <td>${u.mine_name || '<span class="text-muted">All Mines</span>'}</td>
        <td>${u.designation || '-'}</td>
        <td><span class="badge badge-${u.status === 'ACTIVE' ? 'active' : 'inactive'}">${u.status}</span></td>
        <td>
          <div style="display:flex; gap:6px;">
            <button class="btn btn-secondary btn-sm" onclick="openUserModal(${u.id})">Edit</button>
            ${u.status === 'ACTIVE' ? `<button class="btn btn-danger btn-sm" onclick="deactivateUser(${u.id})">Deactivate</button>` : ''}
          </div>
        </td>
      </tr>
    `;
  }).join('');
}

window.openUserModal = function(id) {
  const form = document.getElementById('user-form');
  form.reset();

  const pwdInput = document.getElementById('u-password');
  const pwdHint = document.getElementById('pwd-hint');

  if (id) {
    const u = allUsers.find(item => item.id === id);
    if (!u) return;
    document.getElementById('user-modal-title').textContent = 'Edit User Account';
    document.getElementById('u-id').value = u.id;
    document.getElementById('u-name').value = u.full_name;
    document.getElementById('u-email').value = u.email;
    document.getElementById('u-role').value = u.role_id;
    document.getElementById('u-subsidiary').value = u.subsidiary_id || '';
    document.getElementById('u-mine').value = u.mine_id || '';
    document.getElementById('u-desig').value = u.designation || '';
    document.getElementById('u-phone').value = u.phone || '';
    
    pwdInput.required = false;
    pwdHint.style.display = 'inline';
  } else {
    document.getElementById('user-modal-title').textContent = 'Create System User Account';
    document.getElementById('u-id').value = '';
    pwdInput.required = true;
    pwdHint.style.display = 'none';
  }

  document.getElementById('user-modal').classList.remove('hidden');
};

function closeUserModal() {
  document.getElementById('user-modal').classList.add('hidden');
}

async function handleUserSubmit(e) {
  e.preventDefault();
  const id = document.getElementById('u-id').value;
  const name = document.getElementById('u-name').value.trim();
  const email = document.getElementById('u-email').value.trim();
  const password = document.getElementById('u-password').value;
  const roleId = parseInt(document.getElementById('u-role').value);
  const subVal = document.getElementById('u-subsidiary').value;
  const mineVal = document.getElementById('u-mine').value;
  const desig = document.getElementById('u-desig').value.trim();
  const phone = document.getElementById('u-phone').value.trim();

  const payload = {
    full_name: name,
    email: email,
    role_id: roleId,
    subsidiary_id: subVal ? parseInt(subVal) : null,
    mine_id: mineVal ? parseInt(mineVal) : null,
    designation: desig,
    phone: phone
  };

  if (password) {
    payload.password = password;
  }

  try {
    if (id) {
      await API.put(`/users/${id}`, payload);
      showToast('User account updated successfully', 'success');
    } else {
      await API.post('/users', payload);
      showToast('User account created successfully', 'success');
    }
    closeUserModal();
    await loadUsers();
  } catch (err) {
    showError(err);
  }
}

window.deactivateUser = async function(id) {
  if (!confirm(`Are you sure you want to deactivate user account #${id}?`)) return;
  try {
    await API.delete(`/users/${id}`);
    showToast('User account deactivated', 'warning');
    await loadUsers();
  } catch (err) {
    showError(err);
  }
};
