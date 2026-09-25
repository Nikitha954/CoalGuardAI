/**
 * notifications.js — lightweight toast notifications.
 * Usage: showToast('Mine created successfully', 'success')
 */
function showToast(message, type = 'info', duration = 4000) {
  let root = document.getElementById('toast-root');
  if (!root) {
    root = document.createElement('div');
    root.id = 'toast-root';
    document.body.appendChild(root);
  }

  const toast = document.createElement('div');
  toast.className = `toast ${type}`;
  toast.textContent = message;
  root.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transition = 'opacity 0.3s ease';
    setTimeout(() => toast.remove(), 300);
  }, duration);
}

function showError(err) {
  const message = err && err.message ? err.message : 'Something went wrong. Please try again.';
  showToast(message, 'error');
}
