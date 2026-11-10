// Parolni ko'rsatish/yashirish ("ko'z") — OnDex ID kirish va akkaunt
// sahifalari uchun umumiy. Freymvorksiz, innerHTML'siz (CSP: script-src 'self').
//
// Xavfsizlik:
//   - ko'rinadigan rejimda imlo tekshiruvi/avto-to'g'rilash O'CHIQ (brauzerning
//     kengaytirilgan imlo tekshiruvi matnni bulutga yuborishi mumkin);
//   - yuborishdan oldin (OnDexEye.hideAll), bo'lim almashganda, sahifa
//     yashirilganda/tark etilganda parol yana yashiriladi — parol ekranda
//     "unutilib" qolmaydi va parol menejeri maydonni parol deb taniydi.
(function () {
  'use strict';
  var NS = 'http://www.w3.org/2000/svg';
  var pairs = [];

  function icon(open) {
    var s = document.createElementNS(NS, 'svg');
    s.setAttribute('viewBox', '0 0 24 24'); s.setAttribute('width', '20'); s.setAttribute('height', '20');
    s.setAttribute('aria-hidden', 'true'); s.setAttribute('focusable', 'false'); s.setAttribute('fill', 'none');
    s.setAttribute('stroke', 'currentColor'); s.setAttribute('stroke-width', '1.8');
    s.setAttribute('stroke-linecap', 'round'); s.setAttribute('stroke-linejoin', 'round');
    var p = document.createElementNS(NS, 'path');
    p.setAttribute('d', 'M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12Z');
    var c = document.createElementNS(NS, 'circle');
    c.setAttribute('cx', '12'); c.setAttribute('cy', '12'); c.setAttribute('r', '3');
    s.appendChild(p); s.appendChild(c);
    if (!open) {
      var l = document.createElementNS(NS, 'path');
      l.setAttribute('d', 'M3 3l18 18');
      s.appendChild(l);
    }
    return s;
  }

  function set(inp, btn, visible) {
    inp.type = visible ? 'text' : 'password';
    var label = visible ? 'Parolni yashirish' : "Parolni ko'rsatish";
    btn.setAttribute('aria-pressed', visible ? 'true' : 'false');
    btn.setAttribute('aria-label', label);
    btn.title = label;
    while (btn.firstChild) { btn.removeChild(btn.firstChild); }
    btn.appendChild(icon(!visible));
  }

  function hideAll() { pairs.forEach(function (x) { set(x[0], x[1], false); }); }

  Array.prototype.forEach.call(document.querySelectorAll('input[type="password"]'), function (inp) {
    inp.setAttribute('spellcheck', 'false');
    inp.setAttribute('autocapitalize', 'off');
    inp.setAttribute('autocorrect', 'off');
    var btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'eye';
    btn.setAttribute('aria-controls', inp.id);
    // Fokus maydonda qoladi (kursor joyi yo'qolmaydi).
    btn.addEventListener('mousedown', function (e) { e.preventDefault(); });
    btn.addEventListener('click', function (e) {
      e.preventDefault();
      set(inp, btn, inp.type === 'password');
      inp.focus();
    });
    set(inp, btn, false);
    inp.parentNode.appendChild(btn);
    pairs.push([inp, btn]);
  });

  document.addEventListener('visibilitychange', function () { if (document.hidden) { hideAll(); } });
  window.addEventListener('pagehide', hideAll);

  window.OnDexEye = { hideAll: hideAll };
})();
