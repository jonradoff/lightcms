/* LightCMS Chat Widget — drop-in snippet for any site
 * Reads config from /api/chat/config (same origin or data-base-url attribute)
 * Queries /api/chat?q=... for search results via SSE streaming
 *
 * If #nav-search-wrap is present in the DOM, injects a chat icon button to its
 * right and opens the panel as a centered overlay. Otherwise falls back to the
 * standard floating bottom-corner widget.
 */
(function () {
  'use strict';

  var script = document.currentScript;
  var baseURL = (script && script.getAttribute('data-base-url')) ||
    (script ? new URL(script.src).origin : window.location.origin);

  var WIDGET_ID = 'lc-chat-widget';
  var config = null;
  var useNavMode = false; // true when nav injection is active

  function hexToRgb(hex) {
    var r = parseInt(hex.slice(1, 3), 16);
    var g = parseInt(hex.slice(3, 5), 16);
    var b = parseInt(hex.slice(5, 7), 16);
    return r + ',' + g + ',' + b;
  }

  function injectStyles(cfg) {
    var existing = document.getElementById('lc-chat-widget-styles');
    if (existing) existing.remove();

    var rgb = hexToRgb(cfg.primary_color || '#6366f1');
    var pos = cfg.position === 'bottom-left' ? 'left: 1.5rem;' : 'right: 1.5rem;';

    var css = [
      /* ── Floating corner widget ── */
      '#lc-chat-widget { position: fixed; bottom: 1.5rem; ' + pos + ' z-index: 9999; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 15px; }',
      '#lc-chat-btn { width: 56px; height: 56px; border-radius: 50%; background: ' + cfg.primary_color + '; border: none; cursor: pointer; display: flex; align-items: center; justify-content: center; box-shadow: 0 4px 16px rgba(' + rgb + ',0.45); transition: transform 0.15s, box-shadow 0.15s; }',
      '#lc-chat-btn:hover { transform: scale(1.07); box-shadow: 0 6px 20px rgba(' + rgb + ',0.55); }',
      '#lc-chat-btn svg { width: 26px; height: 26px; fill: #fff; }',
      '#lc-chat-panel { position: absolute; bottom: 68px; ' + pos + ' width: 340px; max-width: calc(100vw - 2rem); background: #fff; border-radius: 16px; box-shadow: 0 8px 40px rgba(0,0,0,0.18); overflow: hidden; display: flex; flex-direction: column; max-height: 520px; }',
      '#lc-chat-panel[hidden] { display: none; }',

      /* ── Overlay mode (nav-triggered) ── */
      '#lc-chat-overlay { display: none; position: fixed; inset: 0; z-index: 9998; background: rgba(0,0,0,0.45); align-items: flex-start; justify-content: center; padding-top: 5rem; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 15px; }',
      '#lc-chat-overlay.open { display: flex; }',
      '#lc-chat-overlay-panel { width: 480px; max-width: calc(100vw - 2rem); background: #fff; border-radius: 16px; box-shadow: 0 16px 56px rgba(0,0,0,0.32); overflow: hidden; display: flex; flex-direction: column; max-height: calc(100vh - 7rem); }',

      /* ── Nav chat button ── */
      '#lc-nav-chat-btn { background: none; border: none; cursor: pointer; padding: 4px 6px; border-radius: 6px; display: flex; align-items: center; justify-content: center; color: rgba(241,245,249,0.65); transition: color 0.2s, background 0.2s; margin-left: 2px; }',
      '#lc-nav-chat-btn:hover { color: rgba(241,245,249,0.95); background: rgba(255,255,255,0.08); }',
      '#lc-nav-chat-btn svg { width: 18px; height: 18px; }',

      /* ── Shared panel internals ── */
      '.lc-chat-header { background: ' + cfg.primary_color + '; color: #fff; padding: 0.875rem 1rem; display: flex; align-items: center; justify-content: space-between; flex-shrink: 0; }',
      '.lc-chat-header-title { font-weight: 600; font-size: 0.9375rem; }',
      '.lc-chat-close { background: none; border: none; color: rgba(255,255,255,0.8); cursor: pointer; font-size: 1.25rem; line-height: 1; padding: 0; display: flex; align-items: center; justify-content: center; width: 24px; height: 24px; border-radius: 4px; transition: background 0.1s; }',
      '.lc-chat-close:hover { background: rgba(255,255,255,0.15); color: #fff; }',
      '.lc-chat-body { flex: 1; overflow-y: auto; padding: 1rem; display: flex; flex-direction: column; gap: 0.75rem; min-height: 120px; }',
      '.lc-welcome { color: #64748b; font-size: 0.875rem; line-height: 1.5; }',
      '.lc-answer { font-size: 0.875rem; line-height: 1.6; color: #1e293b; background: #f8fafc; border-radius: 10px; padding: 0.75rem; white-space: pre-wrap; word-break: break-word; }',
      '.lc-sources-label { font-size: 0.75rem; font-weight: 600; color: #94a3b8; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 0.375rem; }',
      '.lc-sources { display: flex; flex-direction: column; gap: 0.25rem; }',
      '.lc-result { font-size: 0.8125rem; color: ' + cfg.primary_color + '; text-decoration: none; display: flex; align-items: center; gap: 0.375rem; padding: 0.3rem 0; border-bottom: 1px solid #f1f5f9; line-height: 1.3; }',
      '.lc-result:last-child { border-bottom: none; }',
      '.lc-result:hover { text-decoration: underline; }',
      '.lc-result-arrow { opacity: 0.5; flex-shrink: 0; font-size: 0.7rem; }',
      '.lc-result-title { font-weight: 500; color: #1e293b; }',
      '.lc-result-title:hover { color: ' + cfg.primary_color + '; }',
      '.lc-no-results { color: #94a3b8; font-size: 0.875rem; text-align: center; padding: 1rem 0; }',
      '.lc-loading { display: flex; gap: 4px; justify-content: center; align-items: center; padding: 1rem 0; }',
      '.lc-dot { width: 8px; height: 8px; border-radius: 50%; background: ' + cfg.primary_color + '; animation: lc-bounce 0.9s infinite; }',
      '.lc-dot:nth-child(2) { animation-delay: 0.15s; }',
      '.lc-dot:nth-child(3) { animation-delay: 0.3s; }',
      '@keyframes lc-bounce { 0%,80%,100% { transform: scale(0.7); opacity: 0.5; } 40% { transform: scale(1); opacity: 1; } }',
      '.lc-chat-footer { padding: 0.75rem; border-top: 1px solid #f1f5f9; display: flex; gap: 0.5rem; flex-shrink: 0; }',
      '.lc-chat-input { flex: 1; border: 1px solid #e2e8f0; border-radius: 8px; padding: 0.5rem 0.75rem; font-size: 0.875rem; outline: none; transition: border-color 0.15s; color: #1e293b; background: #f8fafc; }',
      '.lc-chat-input:focus { border-color: ' + cfg.primary_color + '; background: #fff; }',
      '.lc-chat-send { background: ' + cfg.primary_color + '; color: #fff; border: none; border-radius: 8px; padding: 0.5rem 0.875rem; cursor: pointer; font-size: 0.875rem; font-weight: 500; white-space: nowrap; transition: opacity 0.15s; }',
      '.lc-chat-send:hover { opacity: 0.88; }',
      '.lc-chat-send:disabled { opacity: 0.5; cursor: not-allowed; }',

      /* ── Legacy IDs kept for corner floating mode ── */
      '#lc-chat-header { background: ' + cfg.primary_color + '; color: #fff; padding: 0.875rem 1rem; display: flex; align-items: center; justify-content: space-between; }',
      '#lc-chat-header-title { font-weight: 600; font-size: 0.9375rem; }',
      '#lc-chat-close { background: none; border: none; color: rgba(255,255,255,0.8); cursor: pointer; font-size: 1.25rem; line-height: 1; padding: 0; display: flex; align-items: center; justify-content: center; width: 24px; height: 24px; border-radius: 4px; transition: background 0.1s; }',
      '#lc-chat-close:hover { background: rgba(255,255,255,0.15); color: #fff; }',
      '#lc-chat-body { flex: 1; overflow-y: auto; padding: 1rem; display: flex; flex-direction: column; gap: 0.75rem; min-height: 120px; }',
      '#lc-chat-footer { padding: 0.75rem; border-top: 1px solid #f1f5f9; display: flex; gap: 0.5rem; }',
      '#lc-chat-input { flex: 1; border: 1px solid #e2e8f0; border-radius: 8px; padding: 0.5rem 0.75rem; font-size: 0.875rem; outline: none; transition: border-color 0.15s; color: #1e293b; background: #f8fafc; }',
      '#lc-chat-input:focus { border-color: ' + cfg.primary_color + '; background: #fff; }',
      '#lc-chat-send { background: ' + cfg.primary_color + '; color: #fff; border: none; border-radius: 8px; padding: 0.5rem 0.875rem; cursor: pointer; font-size: 0.875rem; font-weight: 500; white-space: nowrap; transition: opacity 0.15s; }',
      '#lc-chat-send:hover { opacity: 0.88; }',
      '#lc-chat-send:disabled { opacity: 0.5; cursor: not-allowed; }',
    ].join('\n');

    var style = document.createElement('style');
    style.id = 'lc-chat-widget-styles';
    style.textContent = css;
    document.head.appendChild(style);
  }

  // ── Chat icon SVG (speech bubble) ──────────────────────────────────────────
  var CHAT_ICON_SVG =
    '<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor">' +
      '<path d="M12 2C6.477 2 2 6.0 2 11c0 2.13.772 4.09 2.05 5.63L2.5 21.5l5.13-1.47A10.04 10.04 0 0012 21c5.523 0 10-4.0 10-9S17.523 2 12 2z"/>' +
    '</svg>';

  // ── Build panel HTML (shared between both modes) ───────────────────────────
  function buildPanelHTML(cfg, closeId, headerId, bodyId, inputId, sendId) {
    return '<div class="lc-chat-header" id="' + headerId + '">' +
        '<span class="lc-chat-header-title">' + escHtml(cfg.title) + '</span>' +
        '<button class="lc-chat-close" id="' + closeId + '" aria-label="Close chat">&times;</button>' +
      '</div>' +
      '<div class="lc-chat-body" id="' + bodyId + '">' +
        '<p class="lc-welcome">' + escHtml(cfg.welcome_message) + '</p>' +
      '</div>' +
      '<div class="lc-chat-footer">' +
        '<input class="lc-chat-input" id="' + inputId + '" type="text" placeholder="' + escHtml(cfg.placeholder) + '" autocomplete="off" maxlength="200">' +
        '<button class="lc-chat-send" id="' + sendId + '">Ask</button>' +
      '</div>';
  }

  // ── Nav-mode overlay ───────────────────────────────────────────────────────
  function buildOverlay(cfg) {
    var existing = document.getElementById('lc-chat-overlay');
    if (existing) existing.remove();

    var overlay = document.createElement('div');
    overlay.id = 'lc-chat-overlay';
    overlay.setAttribute('role', 'dialog');
    overlay.setAttribute('aria-label', cfg.title);

    var panel = document.createElement('div');
    panel.id = 'lc-chat-overlay-panel';
    panel.innerHTML = buildPanelHTML(cfg, 'lc-ov-close', 'lc-ov-header', 'lc-ov-body', 'lc-ov-input', 'lc-ov-send');

    overlay.appendChild(panel);
    document.body.appendChild(overlay);

    // Close on backdrop click
    overlay.addEventListener('click', function (e) {
      if (e.target === overlay) closeOverlay();
    });
    document.getElementById('lc-ov-close').addEventListener('click', closeOverlay);

    wireChat('lc-ov-body', 'lc-ov-input', 'lc-ov-send', cfg);
  }

  function openOverlay() {
    var ov = document.getElementById('lc-chat-overlay');
    if (ov) {
      ov.classList.add('open');
      var inp = document.getElementById('lc-ov-input');
      if (inp) setTimeout(function () { inp.focus(); }, 50);
    }
  }

  function closeOverlay() {
    var ov = document.getElementById('lc-chat-overlay');
    if (ov) ov.classList.remove('open');
  }

  // ── Nav button injection ───────────────────────────────────────────────────
  function injectNavButton(cfg) {
    var navSearch = document.getElementById('nav-search-wrap');
    if (!navSearch) return false;

    var existing = document.getElementById('lc-nav-chat-btn');
    if (existing) existing.remove();

    var btn = document.createElement('button');
    btn.id = 'lc-nav-chat-btn';
    btn.setAttribute('aria-label', 'Open chat');
    btn.setAttribute('title', cfg.title);
    btn.innerHTML = CHAT_ICON_SVG;
    btn.addEventListener('click', openOverlay);

    // Append inside the wrap (flex container) so it sits right of the search button
    navSearch.appendChild(btn);
    return true;
  }

  // ── Floating corner widget (fallback) ─────────────────────────────────────
  function buildWidget(cfg) {
    var existing = document.getElementById(WIDGET_ID);
    if (existing) existing.remove();

    var container = document.createElement('div');
    container.id = WIDGET_ID;

    container.innerHTML =
      '<button id="lc-chat-btn" aria-label="Open chat" title="' + escHtml(cfg.title) + '">' +
        CHAT_ICON_SVG.replace('fill="currentColor"', 'fill="#fff"') +
      '</button>' +
      '<div id="lc-chat-panel" hidden role="dialog" aria-label="' + escHtml(cfg.title) + '">' +
        '<div id="lc-chat-header">' +
          '<span id="lc-chat-header-title">' + escHtml(cfg.title) + '</span>' +
          '<button id="lc-chat-close" aria-label="Close chat">&times;</button>' +
        '</div>' +
        '<div id="lc-chat-body">' +
          '<p class="lc-welcome">' + escHtml(cfg.welcome_message) + '</p>' +
        '</div>' +
        '<div id="lc-chat-footer">' +
          '<input id="lc-chat-input" type="text" placeholder="' + escHtml(cfg.placeholder) + '" autocomplete="off" maxlength="200">' +
          '<button id="lc-chat-send">Ask</button>' +
        '</div>' +
      '</div>';

    document.body.appendChild(container);

    var btn = document.getElementById('lc-chat-btn');
    var panel = document.getElementById('lc-chat-panel');
    document.getElementById('lc-chat-close').addEventListener('click', function () { panel.hidden = true; });
    btn.addEventListener('click', function () { panel.hidden = false; document.getElementById('lc-chat-input').focus(); });

    wireChat('lc-chat-body', 'lc-chat-input', 'lc-chat-send', cfg);
  }

  function escHtml(str) {
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  // Minimal inline markdown → safe HTML (escape first, then transform)
  function renderMarkdown(text) {
    var s = text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
    s = s.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>');
    s = s.replace(/\*([^*\n]+)\*/g, '<em>$1</em>');
    // Markdown links — external first, then relative
    s = s.replace(/\[([^\]]+)\]\((https?:\/\/[^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
    s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>');
    // Auto-link bare https:// URLs not already inside an href attribute
    s = s.replace(/(?<!href=")(https?:\/\/[^\s<>")\]]+)/g, '<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>');
    // Auto-link bare relative paths like /about or /projects/foo
    s = s.replace(/(?<![="<\/])(\/[a-zA-Z][a-zA-Z0-9\-_\/]*)/g, '<a href="$1">$1</a>');
    s = s.replace(/\n/g, '<br>');
    return s;
  }

  // ── Shared chat interaction wiring ────────────────────────────────────────
  function wireChat(bodyId, inputId, sendId, cfg) {
    var sendBtn = document.getElementById(sendId);
    var input = document.getElementById(inputId);

    function doQuery() {
      var q = input.value.trim();
      if (!q) return;
      runQuery(q, cfg, bodyId, inputId, sendId);
    }

    sendBtn.addEventListener('click', doQuery);
    input.addEventListener('keydown', function (e) { if (e.key === 'Enter') doQuery(); });
    input.addEventListener('focus', function () { input.style.borderColor = cfg.primary_color; input.style.background = '#fff'; });
    input.addEventListener('blur', function () { input.style.borderColor = '#e2e8f0'; input.style.background = '#f8fafc'; });
  }

  function runQuery(query, cfg, bodyId, inputId, sendId) {
    var body = document.getElementById(bodyId);
    var sendBtn = document.getElementById(sendId);
    var input = document.getElementById(inputId);

    body.innerHTML = '<div class="lc-loading"><div class="lc-dot"></div><div class="lc-dot"></div><div class="lc-dot"></div></div>';
    sendBtn.disabled = true;

    var loadingRemoved = false;
    var answerEl = null;
    var sourcesEl = null;
    var rawAnswer = '';

    function removeLoading() {
      if (!loadingRemoved) {
        var l = body.querySelector('.lc-loading');
        if (l) l.remove();
        loadingRemoved = true;
      }
    }

    function ensureAnswer() {
      if (!answerEl) {
        removeLoading();
        answerEl = document.createElement('div');
        answerEl.className = 'lc-answer';
        body.appendChild(answerEl);
      }
      return answerEl;
    }

    function handleEvent(evt) {
      if (evt.type === 'token' && evt.text) {
        rawAnswer += evt.text;
        ensureAnswer().innerHTML = renderMarkdown(rawAnswer);
        body.scrollTop = body.scrollHeight;
      } else if (evt.type === 'sources') {
        removeLoading();
        var results = evt.results || [];
        if (results.length === 0 && !answerEl) {
          body.innerHTML = '<p class="lc-no-results">No results found. Try rephrasing your question.</p>';
          return;
        }
        if (results.length > 0) {
          sourcesEl = document.createElement('div');
          sourcesEl.className = 'lc-sources';
          var html = '<div class="lc-sources-label">Sources</div>';
          for (var i = 0; i < results.length; i++) {
            var r = results[i];
            html += '<a class="lc-result" href="' + escHtml(r.url) + '">' +
              '<span class="lc-result-arrow">&#8599;</span>' +
              '<span class="lc-result-title">' + escHtml(r.title) + '</span>' +
              '</a>';
          }
          sourcesEl.innerHTML = html;
          body.appendChild(sourcesEl);
          body.scrollTop = body.scrollHeight;
        }
      } else if (evt.type === 'done') {
        sendBtn.disabled = false;
        input.value = '';
        input.focus();
      }
    }

    fetch(baseURL + '/api/chat?q=' + encodeURIComponent(query))
      .then(function (res) {
        if (!res.ok || !res.body) throw new Error('HTTP ' + res.status);
        var reader = res.body.getReader();
        var decoder = new TextDecoder();
        var buffer = '';

        function pump() {
          return reader.read().then(function (result) {
            if (result.done) { sendBtn.disabled = false; return; }
            buffer += decoder.decode(result.value, { stream: true });
            var parts = buffer.split('\n\n');
            buffer = parts.pop();
            for (var i = 0; i < parts.length; i++) {
              var p = parts[i].trim();
              if (p.indexOf('data: ') === 0) {
                try { handleEvent(JSON.parse(p.slice(6))); } catch (e) {}
              }
            }
            return pump();
          });
        }
        return pump();
      })
      .catch(function () {
        sendBtn.disabled = false;
        body.innerHTML = '<p class="lc-no-results">Something went wrong. Please try again.</p>';
      });
  }

  function init() {
    fetch(baseURL + '/api/chat/config')
      .then(function (res) { return res.json(); })
      .then(function (cfg) {
        config = cfg;
        injectStyles(cfg);
        buildOverlay(cfg);

        // Try nav injection first — always inject nav button regardless of enabled state
        useNavMode = injectNavButton(cfg);
        if (!useNavMode) {
          // Only show floating corner widget when explicitly enabled
          if (cfg.enabled) {
            buildWidget(cfg);
          }
        }
      })
      .catch(function () {});
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
