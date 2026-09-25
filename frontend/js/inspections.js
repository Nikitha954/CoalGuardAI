/**
 * inspections.js — Inspections module client logic.
 * Enforces role-based visibility and supports conducting field audits with GPS + photo uploads.
 */
let allInspections = [];
let allMines = [];
let complianceRules = [];
let categories = [];

// --- Offline IndexedDB Setup ---
const DB_NAME = 'CoalGuardOfflineDB';
const STORE_NAME = 'inspections';

function openOfflineDB() {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, 1);
    req.onupgradeneeded = (e) => {
      const db = e.target.result;
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        db.createObjectStore(STORE_NAME, { keyPath: 'id', autoIncrement: true });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

async function saveOfflineInspection(payloadObj) {
  const db = await openOfflineDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite');
    tx.objectStore(STORE_NAME).add(payloadObj);
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
  });
}

async function getOfflineInspections() {
  const db = await openOfflineDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly');
    const req = tx.objectStore(STORE_NAME).getAll();
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

async function deleteOfflineInspection(id) {
  const db = await openOfflineDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite');
    tx.objectStore(STORE_NAME).delete(id);
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
  });
}

window.syncOfflineInspections = async function() {
  if (!navigator.onLine) return;
  try {
    const items = await getOfflineInspections();
    if (items.length === 0) return;

    showToast(`Syncing ${items.length} offline inspection(s)...`, 'info');
    const token = localStorage.getItem('cg_token');
    const API_BASE = window.APP_CONFIG?.API_BASE_URL || 'http://localhost:8081/api';

    let successCount = 0;
    for (const item of items) {
      const fd = new FormData();
      for (const key in item) {
        if (key !== 'id') {
          fd.append(key, item[key]);
        }
      }
      const res = await fetch(`${API_BASE}/inspections`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
        body: fd
      });
      const text = await res.text();
      let json;
      try { json = (text || '').trim() ? JSON.parse(text) : { success: res.ok }; } catch (e) { json = { success: false }; }
      if (json.success) {
        await deleteOfflineInspection(item.id);
        successCount++;
      }
    }
    if (successCount > 0) {
      showToast(`Successfully synced ${successCount} offline inspection(s).`, 'success');
      await loadInspections();
    }
  } catch (err) {
    console.error('Failed to sync offline inspections', err);
  }
};
// -------------------------------

document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('inspections.html');

  const user = AUTH.getUser();
  const canConduct = ['INSPECTOR', 'SAFETY_OFFICER', 'SUPER_ADMIN'].includes(user.role_key);

  if (canConduct) {
    document.getElementById('conduct-inspection-btn').classList.remove('hidden');
  }

  // Bind UI Events
  document.getElementById('conduct-inspection-btn').addEventListener('click', () => openInspectionModal());
  document.getElementById('modal-close').addEventListener('click', closeInspectionModal);
  document.getElementById('modal-cancel').addEventListener('click', closeInspectionModal);
  document.getElementById('inspection-form').addEventListener('submit', handleInspectionSubmit);
  document.getElementById('btn-detect-gps').addEventListener('click', captureGPS);
  document.getElementById('btn-ai-analyze-draft').addEventListener('click', runDraftAIAnalysis);
  
  document.getElementById('detail-close').addEventListener('click', closeDetailModal);
  document.getElementById('detail-back').addEventListener('click', closeDetailModal);

  document.getElementById('filter-mine').addEventListener('change', applyFilters);
  document.getElementById('filter-status').addEventListener('change', applyFilters);

  // Load baseline resources
  await loadMines();
  await loadCategories();
  await loadComplianceRules();
  initInspectionsMap();
  await loadInspections();
});

async function loadMines() {
  try {
    allMines = await API.get('/mines');
    const filterMine = document.getElementById('filter-mine');
    const selectMine = document.getElementById('ins-mine');

    const options = allMines.map(m => `<option value="${m.id}">${m.mine_name} (${m.mine_code})</option>`).join('');
    filterMine.innerHTML = `<option value="">All Mines</option>${options}`;
    selectMine.innerHTML = `<option value="" disabled selected>Select Mine Site...</option>${options}`;
  } catch (err) {
    showError(err);
  }
}

async function loadCategories() {
  try {
    categories = await API.get('/compliance/categories');
    const selectCat = document.getElementById('obs-category');
    selectCat.innerHTML = `<option value="">Select Category...</option>` + 
      categories.map(c => `<option value="${c.id}">${c.name}</option>`).join('');
  } catch (err) {
    showError(err);
  }
}

async function loadComplianceRules() {
  try {
    complianceRules = await API.get('/compliance/rules');
    renderChecklist();
  } catch (err) {
    showError(err);
  }
}

function renderChecklist() {
  const container = document.getElementById('checklist-container');
  if (complianceRules.length === 0) {
    container.innerHTML = `<p class="hint">No compliance rules found. Create some in Compliance panel first.</p>`;
    return;
  }

  // Display daily/weekly/monthly rules as checklist
  container.innerHTML = complianceRules.map((r, idx) => `
    <div class="checklist-item-row" style="display: flex; align-items: center; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid var(--color-border-light);">
      <div style="flex: 1; padding-right: 12px;">
        <div style="font-weight: 600; font-size: 13.5px;">${r.rule_code}: ${r.title}</div>
        <div class="hint" style="font-size:12px;">Freq: ${r.frequency} &middot; Severity: ${r.severity} &middot; Dept: ${r.responsible_dept}</div>
      </div>
      <div style="display: flex; gap: 8px; align-items: center;">
        <input type="hidden" name="rule_code_${idx}" value="${r.rule_code}" />
        <input type="hidden" name="rule_title_${idx}" value="${r.title}" />
        <select name="rule_result_${idx}" style="padding: 4px; font-size: 13px;" required>
          <option value="PASS">Pass</option>
          <option value="FAIL">Fail</option>
          <option value="NA" selected>N/A</option>
        </select>
        <input type="text" name="rule_remarks_${idx}" placeholder="Remarks (optional)..." style="padding: 4px; width: 160px; font-size: 13px;" />
      </div>
    </div>
  `).join('');
}

async function loadInspections() {
  const tbody = document.getElementById('inspections-table-body');
  tbody.innerHTML = `<tr><td colspan="7" class="state-panel">Loading inspections...</td></tr>`;

  try {
    allInspections = await API.get('/inspections');
    renderInspectionsTable(allInspections);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="7" class="state-panel error">${err.message}</td></tr>`;
  }
}

function applyFilters() {
  const mine = document.getElementById('filter-mine').value;
  const status = document.getElementById('filter-status').value;

  const filtered = allInspections.filter(i => {
    if (mine && String(i.mine_id) !== mine) return false;
    if (status && i.status !== status) return false;
    return true;
  });

  renderInspectionsTable(filtered);
}

function renderInspectionsTable(inspections) {
  renderInspectionPins(inspections);
  const tbody = document.getElementById('inspections-table-body');
  if (inspections.length === 0) {
    tbody.innerHTML = `<tr><td colspan="7" class="state-panel">No inspections found.</td></tr>`;
    return;
  }

  tbody.innerHTML = inspections.map(i => {
    let badgeClass = 'inactive';
    if (i.status === 'APPROVED') badgeClass = 'active';
    else if (i.status === 'SUBMITTED') badgeClass = 'warning';
    else if (i.status === 'REVIEWED') badgeClass = 'info';

    return `
      <tr>
        <td><strong>${i.mine_name}</strong></td>
        <td>${i.inspection_type}</td>
        <td>${i.inspector_name}</td>
        <td>${i.inspection_date} &middot; <span class="hint">${i.inspection_time}</span></td>
        <td class="mono">${i.gps_latitude.toFixed(4)}, ${i.gps_longitude.toFixed(4)}</td>
        <td><span class="badge badge-${badgeClass}">${i.status}</span></td>
        <td>
          <button class="btn btn-secondary btn-sm" onclick="viewInspectionDetail(${i.id})">View Details</button>
        </td>
      </tr>
    `;
  }).join('');
}

function openInspectionModal() {
  const form = document.getElementById('inspection-form');
  form.reset();
  document.getElementById('ins-lat').value = '';
  document.getElementById('ins-lng').value = '';
  document.getElementById('obs-description').value = '';
  document.getElementById('inspection-modal').classList.remove('hidden');
  
  // Auto capture GPS immediately
  captureGPS();
}

function closeInspectionModal() {
  document.getElementById('inspection-modal').classList.add('hidden');
}

function captureGPS() {
  const latInput = document.getElementById('ins-lat');
  const lngInput = document.getElementById('ins-lng');
  
  latInput.value = 'Locating...';
  lngInput.value = 'Locating...';

  if (!navigator.geolocation) {
    showToast('Geolocation is not supported by your browser.', 'error');
    latInput.value = '';
    lngInput.value = '';
    return;
  }

  navigator.geolocation.getCurrentPosition(
    (pos) => {
      latInput.value = pos.coords.latitude.toFixed(6);
      lngInput.value = pos.coords.longitude.toFixed(6);
      showToast('GPS coordinates successfully captured.', 'success');
    },
    (err) => {
      showToast('Unable to capture GPS. Please enter coordinates manually.', 'warning');
      latInput.value = '22.3595'; // Gevra lat default
      lngInput.value = '82.6892'; // Gevra lng default
    },
    { enableHighAccuracy: true, timeout: 5000 }
  );
}

async function handleInspectionSubmit(e) {
  e.preventDefault();
  const saveBtn = document.getElementById('modal-save');
  saveBtn.disabled = true;
  saveBtn.textContent = 'Submitting...';

  const mineId = document.getElementById('ins-mine').value;
  const insType = document.getElementById('ins-type').value;
  const lat = document.getElementById('ins-lat').value;
  const lng = document.getElementById('ins-lng').value;
  const remarks = document.getElementById('ins-remarks').value.trim();
  const status = document.getElementById('ins-status').value;

  // Build checklist payload array
  const checklist = [];
  complianceRules.forEach((r, idx) => {
    const resSel = document.getElementsByName(`rule_result_${idx}`)[0];
    const remInput = document.getElementsByName(`rule_remarks_${idx}`)[0];
    if (resSel) {
      checklist.push({
        checklist_item: `${r.rule_code}: ${r.title}`,
        result: resSel.value,
        remarks: remInput ? remInput.value.trim() : ''
      });
    }
  });

  const formData = new FormData();
  formData.append('mine_id', mineId);
  formData.append('inspection_type', insType);
  formData.append('gps_latitude', lat);
  formData.append('gps_longitude', lng);
  formData.append('remarks', remarks);
  formData.append('status', status);
  formData.append('checklist', JSON.stringify(checklist));

  // Add observation details if typed
  const obsText = document.getElementById('obs-description').value.trim();
  if (obsText) {
    formData.append('observation', obsText);
    formData.append('observation_category_id', document.getElementById('obs-category').value);
    formData.append('observation_severity', document.getElementById('obs-severity').value);
    
    const fileField = document.getElementById('obs-evidence');
    if (fileField.files[0]) {
      formData.append('evidence', fileField.files[0]);
    }
  }

  const token = localStorage.getItem('cg_token');
  const API_BASE = window.APP_CONFIG?.API_BASE_URL || 'http://localhost:8081/api';

  const payloadObj = {};
  for (let [key, value] of formData.entries()) {
    payloadObj[key] = value;
  }

  if (!navigator.onLine) {
    try {
      await saveOfflineInspection(payloadObj);
      showToast('You are offline. Inspection queued for sync.', 'warning');
      closeInspectionModal();
    } catch (e) {
      showError('Failed to save inspection offline.');
    } finally {
      saveBtn.disabled = false;
      saveBtn.textContent = 'Submit Inspection';
    }
    return;
  }

  try {
    const res = await fetch(`${API_BASE}/inspections`, {
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
      throw new Error(json.message || 'Inspection submission failed');
    }

    showToast('Inspection report submitted successfully', 'success');
    closeInspectionModal();
    await loadInspections();
  } catch (err) {
    if (err.name === 'TypeError' || err.message.includes('Failed to fetch')) {
      try {
        await saveOfflineInspection(payloadObj);
        showToast('Network error. Inspection queued for sync.', 'warning');
        closeInspectionModal();
      } catch (e) {
        showError('Network error, and failed to save offline.');
      }
    } else {
      showError(err);
    }
  } finally {
    saveBtn.disabled = false;
    saveBtn.textContent = 'Submit Inspection';
  }
}

async function viewInspectionDetail(id) {
  const content = document.getElementById('detail-content');
  const actionsContainer = document.getElementById('workflow-actions-container');
  content.innerHTML = '<p class="state-panel">Loading inspection details...</p>';
  actionsContainer.innerHTML = '';
  document.getElementById('detail-modal').classList.remove('hidden');

  try {
    const data = await API.get(`/inspections/${id}`);
    const ins = data.inspection;
    const items = ins.items || [];
    const observations = data.observations || [];

    let badgeClass = 'inactive';
    if (ins.status === 'APPROVED') badgeClass = 'active';
    else if (ins.status === 'SUBMITTED') badgeClass = 'warning';
    else if (ins.status === 'REVIEWED') badgeClass = 'info';

    let evidenceHtml = '';
    if (observations.length > 0) {
      evidenceHtml = `
        <h4 style="margin-top:20px; border-bottom:1px solid var(--color-border); padding-bottom:4px;">Observations</h4>
        ${observations.map(o => {
          let imgHtml = '';
          if (o.evidence_path) {
            const API_BASE = (window.APP_CONFIG?.API_BASE_URL || 'http://localhost:8081/api').replace('/api', '');
            const relativePath = o.evidence_path.replace('./', '');
            imgHtml = `
              <div style="margin-top: 10px;">
                <strong>Evidence Image:</strong><br/>
                <img src="${API_BASE}/${relativePath}" alt="Evidence Photo" style="max-width: 100%; max-height: 250px; border-radius: 6px; border: 1px solid var(--color-border); margin-top: 5px;" />
              </div>`;
          }
          return `
            <div style="background: var(--color-bg-light); border-left: 4px solid var(--color-warning); padding: 12px; margin-bottom: 12px; border-radius: 0 6px 6px 0;">
              <div><strong>Category:</strong> ${o.category_name || 'N/A'} &middot; <strong>Severity:</strong> <span class="badge badge-inactive">${o.severity}</span></div>
              <p style="margin: 6px 0 0 0; font-size: 13.5px; line-height: 1.5; color: var(--color-ink);">${o.observation}</p>
              ${imgHtml}
            </div>
          `;
        }).join('')}
      `;
    }

    let aiHtml = '';
    if (data.ai_analysis) {
      const ai = data.ai_analysis;
      const sparklesSvg = window.ICONS ? ICONS.get('sparkles') : '';
      aiHtml = `
        <div class="card card-body" style="margin-top: 20px; border-left: 4px solid var(--color-primary); background: var(--color-bg-light); padding: 16px;">
          <h4 style="margin-top: 0; display: flex; align-items: center; gap: 8px;">
            ${sparklesSvg} AI Compliance Analysis (${ai.model_name})
          </h4>
          <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin-bottom: 12px; font-size: 13px;">
            <div><strong>Severity:</strong> <span class="badge badge-inactive" style="font-size: 10.5px; padding: 2px 6px;">${ai.severity}</span></div>
            <div><strong>Risk Level:</strong> <span class="badge badge-inactive" style="font-size: 10.5px; padding: 2px 6px;">${ai.risk_level}</span></div>
            <div><strong>Risk Score:</strong> <strong>${ai.risk_score}</strong>/100</div>
            <div><strong>Recurring Issue:</strong> <span class="badge badge-inactive" style="font-size: 10.5px; padding: 2px 6px;">${ai.recurring_issue ? 'YES' : 'NO'}</span></div>
            <div><strong>Confidence:</strong> <strong>${Math.round(ai.confidence * 100)}</strong>%</div>
          </div>
          <div style="font-size: 13.5px; line-height: 1.5; color: var(--color-ink); margin-bottom: 8px;">
            <strong>Summary:</strong> ${ai.summary}
          </div>
          <div style="font-size: 13.5px; line-height: 1.5; color: var(--color-ink); margin-bottom: 8px;">
            <strong>AI Recommended Action:</strong> ${ai.recommended_action}
          </div>
          <div style="font-size: 13.5px; line-height: 1.5; color: var(--color-ink-muted); margin-bottom: 8px; font-style: italic;">
            <strong>Reasoning:</strong> ${ai.reasoning}
          </div>
          <p class="hint" style="margin: 0; border-top: 1px dashed var(--color-border); padding-top: 6px; font-size: 11.5px;">
            * AI-assisted analysis — final decision remains with authorized official.
          </p>
        </div>`;
    } else {
      const sparklesSvg = window.ICONS ? ICONS.get('sparkles') : '';
      aiHtml = `
        <div class="card card-body" style="margin-top: 20px; text-align: center; padding: 20px; border: 1px dashed var(--color-border);">
          <p class="hint" style="margin-bottom: 12px; font-size: 13px;">No AI analysis report is cached for this inspection observation.</p>
          <button class="btn btn-primary" id="btn-run-ai-analysis" onclick="triggerAISavedAnalysis(${ins.id})">
            ${sparklesSvg} Analyze with AI
          </button>
        </div>`;
    }

    content.innerHTML = `
      <div style="display: flex; gap: 20px; flex-wrap: wrap; margin-bottom: 20px;">
        <div style="flex: 1; min-width: 250px;">
          <table class="detail-info-table" style="width: 100%; font-size: 13.5px;">
            <tr><td style="font-weight:600; padding:4px 0;">Mine:</td><td>${ins.mine_name}</td></tr>
            <tr><td style="font-weight:600; padding:4px 0;">Type:</td><td>${ins.inspection_type}</td></tr>
            <tr><td style="font-weight:600; padding:4px 0;">Inspector:</td><td>${ins.inspector_name}</td></tr>
            <tr><td style="font-weight:600; padding:4px 0;">Date/Time:</td><td>${ins.inspection_date} &middot; ${ins.inspection_time}</td></tr>
          </table>
        </div>
        <div style="flex: 1; min-width: 250px;">
          <table class="detail-info-table" style="width: 100%; font-size: 13.5px;">
            <tr><td style="font-weight:600; padding:4px 0;">GPS Location:</td><td class="mono">${ins.gps_latitude.toFixed(6)}, ${ins.gps_longitude.toFixed(6)}</td></tr>
            <tr><td style="font-weight:600; padding:4px 0;">Status:</td><td><span class="badge badge-${badgeClass}">${ins.status}</span></td></tr>
          </table>
        </div>
      </div>

      <h4 style="border-bottom:1px solid var(--color-border); padding-bottom:4px; margin-top:20px;">General Remarks</h4>
      <p style="font-size: 13.5px; line-height: 1.6; color: var(--color-ink-muted);">${ins.remarks || 'No remarks provided.'}</p>

      <h4 style="border-bottom:1px solid var(--color-border); padding-bottom:4px; margin-top:20px;">Checklist Audit results</h4>
      <table class="data-table" style="margin-top:10px; font-size:13px;">
        <thead>
          <tr>
            <th>Checklist Rule</th>
            <th>Result</th>
            <th>Remarks</th>
          </tr>
        </thead>
        <tbody>
          ${items.map(item => {
            let resClass = 'badge-inactive';
            if (item.result === 'PASS') resClass = 'badge-active';
            else if (item.result === 'FAIL') resClass = 'badge-danger';
            return `
              <tr>
                <td><strong>${item.checklist_item}</strong></td>
                <td><span class="badge ${resClass}">${item.result}</span></td>
                <td style="color: var(--color-ink-muted);">${item.remarks || '&mdash;'}</td>
              </tr>
            `;
          }).join('')}
        </tbody>
      </table>

      ${evidenceHtml}
      ${aiHtml}
    `;

    // Add Approval workflows if allowed
    const user = AUTH.getUser();
    const isSupervisor = ['MINE_MANAGER', 'SAFETY_OFFICER', 'SUPER_ADMIN'].includes(user.role_key);
    
    if (isSupervisor && ins.status === 'SUBMITTED') {
      actionsContainer.innerHTML = `
        <button class="btn btn-secondary" onclick="updateStatus(${ins.id}, 'REVIEWED')" style="margin-right:8px;">Mark Under Review</button>
        <button class="btn btn-primary" onclick="updateStatus(${ins.id}, 'APPROVED')">Approve Inspection</button>
      `;
    } else if (isSupervisor && ins.status === 'REVIEWED') {
      actionsContainer.innerHTML = `
        <button class="btn btn-primary" onclick="updateStatus(${ins.id}, 'APPROVED')">Approve Inspection</button>
      `;
    }
  } catch (err) {
    content.innerHTML = `<p class="state-panel error">${err.message}</p>`;
  }
}

async function runDraftAIAnalysis() {
  const obsText = document.getElementById('obs-description').value.trim();
  const mineId = document.getElementById('ins-mine').value;
  const insType = document.getElementById('ins-type').value;

  if (!obsText) {
    showToast('Please enter an observation description first.', 'warning');
    return;
  }
  if (!mineId) {
    showToast('Please select a target mine first.', 'warning');
    return;
  }

  const btn = document.getElementById('btn-ai-analyze-draft');
  const panel = document.getElementById('ai-analysis-panel');

  btn.disabled = true;
  btn.textContent = 'Analyzing Observation...';

  try {
    const res = await API.post('/inspections/analyze-draft', {
      mine_id: parseInt(mineId, 10),
      inspection_type: insType,
      observation: obsText
    });

    // Populate panel
    document.getElementById('ai-severity').textContent = res.severity;
    document.getElementById('ai-risk-level').textContent = res.risk_level;
    document.getElementById('ai-risk-score').textContent = res.risk_score;
    document.getElementById('ai-recurring-issue').textContent = res.recurring_issue ? 'YES' : 'NO';
    document.getElementById('ai-confidence').textContent = Math.round(res.confidence * 100);
    
    document.getElementById('ai-summary').textContent = res.summary;
    document.getElementById('ai-recommended-action').textContent = res.recommended_action;
    document.getElementById('ai-reasoning').textContent = res.reasoning;

    // Show panel
    panel.classList.remove('hidden');
    showToast('AI analysis completed successfully!', 'success');
  } catch (err) {
    showToast(err.message || 'AI service could not be reached', 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Analyze Observation with AI';
  }
}

async function triggerAISavedAnalysis(id) {
  const btn = document.getElementById('btn-run-ai-analysis');
  if (btn) {
    btn.disabled = true;
    btn.textContent = 'Running AI Analysis...';
  }

  try {
    await API.post(`/inspections/${id}/analyze`);
    showToast('AI report successfully computed & saved.', 'success');
    await viewInspectionDetail(id);
  } catch (err) {
    showToast(err.message || 'AI service could not be reached', 'error');
    // Reload detail to show the fallback message if returned by backend
    await viewInspectionDetail(id);
  }
}

async function updateStatus(id, newStatus) {
  if (!confirm(`Are you sure you want to transition this inspection status to ${newStatus}?`)) return;
  try {
    await API.put(`/inspections/${id}/status`, { status: newStatus });
    showToast(`Inspection ${newStatus.toLowerCase()} successfully`, 'success');
    closeDetailModal();
    await loadInspections();
  } catch (err) {
    showError(err);
  }
}

function closeDetailModal() {
  document.getElementById('detail-modal').classList.add('hidden');
}

let inspectionsMapInstance = null;
let inspectionMarkersGroup = null;

function initInspectionsMap() {
  const mapEl = document.getElementById('inspections-map');
  if (!mapEl || typeof L === 'undefined') return;

  inspectionsMapInstance = L.map('inspections-map').setView([22.8, 83.5], 6);

  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 18,
    attribution: '&copy; OpenStreetMap contributors'
  }).addTo(inspectionsMapInstance);

  inspectionMarkersGroup = L.layerGroup().addTo(inspectionsMapInstance);
}

function renderInspectionPins(inspections) {
  if (!inspectionsMapInstance || !inspectionMarkersGroup) return;

  inspectionMarkersGroup.clearLayers();
  const bounds = [];

  inspections.forEach(i => {
    if (!i.gps_latitude || !i.gps_longitude) return;

    const lat = parseFloat(i.gps_latitude);
    const lng = parseFloat(i.gps_longitude);
    bounds.push([lat, lng]);

    let pinColor = '#10b981'; // Approved
    if (i.status === 'DRAFT') pinColor = '#64748b';
    else if (i.status === 'SUBMITTED') pinColor = '#f59e0b';
    else if (i.status === 'REVIEWED') pinColor = '#3b82f6';

    const marker = L.circleMarker([lat, lng], {
      radius: 9,
      fillColor: pinColor,
      color: '#ffffff',
      weight: 2,
      opacity: 1,
      fillOpacity: 0.9
    }).addTo(inspectionMarkersGroup);

    const popupHTML = `
      <div style="font-family:sans-serif; min-width:180px; padding:2px;">
        <strong style="font-size:13px; color:#1e293b;">${i.mine_name}</strong>
        <div style="font-size:11.5px; color:#64748b; margin-top:2px;">${i.inspection_type}</div>
        <div style="font-size:11.5px; margin-top:4px;">
          <span>Inspector:</span> <strong>${i.inspector_name}</strong>
        </div>
        <div style="font-size:11.5px;">
          <span>Date:</span> ${i.inspection_date}
        </div>
        <div style="font-size:11.5px; margin-top:2px;">
          <span>Status:</span> <strong style="color:${pinColor};">${i.status}</strong>
        </div>
        <div style="margin-top:6px; border-top:1px solid #e2e8f0; padding-top:4px;">
          <button class="btn btn-secondary btn-sm" style="width:100%; font-size:11px; padding:3px 6px;" onclick="viewInspectionDetail(${i.id})">Open Audit</button>
        </div>
      </div>
    `;

    marker.bindPopup(popupHTML);
  });

  if (bounds.length > 0) {
    inspectionsMapInstance.fitBounds(bounds, { padding: [25, 25], maxZoom: 12 });
  }
}

