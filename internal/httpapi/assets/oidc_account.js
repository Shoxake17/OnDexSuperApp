// OnDex ID akkaunt sahifasi (passport.ondex.uz, Plane 270). Freymvorksiz.
//
// Sessiya — faqat HttpOnly SSO cookie; token sahifaga umuman kelmaydi.
// Har bir o'zgartiruvchi so'rov `X-OnDex-Account: 1` sarlavhasi bilan
// (server CSRF tekshiruvi). Ma'lumot DOM'ga faqat textContent orqali.
(function () {
  'use strict';
  function $(id) { return document.getElementById(id); }
  function showErr(m) { var e = $('err'); e.textContent = m; e.hidden = false; $('ok').hidden = true; }
  function showOk(m) { var e = $('ok'); e.textContent = m; e.hidden = false; $('err').hidden = true; }
  function clearMsg() { $('err').hidden = true; $('ok').hidden = true; }

  function toLogin() { location.replace('/v2/oauth/login?account=1'); }

  function call(method, path, body) {
    var h = { 'Accept': 'application/json' };
    if (method !== 'GET') { h['Content-Type'] = 'application/json'; h['X-OnDex-Account'] = '1'; }
    return fetch(path, { method: method, headers: h, credentials: 'same-origin', cache: 'no-store',
      body: method === 'GET' ? undefined : JSON.stringify(body || {}) })
      .then(function (r) {
        return r.text().then(function (t) {
          var d = {};
          try { d = t ? JSON.parse(t) : {}; } catch (e) { d = {}; }
          if (r.status === 401) { toLogin(); throw new Error('Sessiya tugagan'); }
          if (!r.ok) {
            var msg = d.error || 'Xato yuz berdi';
            if (r.status === 429) { msg = "Juda ko'p urinish. Birozdan so'ng qayta urinib ko'ring."; }
            throw new Error(msg);
          }
          return d;
        });
      }, function () { throw new Error("Internetga ulanib bo'lmadi"); });
  }

  function fmt(iso) {
    var d = new Date(iso);
    if (isNaN(d.getTime())) { return ''; }
    var p = function (n) { return (n < 10 ? '0' : '') + n; };
    return p(d.getDate()) + '.' + p(d.getMonth() + 1) + '.' + d.getFullYear() + ' ' + p(d.getHours()) + ':' + p(d.getMinutes());
  }

  function li(text, meta, btn) {
    var el = document.createElement('li');
    var left = document.createElement('div');
    var t = document.createElement('div'); t.textContent = text; left.appendChild(t);
    if (meta) { var m = document.createElement('div'); m.className = 'meta'; m.textContent = meta; left.appendChild(m); }
    el.appendChild(left);
    if (btn) { el.appendChild(btn); }
    return el;
  }

  function badge(el, ok, yes, no) {
    el.textContent = ok ? yes : no;
    el.className = 'badge ' + (ok ? 'ok' : 'warn');
  }

  // Ikki bosishli tasdiq (brauzer dialogisiz): birinchi bosish — "Tasdiqlang".
  function confirmBtn(btn, label, fn) {
    var armed = false, timer = null;
    btn.addEventListener('click', function () {
      if (!armed) {
        armed = true; btn.textContent = 'Tasdiqlash uchun yana bosing';
        timer = setTimeout(function () { armed = false; btn.textContent = label; }, 4000);
        return;
      }
      clearTimeout(timer); armed = false; btn.textContent = label;
      fn();
    });
  }

  var totp = false;

  function render(d) {
    var u = d.user || {};
    $('name').textContent = u.name || 'OnDex foydalanuvchisi';
    $('phone').textContent = u.phone || '';
    var av = $('avatar');
    av.textContent = '';
    if (d.picture && /^https:\/\//.test(d.picture)) {
      var img = document.createElement('img');
      img.src = d.picture; img.alt = ''; img.className = 'avatar';
      av.replaceWith(img); img.id = 'avatar';
    } else {
      av.textContent = (u.name || '?').trim().charAt(0).toUpperCase() || '?';
    }
    totp = !!d.totp_enabled;
    badge($('pwd-badge'), d.password_set, "O'rnatilgan", "O'rnatilmagan");
    badge($('totp-badge'), totp, 'Yoqilgan', "O'chiq");
    $('code-box').hidden = !totp;
    $('btn-pwd-open').hidden = !d.password_set;

    var apps = $('apps'); apps.textContent = '';
    (d.connected_apps || []).forEach(function (a) { apps.appendChild(li(a, 'Faol sessiya bor')); });
    if (!apps.children.length) { apps.appendChild(li("Hozircha hech qaysi xizmatda faol sessiya yo'q")); }

    var biz = $('biz'); biz.textContent = '';
    (d.businesses || []).forEach(function (b) {
      biz.appendChild(li(b.name || 'Biznes', b.is_owner ? 'Egasi' : (b.role || 'Xodim')));
    });

    var list = $('sessions'); list.textContent = '';
    (d.sessions || []).forEach(function (s) {
      var title = s.app + (s.device && s.device !== s.app ? ' — ' + s.device : '');
      var meta = (s.current ? 'Shu brauzer · ' : '') + 'oxirgi faollik: ' + fmt(s.last_used_at);
      var b = document.createElement('button');
      b.type = 'button'; b.className = 'small danger'; b.textContent = 'Chiqarish';
      confirmBtn(b, 'Chiqarish', function () {
        clearMsg();
        call('POST', '/v2/oauth/account/api/sessions/revoke', { id: s.id }).then(function (r) {
          if (r.signed_out) { toLogin(); return; }
          showOk('Qurilma chiqarildi.'); load();
        }).catch(function (e) { showErr(e.message); });
      });
      list.appendChild(li(title, meta, b));
    });
  }

  // ── passkey (AUD-4) ────────────────────────────────────────────────────
  var pkSupported = !!(window.PublicKeyCredential && navigator.credentials && navigator.credentials.create);
  function b64uToBuf(s) {
    s = String(s || '').replace(/-/g, '+').replace(/_/g, '/');
    while (s.length % 4) { s += '='; }
    var bin = atob(s), out = new Uint8Array(bin.length);
    for (var i = 0; i < bin.length; i++) { out[i] = bin.charCodeAt(i); }
    return out.buffer;
  }
  function bufToB64u(buf) {
    var b = new Uint8Array(buf), s = '';
    for (var i = 0; i < b.length; i++) { s += String.fromCharCode(b[i]); }
    return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  }
  function loadPasskeys() {
    return call('GET', '/v2/oauth/account/api/passkeys').then(function (d) {
      var list = $('pk-list'); list.textContent = '';
      (d.passkeys || []).forEach(function (p) {
        var meta = "qo'shilgan: " + fmt(p.created_at) + (p.last_used_at ? ' · oxirgi kirish: ' + fmt(p.last_used_at) : '') +
          (p.synced ? ' · bulutda sinxron' : '');
        var b = document.createElement('button');
        b.type = 'button'; b.className = 'small danger'; b.textContent = "O'chirish";
        b.addEventListener('click', function () {
          clearMsg();
          var code = $('pk-code').value.replace(/\s/g, '');
          if (!code) { $('pk-form').hidden = false; showErr("O'chirish uchun Authenticator kodini kiriting, keyin yana bosing"); $('pk-code').focus(); return; }
          b.disabled = true;
          call('POST', '/v2/oauth/account/api/passkeys/delete', { id: p.id, code: code }).then(function () {
            $('pk-code').value = ''; showOk("Passkey o'chirildi."); loadPasskeys();
          }).catch(function (e) { showErr(e.message); b.disabled = false; });
        });
        list.appendChild(li(p.name, meta, b));
      });
      if (!list.children.length) { list.appendChild(li("Hozircha passkey yo'q")); }
    }).catch(function (e) { showErr(e.message); });
  }
  $('btn-pk-open').addEventListener('click', function () {
    $('pk-form').hidden = !$('pk-form').hidden;
    if (!$('pk-form').hidden) { $('pk-name').focus(); }
  });
  $('btn-pk-add').addEventListener('click', function () {
    var btn = this; clearMsg();
    var code = $('pk-code').value.replace(/\s/g, ''), name = $('pk-name').value.trim();
    if (!code) { showErr('Authenticator kodini kiriting'); return; }
    btn.disabled = true;
    call('POST', '/v2/oauth/account/api/passkeys/options', { code: code }).then(function (o) {
      var pk = o.public_key || {};
      pk.challenge = b64uToBuf(pk.challenge);
      pk.user.id = b64uToBuf(pk.user.id);
      (pk.excludeCredentials || []).forEach(function (c) { c.id = b64uToBuf(c.id); });
      return navigator.credentials.create({ publicKey: pk }).then(function (cred) {
        var r = cred.response;
        var body = {
          id: cred.id, rawId: bufToB64u(cred.rawId), type: cred.type,
          authenticatorAttachment: cred.authenticatorAttachment || undefined,
          clientExtensionResults: cred.getClientExtensionResults ? cred.getClientExtensionResults() : {},
          response: { clientDataJSON: bufToB64u(r.clientDataJSON), attestationObject: bufToB64u(r.attestationObject),
            transports: r.getTransports ? r.getTransports() : undefined }
        };
        return call('POST', '/v2/oauth/account/api/passkeys', { state: o.state, credential: body, name: name });
      }, function (e) {
        if (e && e.name === 'InvalidStateError') { throw new Error('Bu qurilmadagi passkey allaqachon ulangan'); }
        throw new Error('Passkey yaratilmadi yoki bekor qilindi');
      });
    }).then(function () {
      $('pk-code').value = ''; $('pk-name').value = ''; $('pk-form').hidden = true;
      showOk("Passkey qo'shildi. Keyingi kirishda Authenticator kodi o'rniga ishlatishingiz mumkin."); loadPasskeys();
    }).catch(function (e) { showErr(e.message); }).then(function () { btn.disabled = false; });
  });

  function load() {
    return call('GET', '/v2/oauth/account/api/me').then(function (d) {
      $('loading').hidden = true; $('content').hidden = false; render(d);
      // Passkey faqat Authenticator ulangan akkauntda (zaxira + step-up).
      $('pk-box').hidden = !(d.passkeys_enabled && totp && pkSupported);
      if (!$('pk-box').hidden) { loadPasskeys(); }
    }).catch(function (e) { $('loading').hidden = true; showErr(e.message); });
  }

  $('btn-pwd-open').addEventListener('click', function () {
    $('pwd-form').hidden = !$('pwd-form').hidden;
    if (!$('pwd-form').hidden) { $('cur').focus(); }
  });

  $('btn-pwd').addEventListener('click', function () {
    var btn = this; clearMsg();
    var cur = $('cur').value, a = $('np1').value, b = $('np2').value, code = $('code').value.replace(/\s/g, '');
    if (!cur) { showErr('Joriy parolni kiriting'); return; }
    if (a.length < 8) { showErr("Yangi parol kamida 8 belgi bo'lsin"); return; }
    if (a !== b) { showErr('Yangi parollar mos kelmadi'); return; }
    if (totp && !code) { showErr('Authenticator kodini kiriting'); return; }
    btn.disabled = true;
    call('POST', '/v2/oauth/account/api/password',
      { current_password: cur, password: a, password_confirm: b, code: code })
      .then(function () { toLogin(); })
      .catch(function (e) { showErr(e.message); })
      .then(function () { btn.disabled = false; });
  });

  confirmBtn($('btn-all'), 'Hamma qurilmalardan chiqish', function () {
    clearMsg();
    call('POST', '/v2/oauth/account/api/logout-all').then(toLogin).catch(function (e) { showErr(e.message); });
  });

  $('btn-logout').addEventListener('click', function () {
    clearMsg();
    call('POST', '/v2/oauth/account/api/logout').then(toLogin).catch(function (e) { showErr(e.message); });
  });

  load();
})();
