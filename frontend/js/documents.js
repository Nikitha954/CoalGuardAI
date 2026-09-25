/**
 * documents.js — Statutory Documents & OCR Governance Workflow Client Logic.
 * 
 * Access Control:
 * 1. Super Admin: Upload, View, Process OCR, Review, Approve, Regulatory Verify, Flag, Edit, Delete all documents.
 * 2. Mine Manager: Upload mine documents, Run OCR, Review & Approve mine-level documents, Monitor compliance.
 * 3. Safety Officer: Upload safety docs/certificates, Run OCR, Review findings, Create corrective actions.
 * 4. Inspector: Upload field documents/reports, Run OCR, View assigned inspections (Cannot approve/delete).
 * 5. Corporate Manager: Multi-mine monitoring, reporting, and expiry tracking.
 * 6. Regulatory Officer: Upload statutory docs, Run OCR, Review evidence, Verify regulatory compliance & flag violations.
 */
let allDocuments = [];
let allMines = [];
let allContractors = [];
let currentStageFilter = '';

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('documents.html');

  const user = AUTH.getUser();
  setupRoleUI(user);

  // Stage Filter Tabs
  document.querySelectorAll('.stage-tab').forEach((tab) => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.stage-tab').forEach((t) => t.classList.remove('active'));
      tab.classList.add('active');
      currentStageFilter = tab.dataset.stage;
      applyFilters();
    });
  });

  // Toolbar Filter & Search Events
  document.getElementById('search-doc').addEventListener('input', applyFilters);
  document.getElementById('filter-status').addEventListener('change', applyFilters);
  document.getElementById('filter-mine').addEventListener('change', applyFilters);

  // Upload Modal Events
  const uploadBtn = document.getElementById('upload-doc-btn');
  if (uploadBtn) uploadBtn.addEventListener('click', openUploadModal);
  document.getElementById('modal-close').addEventListener('click', closeUploadModal);
  document.getElementById('modal-cancel').addEventListener('click', closeUploadModal);
  document.getElementById('document-form').addEventListener('submit', handleUploadSubmit);

  // Review Modal Events
  document.getElementById('review-modal-close').addEventListener('click', closeReviewModal);
  document.getElementById('review-modal-cancel').addEventListener('click', closeReviewModal);
  document.getElementById('review-form').addEventListener('submit', handleReviewSubmit);

  // Regulatory Verification Modal Events
  document.getElementById('regulatory-modal-close').addEventListener('click', closeRegulatoryModal);
  document.getElementById('regulatory-modal-cancel').addEventListener('click', closeRegulatoryModal);
  document.getElementById('regulatory-form').addEventListener('submit', handleRegulatorySubmit);

  // Flag Violation Modal Events
  document.getElementById('flag-violation-close').addEventListener('click', closeFlagViolationModal);
  document.getElementById('flag-violation-cancel').addEventListener('click', closeFlagViolationModal);
  document.getElementById('flag-violation-form').addEventListener('submit', handleFlagViolationSubmit);

  // Edit Modal Events
  document.getElementById('edit-modal-close').addEventListener('click', closeEditModal);
  document.getElementById('edit-modal-cancel').addEventListener('click', closeEditModal);
  document.getElementById('edit-document-form').addEventListener('submit', handleEditSubmit);

  // OCR Modal Events
  document.getElementById('ocr-close').addEventListener('click', closeOCRModal);
  document.getElementById('ocr-back').addEventListener('click', closeOCRModal);
  document.getElementById('ocr-copy-btn').addEventListener('click', copyOCRTranscript);

  // Load resources
  await loadMines();
  await loadContractorsDropdown();
  await loadDocuments();
});

function setupRoleUI(user) {
  if (!user) return;
  const uploadBtn = document.getElementById('upload-doc-btn');
  const canUpload = ['SUPER_ADMIN', 'MINE_MANAGER', 'SAFETY_OFFICER', 'INSPECTOR', 'REGULATORY_OFFICER'].includes(user.role_key);

  if (canUpload && uploadBtn) {
    uploadBtn.classList.remove('hidden');
    if (user.role_key === 'INSPECTOR') {
      uploadBtn.textContent = '+ Upload Inspection Report';
    } else if (user.role_key === 'REGULATORY_OFFICER') {
      uploadBtn.textContent = '+ Upload Statutory Notice';
    } else if (user.role_key === 'SAFETY_OFFICER') {
      uploadBtn.textContent = '+ Upload Safety Clearance';
    } else {
      uploadBtn.textContent = '+ Upload Statutory Document';
    }
  }
}

async function loadMines() {
  try {
    allMines = await API.get('/mines');
    
    // Filter dropdown
    const filterMine = document.getElementById('filter-mine');
    filterMine.innerHTML = '<option value="">All Mines</option>' + 
      allMines.map(m => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');

    // Upload modal dropdown
    const selectMine = document.getElementById('doc-mine');
    selectMine.innerHTML = '<option value="">No specific mine (General / HQ Certificate)</option>' +
      allMines.map(m => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');

    // Edit modal dropdown
    const editMine = document.getElementById('edit-doc-mine');
    if (editMine) {
      editMine.innerHTML = '<option value="">General / Headquarters</option>' +
        allMines.map(m => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');
    }
  } catch (err) {
    showError(err);
  }
}

async function loadContractorsDropdown() {
  try {
    allContractors = await API.get('/contractors').catch(() => []);
    
    const selectCont = document.getElementById('doc-contractor');
    if (selectCont) {
      selectCont.innerHTML = '<option value="">No Contractor (CIL Direct / Departmental)</option>' +
        allContractors.map(c => `<option value="${c.id}">${c.company_name}</option>`).join('');
    }

    const editCont = document.getElementById('edit-doc-contractor');
    if (editCont) {
      editCont.innerHTML = '<option value="">None (Departmental / CIL Direct)</option>' +
        allContractors.map(c => `<option value="${c.id}">${c.company_name}</option>`).join('');
    }
  } catch (e) {
    console.error('Failed to load contractors', e);
  }
}

async function loadDocuments() {
  const tbody = document.getElementById('documents-table-body');
  tbody.innerHTML = '<tr><td colspan="9" class="state-panel">Loading statutory governance documents...</td></tr>';

  try {
    allDocuments = await API.get('/documents');
    updateKPIs(allDocuments);
    updateTabCounts(allDocuments);
    applyFilters();
    renderExpiryReminders(allDocuments);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="9" class="state-panel error">${err.message}</td></tr>`;
  }
}

function updateKPIs(docs) {
  document.getElementById('kpi-total-docs').textContent = docs.length;
  document.getElementById('kpi-pending-docs').textContent = docs.filter(d => d.workflow_status === 'PENDING_REVIEW').length;
  document.getElementById('kpi-approved-docs').textContent = docs.filter(d => d.workflow_status === 'APPROVED' || d.workflow_status === 'REVIEWED').length;
  document.getElementById('kpi-verified-docs').textContent = docs.filter(d => d.workflow_status === 'REGULATORY_VERIFIED').length;
  document.getElementById('kpi-violations-docs').textContent = docs.filter(d => d.workflow_status === 'VIOLATION_FLAGGED' || d.status === 'EXPIRED').length;
}

function updateTabCounts(docs) {
  document.getElementById('count-tab-all').textContent = docs.length;
  document.getElementById('count-tab-pending').textContent = docs.filter(d => d.workflow_status === 'PENDING_REVIEW').length;
  document.getElementById('count-tab-reviewed').textContent = docs.filter(d => d.workflow_status === 'REVIEWED').length;
  document.getElementById('count-tab-approved').textContent = docs.filter(d => d.workflow_status === 'APPROVED').length;
  document.getElementById('count-tab-verified').textContent = docs.filter(d => d.workflow_status === 'REGULATORY_VERIFIED').length;
  document.getElementById('count-tab-violations').textContent = docs.filter(d => d.workflow_status === 'VIOLATION_FLAGGED').length;
}

function applyFilters() {
  const query = document.getElementById('search-doc').value.toLowerCase().trim();
  const status = document.getElementById('filter-status').value;
  const mineId = document.getElementById('filter-mine').value;

  const filtered = allDocuments.filter(d => {
    if (currentStageFilter && d.workflow_status !== currentStageFilter) return false;
    if (status && d.status !== status) return false;
    if (mineId && String(d.mine_id) !== mineId) return false;
    if (query) {
      const matchCert = d.certificate_number?.toLowerCase().includes(query);
      const matchType = d.document_type?.toLowerCase().includes(query);
      const matchMine = d.mine_name?.toLowerCase().includes(query);
      const matchCode = d.mine_code?.toLowerCase().includes(query);
      const matchInspector = d.inspector_name?.toLowerCase().includes(query);
      const matchReg = d.regulatory_reference?.toLowerCase().includes(query);
      const matchUploader = d.uploaded_by_name?.toLowerCase().includes(query);
      if (!matchCert && !matchType && !matchMine && !matchCode && !matchInspector && !matchReg && !matchUploader) return false;
    }
    return true;
  });

  renderDocumentsTable(filtered);
}

function formatDate(val) {
  if (!val) return '-';
  if (typeof val === 'string') {
    return val.split('T')[0];
  }
  return val;
}

function renderDocumentsTable(docs) {
  const tbody = document.getElementById('documents-table-body');
  const user = AUTH.getUser();
  if (!user) return;

  const isAdmin = user.role_key === 'SUPER_ADMIN';
  const isManager = user.role_key === 'MINE_MANAGER' || isAdmin;
  const isSafetyOfficer = user.role_key === 'SAFETY_OFFICER' || isAdmin;
  const isRegulatory = user.role_key === 'REGULATORY_OFFICER' || isAdmin;

  if (!docs || docs.length === 0) {
    tbody.innerHTML = '<tr><td colspan="9" class="state-panel">No documents match the current criteria.</td></tr>';
    return;
  }

  tbody.innerHTML = docs.map(d => {
    // Validity badge
    let validityBadge = 'badge-active';
    if (d.status === 'EXPIRING_SOON') validityBadge = 'badge-warning';
    else if (d.status === 'EXPIRED') validityBadge = 'badge-critical';

    // Workflow status badge
    let wfBadge = 'badge-info';
    let wfLabel = d.workflow_status || 'PENDING_REVIEW';
    if (wfLabel === 'PENDING_REVIEW') {
      wfBadge = 'badge-warning';
      wfLabel = 'Pending Review';
    } else if (wfLabel === 'REVIEWED') {
      wfBadge = 'badge-info';
      wfLabel = 'Safety Reviewed';
    } else if (wfLabel === 'APPROVED') {
      wfBadge = 'badge-active';
      wfLabel = 'Manager Approved';
    } else if (wfLabel === 'REGULATORY_VERIFIED') {
      wfBadge = 'badge-active';
      wfLabel = 'DGMS Verified';
    } else if (wfLabel === 'VIOLATION_FLAGGED') {
      wfBadge = 'badge-critical';
      wfLabel = 'Breach Flagged';
    }

    // Risk badge
    const riskClass = { LOW: 'badge-low', MEDIUM: 'badge-medium', HIGH: 'badge-high', CRITICAL: 'badge-critical' };
    const riskBadge = riskClass[d.risk_level] || 'badge-low';

    const API_BASE = (window.APP_CONFIG?.API_BASE_URL || '/api').replace('/api', '');
    const relativePath = d.file_path ? d.file_path.replace('./', '') : '';
    const fileUrl = `${API_BASE}/${relativePath}`;

    // Dynamic Role-Based Actions
    let actionButtons = `
      <a href="${fileUrl}" target="_blank" class="btn btn-secondary btn-sm" style="text-decoration:none; padding:4px 8px; font-size:12px;">View File</a>
      <button class="btn btn-primary btn-sm" style="padding:4px 8px; font-size:12px;" onclick="open13FieldOCRModal(${d.id})">OCR Details</button>
    `;

    // 1. Safety Officer / Mine Manager / Admin: Review Findings
    if ((isSafetyOfficer || isManager) && d.workflow_status === 'PENDING_REVIEW') {
      actionButtons += `<button class="btn btn-secondary btn-sm" style="border-color:#0f766e; color:#0f766e; padding:4px 8px; font-size:12px;" onclick="openReviewModal(${d.id})">Review Findings</button>`;
    }

    // 2. Mine Manager / Admin: Approve
    if (isManager && d.workflow_status === 'REVIEWED') {
      actionButtons += `<button class="btn btn-primary btn-sm" style="background:#10b981; border-color:#10b981; padding:4px 8px; font-size:12px;" onclick="approveDocument(${d.id})">Approve</button>`;
    }

    // 3. Regulatory Officer / Admin: Regulatory Verification & Flagging
    if (isRegulatory && (d.workflow_status === 'APPROVED' || d.workflow_status === 'REVIEWED')) {
      actionButtons += `<button class="btn btn-secondary btn-sm" style="border-color:#0284c7; color:#0284c7; padding:4px 8px; font-size:12px;" onclick="openRegulatoryModal(${d.id})">Verify DGMS</button>`;
    }

    // 4. Create Live Action / Flag Violation (Safety Officer / Manager / Regulatory / Admin)
    if (isSafetyOfficer || isManager || isRegulatory) {
      actionButtons += `<button class="btn btn-danger btn-sm" style="padding:4px 8px; font-size:12px;" onclick="openFlagViolationModal(${d.id})">Flag Action</button>`;
    }

    // 5. Admin edit / delete
    if (isAdmin || isSafetyOfficer || isManager) {
      actionButtons += `<button class="btn btn-secondary btn-sm" style="padding:4px 8px; font-size:12px;" onclick="openEditModal(${d.id})">Edit</button>`;
    }
    if (isAdmin) {
      actionButtons += `<button class="btn btn-danger btn-sm" style="padding:4px 8px; font-size:12px;" onclick="deleteDocument(${d.id})">Delete</button>`;
    }

    return `
      <tr>
        <td class="mono" style="color:var(--color-brand); font-weight:600; font-size:12.5px;">${d.certificate_number}</td>
        <td>
          <div style="font-weight:600; color:var(--color-ink);">${d.document_type}</div>
          <div style="font-size:11px; color:var(--color-ink-muted); margin-top:2px;">${d.regulatory_reference || 'CMR 2017'}</div>
        </td>
        <td>
          <div style="font-weight:600; color:var(--color-ink);">${d.mine_name || 'General / HQ'}</div>
          <div class="mono" style="font-size:11px; color:var(--color-ink-muted);">${d.mine_code || '-'}</div>
        </td>
        <td class="mono" style="font-size:12.5px;">${formatDate(d.inspection_date || d.issue_date)}</td>
        <td style="font-size:12.5px;">${d.inspector_name || d.uploaded_by_name || '-'}</td>
        <td>
          <span class="badge ${riskBadge}">${d.risk_level || 'LOW'}</span>
          <span class="badge ${d.compliance_status === 'NON_COMPLIANT' ? 'badge-critical' : 'badge-active'}" style="margin-left:4px; font-size:10px;">${d.compliance_status || 'COMPLIANT'}</span>
        </td>
        <td><span class="badge ${wfBadge}">${wfLabel}</span></td>
        <td><span class="badge ${validityBadge}">${d.status}</span></td>
        <td style="text-align: right;">
          <div style="display:flex; gap:4px; flex-wrap:wrap; justify-content:flex-end;">
            ${actionButtons}
          </div>
        </td>
      </tr>
    `;
  }).join('');
}

function renderExpiryReminders(docs) {
  const container = document.getElementById('expiry-reminders-container');
  const criticalDocs = docs.filter(d => d.workflow_status === 'VIOLATION_FLAGGED' || d.status === 'EXPIRED' || d.status === 'EXPIRING_SOON');
  
  if (criticalDocs.length === 0) {
    container.innerHTML = '<p class="hint" style="margin:0;">No statutory violations or pending document expirations. All systems are compliant.</p>';
    return;
  }

  container.innerHTML = criticalDocs.map(d => {
    let alertClass = (d.status === 'EXPIRED' || d.workflow_status === 'VIOLATION_FLAGGED') ? 'critical' : 'warning';
    let iconSvg = alertClass === 'critical' ? (window.ICONS ? ICONS.get('siren') : '') : (window.ICONS ? ICONS.get('alert') : '');
    return `
      <div style="background: var(--color-surface); border-left: 4px solid var(--color-${alertClass}); padding: 10px 14px; border-radius: 0 4px 4px 0; font-size: 13px; display:flex; align-items:center; justify-content:space-between; box-shadow:0 1px 3px rgba(0,0,0,0.05);">
        <div style="display:flex; align-items:center; gap:8px;">
          <span style="display:inline-flex; align-items:center;">${iconSvg}</span>
          <div>
            <strong>${d.document_type} (${d.certificate_number})</strong> assoc. with <strong>${d.mine_name || 'General'}</strong>:
            <span style="font-weight:600; color:var(--color-${alertClass});"> ${d.violation_details || d.status.replace('_', ' ')}</span>
            (Due: ${d.due_date || d.expiry_date || 'N/A'}).
          </div>
        </div>
      </div>
    `;
  }).join('');
}

// ----------------- Upload Modal Handlers -----------------
function openUploadModal() {
  const form = document.getElementById('document-form');
  form.reset();
  document.getElementById('document-modal').classList.remove('hidden');
}

function closeUploadModal() {
  document.getElementById('document-modal').classList.add('hidden');
}

async function handleUploadSubmit(e) {
  e.preventDefault();
  const saveBtn = document.getElementById('modal-save');
  saveBtn.disabled = true;
  saveBtn.textContent = 'Uploading & Running Multimodal OCR...';

  const mineId = document.getElementById('doc-mine').value;
  const contractorId = document.getElementById('doc-contractor')?.value;
  const fileField = document.getElementById('doc-file');

  const formData = new FormData();
  if (mineId) formData.append('mine_id', mineId);
  if (contractorId) formData.append('contractor_id', contractorId);
  if (fileField.files[0]) formData.append('document', fileField.files[0]);

  const token = localStorage.getItem('cg_token');
  const API_BASE = window.APP_CONFIG?.API_BASE_URL || 'http://localhost:8081/api';

  try {
    const res = await fetch(`${API_BASE}/documents`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`
      },
      body: formData
    });
    const text = await res.text();
    let json;
    const trimmed = (text || '').trim();
    if (!trimmed) {
      json = { success: res.ok, data: null };
    } else {
      try {
        json = JSON.parse(trimmed);
      } catch (parseErr) {
        if (trimmed.startsWith('<')) {
          throw new Error(`Unexpected server response (status ${res.status}): Server returned HTML instead of JSON. Ensure backend is running on port 8081.`);
        }
        throw new Error(`Unexpected server response (status ${res.status}): ${trimmed.slice(0, 120)}`);
      }
    }
    if (!json.success) {
      throw new Error(json.message || 'Document upload and OCR failed');
    }

    showToast(`OCR Completed! Extracted 13 fields for: ${json.data.document_type} (${json.data.certificate_number})`, 'success');
    closeUploadModal();
    await loadDocuments();
  } catch (err) {
    showError(err);
  } finally {
    saveBtn.disabled = false;
    saveBtn.textContent = 'Upload & Run AI OCR';
  }
}

// ----------------- Review Findings Modal (Safety Officer / Manager) -----------------
window.openReviewModal = function(id) {
  const doc = allDocuments.find(d => d.id === id);
  if (!doc) return;

  document.getElementById('rev-doc-id').value = doc.id;
  document.getElementById('rev-compliance').value = doc.compliance_status || 'COMPLIANT';
  document.getElementById('rev-risk').value = doc.risk_level || 'LOW';
  document.getElementById('rev-violations').value = doc.violation_details || '';
  document.getElementById('rev-action').value = doc.corrective_action || '';
  document.getElementById('rev-due-date').value = doc.due_date || '';
  document.getElementById('rev-notes').value = '';

  document.getElementById('review-modal-title').textContent = `Review AI Findings — ${doc.certificate_number}`;
  document.getElementById('review-modal').classList.remove('hidden');
};

function closeReviewModal() {
  document.getElementById('review-modal').classList.add('hidden');
}

async function handleReviewSubmit(e) {
  e.preventDefault();
  const id = document.getElementById('rev-doc-id').value;
  const payload = {
    compliance_status: document.getElementById('rev-compliance').value,
    risk_level: document.getElementById('rev-risk').value,
    violation_details: document.getElementById('rev-violations').value.trim(),
    corrective_action: document.getElementById('rev-action').value.trim(),
    due_date: document.getElementById('rev-due-date').value,
    review_notes: document.getElementById('rev-notes').value.trim(),
  };

  try {
    await API.post(`/documents/${id}/review`, payload);
    showToast('Document findings reviewed and escalated for approval', 'success');
    closeReviewModal();
    await loadDocuments();
  } catch (err) {
    showError(err);
  }
}

// ----------------- Approve Document (Mine Manager / Super Admin) -----------------
window.approveDocument = async function(id) {
  const doc = allDocuments.find(d => d.id === id);
  const certName = doc ? doc.certificate_number : `#${id}`;

  if (!confirm(`Are you sure you want to approve document ${certName} for mine-level statutory compliance?`)) {
    return;
  }

  try {
    await API.post(`/documents/${id}/approve`, {});
    showToast(`Document ${certName} approved successfully`, 'success');
    await loadDocuments();
  } catch (err) {
    showError(err);
  }
};

// ----------------- Regulatory Verification (Regulatory Officer / Super Admin) -----------------
window.openRegulatoryModal = function(id) {
  const doc = allDocuments.find(d => d.id === id);
  if (!doc) return;

  document.getElementById('reg-doc-id').value = doc.id;
  document.getElementById('reg-notes').value = `Verified against ${doc.regulatory_reference || 'Coal Mines Regulations 2017'}. All parameters checked.`;
  document.getElementById('regulatory-modal-title').textContent = `DGMS Verification — ${doc.certificate_number}`;
  document.getElementById('regulatory-modal').classList.remove('hidden');
};

function closeRegulatoryModal() {
  document.getElementById('regulatory-modal').classList.add('hidden');
}

async function handleRegulatorySubmit(e) {
  e.preventDefault();
  const id = document.getElementById('reg-doc-id').value;
  const actionRadio = document.querySelector('input[name="reg_action"]:checked');
  const action = actionRadio ? actionRadio.value : 'VERIFY';
  const notes = document.getElementById('reg-notes').value.trim();

  try {
    await API.post(`/documents/${id}/verify-regulatory`, {
      action: action,
      verification_notes: notes
    });
    showToast(action === 'VERIFY' ? 'Regulatory compliance certified' : 'Statutory breach notice issued', action === 'VERIFY' ? 'success' : 'warning');
    closeRegulatoryModal();
    await loadDocuments();
  } catch (err) {
    showError(err);
  }
}

// ----------------- Flag Violation Modal -----------------
window.openFlagViolationModal = function(id) {
  const doc = allDocuments.find(d => d.id === id);
  if (!doc) return;

  document.getElementById('fv-doc-id').value = doc.id;
  document.getElementById('fv-title').value = `Compliance Breach: ${doc.document_type} (${doc.certificate_number})`;
  document.getElementById('fv-desc').value = doc.violation_details || `Identified statutory breach during inspection on ${doc.inspection_date || doc.issue_date}. Ref: ${doc.regulatory_reference}`;
  document.getElementById('fv-action').value = doc.corrective_action || 'Immediately remediate hazard and submit compliance proof.';
  document.getElementById('fv-severity').value = doc.risk_level === 'CRITICAL' ? 'CRITICAL' : 'HIGH';
  document.getElementById('fv-due-date').value = doc.due_date || new Date(Date.now() + 7 * 86400000).toISOString().split('T')[0];

  document.getElementById('flag-violation-modal').classList.remove('hidden');
};

function closeFlagViolationModal() {
  document.getElementById('flag-violation-modal').classList.add('hidden');
}

async function handleFlagViolationSubmit(e) {
  e.preventDefault();
  const id = document.getElementById('fv-doc-id').value;
  const payload = {
    title: document.getElementById('fv-title').value.trim(),
    description: document.getElementById('fv-desc').value.trim(),
    severity: document.getElementById('fv-severity').value,
    corrective_action: document.getElementById('fv-action').value.trim(),
    due_date: document.getElementById('fv-due-date').value,
    responsible_dept: 'Safety & Vigilance'
  };

  try {
    await API.post(`/documents/${id}/flag-violation`, payload);
    showToast('Violation and corrective action created successfully', 'warning');
    closeFlagViolationModal();
    await loadDocuments();
  } catch (err) {
    showError(err);
  }
}

// ----------------- 13-Field OCR Inspector Modal -----------------
function renderOCRCategoryTable(doc) {
  const body = document.getElementById('ocr-categorized-table-body');
  if (!body) return;

  const ocrData = (doc.ocr_data_json && typeof doc.ocr_data_json === 'object') ? doc.ocr_data_json : {};
  const fieldCategories = ocrData.field_categories || doc.field_categories || [];

  if (Array.isArray(fieldCategories) && fieldCategories.length > 0) {
    body.innerHTML = fieldCategories.map(cat => {
      const matchedTermsBadges = (cat.matched_terms || []).map(t => 
        `<span style="display:inline-block; background:#e0f2fe; color:#0369a1; padding:2px 6px; border-radius:4px; font-size:11px; margin-right:4px; margin-bottom:4px;">${t}</span>`
      ).join('') || '<span style="color:#94a3b8; font-size:11px;">-</span>';

      const fieldsText = (cat.fields && cat.fields.length > 0)
        ? cat.fields.map(f => `<div><strong>${f.field}:</strong> ${f.value}</div>`).join('')
        : '<span style="color:#64748b; font-size:11.5px;">No direct fields mapped to this category</span>';

      const percentageScore = cat.percentage || `${Math.round((cat.score || 0) * 100)}%`;
      let scoreBadgeClass = 'badge-low';
      if ((cat.score || 0) >= 0.08) scoreBadgeClass = 'badge-active';
      else if ((cat.score || 0) >= 0.04) scoreBadgeClass = 'badge-info';

      return `
        <tr style="border-bottom: 1px solid var(--color-border);">
          <td style="padding:10px 12px; vertical-align:top; font-weight:600; color:var(--color-ink);">${cat.category}</td>
          <td style="padding:10px 12px; vertical-align:top;">
            <span class="badge ${scoreBadgeClass}">${percentageScore} (Score: ${cat.score || 0})</span>
          </td>
          <td style="padding:10px 12px; vertical-align:top;">${matchedTermsBadges}</td>
          <td style="padding:10px 12px; vertical-align:top;">${fieldsText}</td>
        </tr>
      `;
    }).join('');
    return;
  }

  // Fallback if field_categories is missing
  const categoryMap = [
    {
      category: 'Mine Details',
      score: '95%',
      terms: ['mine name', 'mine code', 'colliery'],
      fields: [
        { field: 'Mine Name', value: doc.mine_name || 'Not detected' },
        { field: 'Mine Code', value: doc.mine_code || 'N/A' }
      ]
    },
    {
      category: 'Inspection',
      score: '90%',
      terms: ['inspection date', 'inspector', 'certificate number'],
      fields: [
        { field: 'Certificate Number', value: doc.certificate_number || 'UNSPECIFIED' },
        { field: 'Document Type', value: doc.document_type || 'Unidentified Document' },
        { field: 'Inspection Date', value: formatDate(doc.inspection_date || doc.issue_date) },
        { field: 'Inspector Name', value: doc.inspector_name || doc.uploaded_by_name || 'N/A' }
      ]
    },
    {
      category: 'Compliance & Regulatory Reference',
      score: '85%',
      terms: ['compliant', 'violation', 'regulation', 'dgms'],
      fields: [
        { field: 'Compliance Status', value: doc.compliance_status || 'UNKNOWN' },
        { field: 'Regulatory Reference', value: doc.regulatory_reference || 'Not available' },
        { field: 'Violation Details', value: doc.violation_details || 'No mining compliance details detected in the uploaded image.' }
      ]
    },
    {
      category: 'Risk & Safety',
      score: '75%',
      terms: ['risk level', 'safety hazard'],
      fields: [
        { field: 'Risk Level', value: doc.risk_level || 'LOW' }
      ]
    },
    {
      category: 'Action & Timeline',
      score: '88%',
      terms: ['corrective action', 'due date', 'expiry date'],
      fields: [
        { field: 'Corrective Action', value: doc.corrective_action || 'No specific corrective action detected' },
        { field: 'Due Date', value: formatDate(doc.due_date) },
        { field: 'Expiry Date', value: formatDate(doc.expiry_date) }
      ]
    }
  ];

  body.innerHTML = categoryMap.map(cat => `
    <tr style="border-bottom: 1px solid var(--color-border);">
      <td style="padding:10px 12px; vertical-align:top; font-weight:600;">${cat.category}</td>
      <td style="padding:10px 12px; vertical-align:top;"><span class="badge badge-active">${cat.score}</span></td>
      <td style="padding:10px 12px; vertical-align:top;">
        ${cat.terms.map(t => `<span style="display:inline-block; background:#e0f2fe; color:#0369a1; padding:2px 6px; border-radius:4px; font-size:11px; margin-right:4px; margin-bottom:4px;">${t}</span>`).join('')}
      </td>
      <td style="padding:10px 12px; vertical-align:top;">
        ${cat.fields.map(f => `<div><strong>${f.field}:</strong> ${f.value}</div>`).join('')}
      </td>
    </tr>
  `).join('');
}

window.open13FieldOCRModal = function(id) {
  const doc = allDocuments.find(d => d.id === id);
  if (!doc) return;

  document.getElementById('ocr-modal-title').textContent = `AI OCR 13-Field Inspector — ${doc.certificate_number}`;
  
  document.getElementById('ocr-m-name').textContent = doc.mine_name || 'Not detected';
  document.getElementById('ocr-m-code').textContent = doc.mine_code || 'N/A';
  document.getElementById('ocr-m-type').textContent = doc.document_type || 'Unidentified Document';
  document.getElementById('ocr-m-ins-date').textContent = formatDate(doc.inspection_date || doc.issue_date);
  document.getElementById('ocr-m-inspector').textContent = doc.inspector_name || doc.uploaded_by_name || 'N/A';
  document.getElementById('ocr-m-compliance').textContent = doc.compliance_status || 'UNKNOWN';
  document.getElementById('ocr-m-violations').textContent = doc.violation_details || 'No mining compliance details detected in the uploaded image.';
  document.getElementById('ocr-m-risk').textContent = doc.risk_level || 'LOW';
  document.getElementById('ocr-m-action').textContent = doc.corrective_action || 'No specific corrective action detected';
  document.getElementById('ocr-m-due').textContent = formatDate(doc.due_date);
  document.getElementById('ocr-m-cert').textContent = doc.certificate_number || 'UNSPECIFIED';
  document.getElementById('ocr-m-expiry').textContent = formatDate(doc.expiry_date);
  document.getElementById('ocr-m-reg-ref').textContent = doc.regulatory_reference || 'Not available';
  
  document.getElementById('ocr-raw-text').value = doc.ocr_raw_text || 'No raw text transcript available.';
  renderOCRCategoryTable(doc);

  document.getElementById('ocr-modal').classList.remove('hidden');
};

function closeOCRModal() {
  document.getElementById('ocr-modal').classList.add('hidden');
}

function copyOCRTranscript() {
  const text = document.getElementById('ocr-raw-text').value;
  navigator.clipboard.writeText(text).then(() => {
    showToast('OCR raw transcript copied to clipboard', 'success');
  }).catch(() => {
    showToast('Failed to copy text', 'error');
  });
}

// ----------------- Edit & Delete Handlers -----------------
window.openEditModal = function(id) {
  const doc = allDocuments.find(d => d.id === id);
  if (!doc) return;

  const insDateFormatted = formatDate(doc.inspection_date);
  const expDateFormatted = formatDate(doc.expiry_date);

  document.getElementById('edit-doc-id').value = doc.id;
  document.getElementById('edit-cert-no').value = doc.certificate_number || '';
  document.getElementById('edit-doc-type').value = doc.document_type || '';
  document.getElementById('edit-doc-mine').value = doc.mine_id || '';
  document.getElementById('edit-doc-contractor').value = doc.contractor_id || '';
  document.getElementById('edit-inspector').value = doc.inspector_name || '';
  document.getElementById('edit-ins-date').value = insDateFormatted !== '-' ? insDateFormatted : '';
  document.getElementById('edit-expiry-date').value = expDateFormatted !== '-' ? expDateFormatted : '';
  document.getElementById('edit-status').value = doc.status || 'VALID';
  document.getElementById('edit-ocr-text').value = doc.ocr_raw_text || '';

  document.getElementById('edit-modal-title').textContent = `Edit Document — ${doc.certificate_number}`;
  document.getElementById('edit-document-modal').classList.remove('hidden');
};

function closeEditModal() {
  document.getElementById('edit-document-modal').classList.add('hidden');
}

async function handleEditSubmit(e) {
  e.preventDefault();
  const id = document.getElementById('edit-doc-id').value;
  const payload = {
    certificate_number: document.getElementById('edit-cert-no').value.trim(),
    document_type: document.getElementById('edit-doc-type').value.trim(),
    mine_id: document.getElementById('edit-doc-mine').value ? parseInt(document.getElementById('edit-doc-mine').value, 10) : null,
    contractor_id: document.getElementById('edit-doc-contractor').value ? parseInt(document.getElementById('edit-doc-contractor').value, 10) : null,
    inspector_name: document.getElementById('edit-inspector').value.trim(),
    inspection_date: document.getElementById('edit-ins-date').value || null,
    expiry_date: document.getElementById('edit-expiry-date').value || null,
    status: document.getElementById('edit-status').value,
    ocr_raw_text: document.getElementById('edit-ocr-text').value.trim()
  };

  try {
    await API.put(`/documents/${id}`, payload);
    showToast('Document record updated successfully', 'success');
    closeEditModal();
    await loadDocuments();
  } catch (err) {
    showError(err);
  }
}

window.deleteDocument = async function(id) {
  const user = AUTH.getUser();
  if (!user || user.role_key !== 'SUPER_ADMIN') {
    showToast('Permission denied: Only Administrators can delete statutory documents.', 'error');
    return;
  }

  const doc = allDocuments.find(d => d.id === id);
  const certName = doc ? doc.certificate_number : `#${id}`;

  if (!confirm(`Are you sure you want to permanently delete document ${certName}? This action cannot be undone.`)) {
    return;
  }

  try {
    await API.delete(`/documents/${id}`);
    showToast(`Document ${certName} deleted successfully`, 'warning');
    await loadDocuments();
  } catch (err) {
    showError(err);
  }
};
