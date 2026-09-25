// voice-assistant.js - Multilingual Voice Assistant & Description Voice Input
// Supports: English (en-IN), Hindi (hi-IN), Tamil (ta-IN), Telugu (te-IN)

(function () {
  const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;

  const SUPPORTED_LANGUAGES = [
    { code: 'en-IN', name: 'English (IN)', native: 'English' },
    { code: 'hi-IN', name: 'Hindi', native: 'हिंदी' },
    { code: 'ta-IN', name: 'Tamil', native: 'தமிழ்' },
    { code: 'te-IN', name: 'Telugu', native: 'తెలుగు' }
  ];

  function getSavedLanguage() {
    const saved = localStorage.getItem('cg_voice_lang');
    if (saved && SUPPORTED_LANGUAGES.some(l => l.code === saved)) {
      return saved;
    }
    // Backward compatibility for old 'en-US'
    if (saved === 'en-US') return 'en-IN';
    return 'en-IN';
  }

  function setSavedLanguage(langCode) {
    localStorage.setItem('cg_voice_lang', langCode);
  }

  // Ensure bottom-right floating AI assistant UI exists on the page
  function ensureFloatingUI() {
    let ui = document.getElementById('voice-assistant-ui');
    if (!ui) {
      ui = document.createElement('div');
      ui.id = 'voice-assistant-ui';
      ui.style.cssText = 'position: fixed; bottom: 20px; right: 20px; z-index: 1000; display: flex; flex-direction: column; align-items: flex-end; gap: 10px;';
      const botIcon = window.ICONS ? ICONS.get('bot') : '';
      const micIcon = window.ICONS ? ICONS.get('mic') : '';
      ui.innerHTML = `
        <div id="voice-response-panel" class="card hidden" style="background: var(--color-surface, #fff); padding: 14px; width: 320px; border-left: 4px solid var(--color-primary, #1e40af); box-shadow: 0 8px 24px rgba(0,0,0,0.15); border-radius: 8px;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px;">
            <div style="font-weight: 600; font-size: 13px; color: var(--color-ink, #1f2937); display:flex; align-items:center; gap:6px;">${botIcon} AI Assistant</div>
            <button type="button" id="voice-response-close" style="background:none; border:none; font-size:16px; cursor:pointer; color:var(--color-ink-muted, #6b7280); line-height:1;">&times;</button>
          </div>
          <div id="voice-response-text" style="font-size: 13px; line-height: 1.5; color: var(--color-ink, #1f2937); max-height: 220px; overflow-y: auto;"></div>
        </div>
        <div style="display: flex; align-items: center; gap: 10px; background: white; padding: 6px 14px; border-radius: 30px; box-shadow: 0 4px 16px rgba(0,0,0,0.15); border: 1px solid var(--color-border, #e5e7eb);">
          <select id="voice-lang-select" style="border: none; background: transparent; font-size: 13px; font-weight: 500; outline: none; cursor: pointer; color: var(--color-ink, #374151);">
          </select>
          <div id="voice-status" style="font-size: 12px; color: var(--color-ink-muted, #6b7280); max-width: 140px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;"></div>
          <button id="btn-voice-mic" class="btn btn-primary" title="Click to ask AI assistant via voice" style="border-radius: 50%; width: 38px; height: 38px; padding: 0; display: flex; align-items: center; justify-content: center; font-size: 14px; cursor: pointer;">
            ${micIcon}
          </button>
        </div>
      `;
      document.body.appendChild(ui);
    }

    // Populate / update language select options
    const langSelect = document.getElementById('voice-lang-select');
    if (langSelect) {
      const currentVal = getSavedLanguage();
      langSelect.innerHTML = SUPPORTED_LANGUAGES.map(l => 
        `<option value="${l.code}" ${l.code === currentVal ? 'selected' : ''}>${l.native} (${l.name.split(' ')[0]})</option>`
      ).join('');

      langSelect.value = currentVal;
      langSelect.onchange = () => {
        setSavedLanguage(langSelect.value);
      };
    }

    // Bind close button on response panel
    const closeBtn = document.getElementById('voice-response-close');
    if (closeBtn) {
      closeBtn.onclick = () => {
        const panel = document.getElementById('voice-response-panel');
        if (panel) panel.classList.add('hidden');
      };
    }
  }

  // Safe Text-to-Speech playback for AI voice assistant responses
  function speakResponse(text, langCode) {
    if (!('speechSynthesis' in window)) return;
    try {
      window.speechSynthesis.cancel();
      const utterance = new SpeechSynthesisUtterance(text);
      utterance.lang = langCode;

      // Select preferred voice for language if available in browser
      const voices = window.speechSynthesis.getVoices();
      if (voices && voices.length > 0) {
        const target = langCode.toLowerCase();
        const prefix = target.split('-')[0];
        const match = voices.find(v => v.lang.toLowerCase() === target || v.lang.toLowerCase().startsWith(prefix));
        if (match) utterance.voice = match;
      }

      utterance.onerror = (e) => {
        // If TTS voice is missing on user OS/browser for Tamil or Telugu, 
        // display of the text response is already safely completed.
        console.warn(`[TTS] Voice output unavailable for ${langCode}:`, e);
      };

      window.speechSynthesis.speak(utterance);
    } catch (err) {
      console.warn("[TTS] Speech synthesis error:", err);
    }
  }

  // =========================================================================
  // 1. AI VOICE ASSISTANT (Bottom-Right Widget)
  // =========================================================================
  let assistantRecognition = null;

  function initAssistant() {
    const btnMic = document.getElementById('btn-voice-mic');
    const langSelect = document.getElementById('voice-lang-select');
    const statusIndicator = document.getElementById('voice-status');
    const aiResponsePanel = document.getElementById('voice-response-panel');
    const aiResponseText = document.getElementById('voice-response-text');

    if (!btnMic || !langSelect) return;

    if (!SpeechRecognition) {
      btnMic.title = "Speech recognition is not supported in this browser.";
      btnMic.style.opacity = "0.6";
      return;
    }

    btnMic.addEventListener('click', () => {
      // Toggle stop if already recording
      if (btnMic.classList.contains('recording')) {
        if (assistantRecognition) assistantRecognition.stop();
        return;
      }

      const activeLang = langSelect.value || getSavedLanguage();
      setSavedLanguage(activeLang);

      assistantRecognition = new SpeechRecognition();
      assistantRecognition.continuous = false;
      assistantRecognition.interimResults = false;
      assistantRecognition.lang = activeLang;

      assistantRecognition.onstart = () => {
        btnMic.classList.add('recording');
        if (statusIndicator) statusIndicator.textContent = "Listening...";
      };

      assistantRecognition.onresult = async (event) => {
        const transcript = event.results[0][0].transcript;
        if (statusIndicator) statusIndicator.textContent = `Heard: "${transcript.substring(0, 18)}..."`;
        btnMic.classList.remove('recording');

        try {
          if (statusIndicator) statusIndicator.textContent = "Thinking...";
          const token = localStorage.getItem('cg_token');
          const API_BASE = window.APP_CONFIG?.API_BASE_URL || '/api';

          const res = await fetch(`${API_BASE}/ai/voice-query`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({
              query: transcript,
              language: activeLang
            })
          });

          const text = await res.text();
          let json;
          try { json = (text || '').trim() ? JSON.parse(text) : { success: res.ok }; } catch (e) { json = { success: false, message: 'Invalid AI response' }; }

          if (json.success && json.data && json.data.answer) {
            const answer = json.data.answer;
            if (aiResponseText) aiResponseText.textContent = answer;
            if (aiResponsePanel) aiResponsePanel.classList.remove('hidden');
            if (statusIndicator) statusIndicator.textContent = "";

            // Speak answer using TTS when supported
            speakResponse(answer, activeLang);
          } else {
            if (statusIndicator) statusIndicator.textContent = "Error processing query.";
          }
        } catch (err) {
          console.error(err);
          if (statusIndicator) statusIndicator.textContent = "Network error.";
        }
      };

      assistantRecognition.onerror = (event) => {
        console.warn("Assistant recognition error:", event.error);
        btnMic.classList.remove('recording');
        if (statusIndicator) statusIndicator.textContent = "Speech error.";
      };

      assistantRecognition.onend = () => {
        btnMic.classList.remove('recording');
      };

      try {
        assistantRecognition.start();
      } catch (e) {
        console.warn("Could not start recognition:", e);
      }
    });
  }

  // =========================================================================
  // 2. MULTILINGUAL VOICE INPUT IN DESCRIPTION / OBSERVATION FIELDS
  // =========================================================================
  let activeDescRecognition = null;
  let activeDescButton = null;

  function insertTextIntoField(targetEl, textToInsert) {
    if (!targetEl) return;
    const current = targetEl.value || "";
    
    // Insert with clean spacing
    if (current.length > 0 && !current.endsWith(' ') && !current.endsWith('\n')) {
      targetEl.value = current + ' ' + textToInsert;
    } else {
      targetEl.value = current + textToInsert;
    }

    targetEl.focus();
    // Dispatch input events so framework listeners & form validators catch the change
    targetEl.dispatchEvent(new Event('input', { bubbles: true }));
    targetEl.dispatchEvent(new Event('change', { bubbles: true }));
  }

  function handleDescVoiceClick(btn) {
    if (!SpeechRecognition) {
      alert("Web Speech recognition is not supported in this browser. Please use Chrome, Edge, or Safari.");
      return;
    }

    const targetId = btn.getAttribute('data-target');
    const targetEl = document.getElementById(targetId);
    if (!targetEl) {
      console.warn("Target field not found:", targetId);
      return;
    }

    // If currently recording on this button, stop it
    if (btn.classList.contains('recording')) {
      if (activeDescRecognition) {
        activeDescRecognition.stop();
      }
      return;
    }

    // If recording on another button, stop that one first
    if (activeDescRecognition) {
      activeDescRecognition.stop();
    }

    // Read the active language from the EXISTING bottom-right language selector
    const langSelect = document.getElementById('voice-lang-select');
    const selectedLang = langSelect ? langSelect.value : getSavedLanguage();

    // Stop any active assistant recognition to avoid device lock
    if (assistantRecognition) {
      try { assistantRecognition.abort(); } catch(e) {}
    }

    const langProfile = SUPPORTED_LANGUAGES.find(l => l.code === selectedLang) || SUPPORTED_LANGUAGES[0];

    const recognition = new SpeechRecognition();
    recognition.continuous = true;
    recognition.interimResults = true;
    recognition.lang = selectedLang;

    activeDescRecognition = recognition;
    activeDescButton = btn;

    const initialText = targetEl.value || '';
    const prefix = initialText && !initialText.endsWith(' ') && !initialText.endsWith('\n') ? initialText + ' ' : initialText;
    let accumulatedFinal = '';
    let hasInserted = false;

    recognition.onstart = () => {
      btn.classList.add('recording');
      const micSvg = window.ICONS ? ICONS.get('mic') : '';
      btn.innerHTML = `<span class="mic-icon" style="display:inline-flex; align-items:center;">${micSvg}</span> <span class="mic-status-text">Listening (${langProfile.native})... Click to Stop</span>`;
    };

    recognition.onresult = (event) => {
      let interim = '';
      for (let i = event.resultIndex; i < event.results.length; ++i) {
        const item = event.results[i];
        if (item.isFinal) {
          accumulatedFinal += item[0].transcript + ' ';
        } else {
          interim += item[0].transcript;
        }
      }

      const liveText = (accumulatedFinal + interim).trim();
      if (liveText) {
        targetEl.value = prefix + liveText;
        targetEl.dispatchEvent(new Event('input', { bubbles: true }));
        targetEl.scrollTop = targetEl.scrollHeight;
        hasInserted = true;
      }
    };

    recognition.onerror = (event) => {
      console.warn("Description voice input error:", event.error);
      const notify = window.showToast ? (msg, type) => window.showToast(msg, type) : (msg) => alert(msg);
      if (event.error === 'not-allowed' || event.error === 'service-not-allowed') {
        notify("Microphone access blocked. Click the lock/camera icon in your address bar to allow microphone.", "error");
      } else if (event.error === 'network') {
        notify("Speech service connection error. Please check your network.", "error");
      }
      resetDescButton(btn);
    };

    recognition.onend = () => {
      if (hasInserted) {
        targetEl.value = (prefix + accumulatedFinal).trim();
        targetEl.dispatchEvent(new Event('input', { bubbles: true }));
        targetEl.dispatchEvent(new Event('change', { bubbles: true }));
        const checkSvg = window.ICONS ? ICONS.get('check') : '';
        btn.innerHTML = `<span class="mic-icon" style="display:inline-flex; align-items:center;">${checkSvg}</span> <span class="mic-status-text">Inserted!</span>`;
        setTimeout(() => resetDescButton(btn), 1200);
      } else {
        resetDescButton(btn);
      }
    };

    try {
      recognition.start();
    } catch (err) {
      console.warn("Error starting description speech recognition:", err);
      resetDescButton(btn);
    }
  }

  function resetDescButton(btn) {
    if (!btn) return;
    btn.classList.remove('recording');
    const micSvg = window.ICONS ? ICONS.get('mic') : '';
    btn.innerHTML = `<span class="mic-icon" style="display:inline-flex; align-items:center;">${micSvg}</span> <span class="mic-status-text">Voice Input</span>`;
    if (activeDescRecognition) {
      try { activeDescRecognition.stop(); } catch(e) {}
      activeDescRecognition = null;
      activeDescButton = null;
    }
  }

  // Delegated click listener for all Description/Observation mic buttons
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('.btn-desc-mic');
    if (btn) {
      e.preventDefault();
      handleDescVoiceClick(btn);
      return;
    }

    // Delegated click listener for Translate buttons
    const transBtn = e.target.closest('.btn-desc-translate');
    if (transBtn) {
      e.preventDefault();
      handleTranslateClick(transBtn);
    }
  });

  async function handleTranslateClick(transBtn) {
    const targetId = transBtn.getAttribute('data-target');
    const targetEl = document.getElementById(targetId);
    if (!targetEl || !targetEl.value.trim()) {
      const notify = window.showToast ? (msg, type) => window.showToast(msg, type) : (msg) => alert(msg);
      notify("Please speak or type text into the field first before translating.", "warning");
      return;
    }

    const origHtml = transBtn.innerHTML;
    transBtn.disabled = true;
    const refreshSvg = window.ICONS ? ICONS.get('refresh') : '';
    transBtn.innerHTML = `<span class="trans-icon" style="display:inline-flex; align-items:center;">${refreshSvg}</span> Translating...`;

    try {
      const token = localStorage.getItem('cg_token');
      const API_BASE = window.APP_CONFIG?.API_BASE_URL || 'http://localhost:8081/api';
      const res = await fetch(`${API_BASE}/ai/translate`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': token ? `Bearer ${token}` : ''
        },
        body: JSON.stringify({
          text: targetEl.value.trim(),
          target_language: 'English'
        })
      });

      const text = await res.text();
      let data;
      try { data = (text || '').trim() ? JSON.parse(text) : { success: res.ok }; } catch (e) { data = { success: false, translated_text: text }; }
      if (data.success && data.data && data.data.translated_text) {
        targetEl.value = data.data.translated_text;
        targetEl.dispatchEvent(new Event('input', { bubbles: true }));
        targetEl.dispatchEvent(new Event('change', { bubbles: true }));
        if (window.showToast) window.showToast("Translated to English successfully!", "success");
      } else {
        if (window.showToast) window.showToast("Translation failed. Keeping original text.", "error");
      }
    } catch (err) {
      if (window.showToast) window.showToast("Cannot reach translation service.", "error");
    } finally {
      transBtn.disabled = false;
      transBtn.innerHTML = origHtml;
    }
  }

  // Auto inject Translate button next to any .btn-desc-mic if not already there
  function ensureTranslateButtons() {
    document.querySelectorAll('.btn-desc-mic').forEach(micBtn => {
      const targetId = micBtn.getAttribute('data-target');
      if (!targetId) return;
      const parent = micBtn.parentElement;
      if (parent && !parent.querySelector(`.btn-desc-translate[data-target="${targetId}"]`)) {
        const transBtn = document.createElement('button');
        transBtn.type = 'button';
        transBtn.className = 'btn-desc-translate';
        transBtn.setAttribute('data-target', targetId);
        transBtn.setAttribute('title', 'Translate this text to English');
        const globeSvg = window.ICONS ? ICONS.get('globe') : '';
        transBtn.innerHTML = `<span class="trans-icon" style="display:inline-flex; align-items:center;">${globeSvg}</span> <span class="trans-text">Translate to EN</span>`;
        micBtn.after(transBtn);
      }
    });
  }

  // Pre-load voices for TTS if available
  if ('speechSynthesis' in window) {
    window.speechSynthesis.onvoiceschanged = () => {
      window.speechSynthesis.getVoices();
    };
  }

  // Initialize once DOM is ready
  function init() {
    ensureFloatingUI();
    initAssistant();
    ensureTranslateButtons();

    // Auto-inject Translate button whenever a modal or dynamic form is opened
    const observer = new MutationObserver(() => {
      ensureTranslateButtons();
    });
    observer.observe(document.body, { childList: true, subtree: true });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
