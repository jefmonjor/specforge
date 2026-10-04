(() => {
  const visible = (e) => {
    const s = getComputedStyle(e);
    const r = e.getBoundingClientRect();
    return s.display !== 'none' && s.visibility !== 'hidden' && Number(s.opacity) !== 0 && r.width > 0 && r.height > 0;
  };
  const esc = (v) => CSS.escape(v);
  const unique = (sel) => {
    try { return document.querySelectorAll(sel).length === 1; } catch (e) { return false; }
  };
  const path = (el) => {
    const parts = [];
    while (el && el.nodeType === 1 && el !== document.body && el !== document.documentElement) {
      let i = 1;
      let s = el;
      while ((s = s.previousElementSibling)) if (s.tagName === el.tagName) i++;
      parts.unshift(el.tagName.toLowerCase() + ':nth-of-type(' + i + ')');
      el = el.parentElement;
    }
    return 'body > ' + parts.join(' > ');
  };
  const selectorFor = (el) => {
    const tag = el.tagName.toLowerCase();
    const candidates = [];
    if (el.id) candidates.push('#' + esc(el.id));
    for (const [attr, prefix] of [['data-testid', ''], ['name', tag], ['aria-label', tag], ['placeholder', tag]]) {
      const v = el.getAttribute(attr);
      if (v) candidates.push(prefix + '[' + attr + '="' + esc(v) + '"]');
    }
    for (const c of candidates) if (unique(c)) return c;
    return path(el);
  };
  const query = 'a, button, input, select, textarea, [role=button], [role=link], [role=checkbox], [role=tab], [role=menuitem], [role=alert], [contenteditable=true], label, h1, h2, h3, [data-testid]';
  const elements = Array.from(document.querySelectorAll(query)).filter(visible).slice(0, 200).map((e) => ({
    tag: e.tagName.toLowerCase(),
    type: e.getAttribute('type') || '',
    text: ((e.innerText || e.value || '') + '').trim().replace(/\s+/g, ' ').slice(0, 80),
    placeholder: e.getAttribute('placeholder') || '',
    aria_label: e.getAttribute('aria-label') || '',
    selector: selectorFor(e),
  }));
  return JSON.stringify({
    url: location.href,
    title: document.title,
    elements: elements,
    text: (document.body ? document.body.innerText : '').slice(0, 8000),
  });
})()
