/**
 * analytics.js — AI Analytics panel logic.
 * Renders explainable risk scores and Isolation Forest anomalies.
 */
document.addEventListener('DOMContentLoaded', async () => {
  AUTH.guardPage();
  AUTH.renderShell('analytics.html');

  document.getElementById('recalc-risk-btn').addEventListener('click', triggerRecalculate);

  await loadRiskScores();
  await loadAnomalies();
  await loadRecurringViolations();
});

async function loadRiskScores() {
  const container = document.getElementById('risk-scores-container');
  container.innerHTML = '<p class="state-panel">Loading risk scores...</p>';

  try {
    const scores = await API.get('/analytics/risk');
    if (scores.length === 0) {
      container.innerHTML = '<p class="state-panel">No risk evaluations found. Trigger a re-evaluation first.</p>';
      return;
    }

    container.innerHTML = scores.map(item => {
      let badgeClass = 'inactive';
      if (item.classification === 'CRITICAL' || item.classification === 'HIGH') badgeClass = 'danger';
      else if (item.classification === 'MEDIUM') badgeClass = 'warning';
      else if (item.classification === 'LOW') badgeClass = 'active';

      let factorsHtml = '<p class="hint" style="margin: 4px 0 0 0;">No significant risk factors flagged. Site compliant.</p>';
      
      const factors = item.factors || [];
      if (factors.length > 0) {
        factorsHtml = `
          <ul style="margin: 8px 0 0 0; padding-left: 18px; font-size: 13px; line-height: 1.5; color: var(--color-ink-muted);">
            ${factors.map(f => `
              <li style="margin-bottom: 2px;">
                ${f.name} <strong style="color: var(--color-critical); float: right;">+${f.impact}</strong>
              </li>
            `).join('')}
          </ul>
        `;
      }

      return `
        <div class="card card-body" style="border-left: 4px solid var(--color-${item.classification.toLowerCase()}); padding: 16px;">
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <strong style="font-size: 15px; color: var(--color-ink);">${item.mine_name}</strong>
            <div style="text-align: right;">
              <span class="badge badge-${badgeClass}" style="font-size: 11.5px;">${item.classification}</span>
              <strong style="display: block; font-size: 16px; margin-top: 2px;">Score: ${item.score}</strong>
            </div>
          </div>
          <div style="margin-top: 10px; border-top: 1px dashed var(--color-border-light); padding-top: 8px;">
            <div style="font-size:12px; text-transform:uppercase; font-weight:600; color:var(--color-ink-muted);">Contributing Factors:</div>
            ${factorsHtml}
          </div>
        </div>
      `;
    }).join('');

  } catch (err) {
    container.innerHTML = `<p class="state-panel error">${err.message}</p>`;
  }
}

async function loadAnomalies() {
  const tbody = document.getElementById('anomalies-table-body');
  tbody.innerHTML = '<tr><td colspan="5" class="state-panel">Loading anomalies...</td></tr>';

  try {
    const list = await API.get('/analytics/anomalies');
    if (list.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="state-panel">No operational anomalies registered by scikit-learn model.</td></tr>';
      return;
    }

    tbody.innerHTML = list.map(item => {
      let badgeClass = 'inactive';
      if (item.severity === 'CRITICAL' || item.severity === 'HIGH') badgeClass = 'danger';
      else if (item.severity === 'MEDIUM') badgeClass = 'warning';

      let dateStr = new Date(item.detected_at).toLocaleString();

      return `
        <tr>
          <td><strong>${item.mine_name}</strong></td>
          <td><span class="badge badge-info">${item.anomaly_type}</span></td>
          <td style="font-size:12.5px; line-height:1.4;">
            <strong>${item.description}</strong><br/>
            <span class="hint" style="font-size:11.5px;">Val: ${item.detected_value.toFixed(1)} &middot; Exp: ${item.expected_value.toFixed(1)}</span>
          </td>
          <td><span class="badge badge-${badgeClass}">${item.severity}</span></td>
          <td style="font-size:12px;">${dateStr}</td>
        </tr>
      `;
    }).join('');

  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="5" class="state-panel error">${err.message}</td></tr>`;
  }
}

async function triggerRecalculate() {
  const btn = document.getElementById('recalc-risk-btn');
  btn.disabled = true;
  btn.textContent = 'Recalculating...';

  try {
    await API.post('/analytics/risk/recalculate');
    showToast('Mine risk levels successfully re-evaluated', 'success');
    await loadRiskScores();
    await loadRecurringViolations();
  } catch (err) {
    showError(err);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Re-evaluate Mine Risk Scores';
  }
}

async function loadRecurringViolations() {
  const tbody = document.getElementById('recurring-table-body');
  if (!tbody) return;
  tbody.innerHTML = '<tr><td colspan="8" class="state-panel">Loading repeat violation analysis...</td></tr>';

  try {
    const list = await API.get('/analytics/recurring-violations');
    if (list.length === 0) {
      tbody.innerHTML = '<tr><td colspan="8" class="state-panel">No chronic repeat violations detected across mine inspections.</td></tr>';
      return;
    }

    tbody.innerHTML = list.map(item => {
      let sevBadge = 'badge-active';
      if (item.severity === 'CRITICAL') sevBadge = 'badge-critical';
      else if (item.severity === 'HIGH') sevBadge = 'badge-warning';
      else if (item.severity === 'MEDIUM') sevBadge = 'badge-info';

      const lastDate = item.last_occurred_at ? new Date(item.last_occurred_at).toLocaleDateString('en-IN') : '-';
      const riskSurcharge = Math.min(item.repeat_count * 10, 30);

      return `
        <tr>
          <td><strong>${item.mine_name}</strong></td>
          <td><span class="badge badge-info">${item.category_name}</span></td>
          <td class="mono font-semibold">${item.rule_code}</td>
          <td><strong>${item.title}</strong></td>
          <td>
            <span class="badge ${item.repeat_count >= 3 ? 'badge-critical' : 'badge-warning'}">
              ${window.ICONS ? ICONS.get('warning') : ''} ${item.repeat_count}x Repeated
            </span>
          </td>
          <td><span class="badge ${sevBadge}">${item.severity}</span></td>
          <td><strong style="color:var(--color-critical);">+${riskSurcharge} pts</strong></td>
          <td class="mono">${lastDate}</td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="8" class="state-panel error">${err.message}</td></tr>`;
  }
}

