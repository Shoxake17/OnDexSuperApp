// OnDex kirish sahifasi (OIDC, A4c.2). Freymvorksiz, tashqi skriptsiz.
//
// Oqim: merchant kirish API'lari client_id=ondex-sso bilan -> ondex-sso
// sessiyasining access tokeni -> POST /v2/oauth/login/complete (Bearer)
// -> server SSO cookie qo'yadi va ilovaga qaytish manzilini beradi.
// Tokenlar faqat shu sahifa xotirasida va faqat shu so'rovgacha yashaydi:
// localStorage/sessionStorage ga YOZILMAYDI.
(function () {
  'use strict';
  var CLIENT = 'ondex-sso';
  var DEVICE = 'Brauzer (OnDex kirish)';
  var params = new URLSearchParams(location.search);
  var REQ = params.get('req') || '';
  // Akkaunt sahifasiga kirish (passport.ondex.uz): ilova so'rovi yo'q.
  var ACCOUNT = !REQ && params.get('account') === '1';
  var phone = '';
  var mfaToken = '';
  var pendingAccess = '';

  function $(id) { return document.getElementById(id); }
  var sections = ['s-start', 's-code', 's-fpwd', 's-setpwd', 's-totp', 's-enroll', 's-recovery'];
  var codeMode = 'login'; // login | factor

  function show(id) {
    sections.forEach(function (s) { $(s).hidden = s !== id; });
    hideErr();
    var f = $(id).querySelector('input');
    if (f) { setTimeout(function () { f.focus(); }, 0); }
  }
  function showErr(msg) { var e = $('err'); e.textContent = msg; e.hidden = false; }
  function hideErr() { $('err').hidden = true; }
  function busy(btn, on) { btn.disabled = on; }

  function normPhone(v) {
    var d = String(v || '').replace(/\D/g, '');
    if (d.length === 9) { d = '998' + d; }
    return /^998\d{9}$/.test(d) ? '+' + d : '';
  }

  function api(path, body, bearer) {
    var h = { 'Content-Type': 'application/json', 'Accept': 'application/json' };
    if (bearer) { h['Authorization'] = 'Bearer ' + bearer; }
    return fetch(path, { method: 'POST', headers: h, body: JSON.stringify(body || {}),
      credentials: 'same-origin', cache: 'no-store' })
      .then(function (r) {
        return r.text().then(function (t) {
          var d = {};
          try { d = t ? JSON.parse(t) : {}; } catch (e) { d = {}; }
          if (!r.ok) {
            var msg = d.error_description || d.error || 'Xato yuz berdi';
            if (r.status === 429) { msg = "Juda ko'p urinish. Birozdan so'ng qayta urinib ko'ring."; }
            var err = new Error(msg); err.status = r.status; throw err;
          }
          return d;
        });
      }, function () { throw new Error("Internetga ulanib bo'lmadi"); });
  }

  // Natija: tokenlar (kirish tugadi) yoki ikkinchi bosqich.
  function outcome(d) {
    if (d && d.access_token) { return complete(d.access_token); }
    if (!d || !d.mfa_required) { throw new Error('Kutilmagan javob'); }
    mfaToken = d.mfa_token;
    switch (d.method) {
      case 'otp': return startFactorCode();
      case 'password': show('s-fpwd'); return;
      case 'password_set': show('s-setpwd'); return;
      case 'totp':
        // Passkey (AUD-4): akkauntda bo'lsa va brauzer qo'llasa — muqobil.
        $('btn-passkey').hidden = !(d.passkey && window.PublicKeyCredential && navigator.credentials);
        show('s-totp'); return;
      case 'totp_enroll': return startEnroll();
      default: throw new Error("Noma'lum tekshiruv usuli");
    }
  }

  function complete(access) {
    return api('/v2/oauth/login/complete', ACCOUNT ? { account: true } : { req: REQ }, access).then(function (d) {
      if (!d.redirect_to) { throw new Error('Qaytish manzili kelmadi'); }
      location.replace(d.redirect_to);
    });
  }

  function sendTelegram() {
    return api('/auth/telegram/start', { phone: phone }).then(function (d) {
      var a = $('tg-link');
      if (d && d.deep_link && /^https:\/\/t\.me\//.test(d.deep_link)) {
        a.href = d.deep_link; a.hidden = false;
        window.open(d.deep_link, '_blank', 'noopener,noreferrer');
      }
    });
  }

  function startFactorCode() {
    codeMode = 'factor';
    $('code-hint').textContent = 'Yangi qurilma: Telegram orqali kelgan kodni ham kiriting.';
    $('btn-back').hidden = true;
    show('s-code');
    return sendTelegram();
  }

  function startEnroll() {
    return api('/v2/auth/mfa/totp/enroll', { mfa_token: mfaToken }).then(function (d) {
      if (d.qr_png && /^data:image\/png;base64,/.test(d.qr_png)) { $('qr').src = d.qr_png; }
      $('secret').textContent = d.secret ? 'Kalit: ' + d.secret.replace(/(.{4})/g, '$1 ').trim() : '';
      show('s-enroll');
    });
  }

  function run(btn, fn) {
    hideErr(); busy(btn, true);
    Promise.resolve().then(fn).catch(function (e) { showErr(e.message || 'Xato yuz berdi'); })
      .then(function () { busy(btn, false); });
  }

  function needPhone() {
    phone = normPhone($('phone').value);
    if (!phone) { throw new Error("Telefon raqami noto'g'ri (+998 XX XXX XX XX)"); }
  }

  // ── boshlash: telefon -> "Davom etish" ────────────────────────────────
  // Asosiy yo'l — Telegram kodi: u HAMMA uchun ishlaydi (xodim, parolsiz
  // egasi, parolli egasi — keyingi bosqichni server o'zi aytadi). Parol —
  // ixtiyoriy havola. Akkaunt mavjudligi hech qaysi javobdan bilinmaydi.
  var pwdMode = false;
  function setMode(pwd) {
    pwdMode = pwd;
    $('pwd-box').hidden = !pwd;
    $('btn-mode').textContent = pwd ? 'Telegram kodi bilan kirish' : 'Parol bilan kirish';
    $('start-hint').textContent = pwd ? 'Keyin Telegram kodi yoki Authenticator so\'raladi.' : 'Kod Telegram orqali keladi.';
    hideErr();
    (pwd ? $('password') : $('phone')).focus();
  }
  $('btn-mode').addEventListener('click', function () { setMode(!pwdMode); });

  $('btn-next').addEventListener('click', function () {
    run(this, function () {
      needPhone();
      if (pwdMode) {
        var pw = $('password').value;
        if (!pw) { throw new Error('Parolni kiriting'); }
        return api('/v2/auth/merchant/password',
          { phone: phone, password: pw, client_id: CLIENT, device_name: DEVICE }).then(outcome);
      }
      codeMode = 'login';
      $('code-hint').textContent = "Telegram'ga kelgan kodni kiriting.";
      $('btn-back').hidden = false;
      return sendTelegram().then(function () { show('s-code'); });
    });
  });

  $('btn-code').addEventListener('click', function () {
    run(this, function () {
      var c = $('code').value.trim();
      if (!/^\d{4,8}$/.test(c)) { throw new Error("Kod noto'g'ri"); }
      if (codeMode === 'factor') {
        return api('/v2/auth/mfa/factor', { mfa_token: mfaToken, code: c, device_name: DEVICE }).then(outcome);
      }
      return api('/v2/auth/merchant/otp/verify',
        { phone: phone, code: c, client_id: CLIENT, device_name: DEVICE }).then(outcome);
    });
  });
  $('btn-back').addEventListener('click', function () { show('s-start'); });

  $('btn-fpwd').addEventListener('click', function () {
    run(this, function () {
      var pw = $('fpassword').value;
      if (!pw) { throw new Error('Parolni kiriting'); }
      return api('/v2/auth/mfa/factor', { mfa_token: mfaToken, password: pw, device_name: DEVICE }).then(outcome);
    });
  });

  $('btn-setpwd').addEventListener('click', function () {
    run(this, function () {
      var a = $('npassword').value, b = $('npassword2').value;
      if (a.length < 8) { throw new Error('Parol kamida 8 belgi bo\'lsin'); }
      if (a !== b) { throw new Error('Parollar mos kelmadi'); }
      return api('/v2/auth/mfa/password', { mfa_token: mfaToken, password: a, password_confirm: b }).then(outcome);
    });
  });

  $('btn-totp').addEventListener('click', function () {
    run(this, function () {
      var c = $('totp').value.replace(/\s/g, '');
      if (!c) { throw new Error('Kodni kiriting'); }
      return api('/v2/auth/mfa/verify', { mfa_token: mfaToken, code: c, device_name: DEVICE }).then(outcome);
    });
  });

  // ── passkey (WebAuthn) ─────────────────────────────────────────────────
  // Imzoni brauzer SHU sayt manziliga bog'laydi — soxta domenda yaratilgan
  // javob serverda rad etiladi (fishingga chidamli).
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
  $('btn-passkey').addEventListener('click', function () {
    run(this, function () {
      return api('/v2/auth/mfa/passkey/options', { mfa_token: mfaToken }).then(function (o) {
        var pk = o.public_key || {};
        pk.challenge = b64uToBuf(pk.challenge);
        (pk.allowCredentials || []).forEach(function (c) { c.id = b64uToBuf(c.id); });
        return navigator.credentials.get({ publicKey: pk }).then(function (cred) {
          if (!cred) { throw new Error('Passkey tanlanmadi'); }
          var r = cred.response;
          var body = {
            id: cred.id, rawId: bufToB64u(cred.rawId), type: cred.type,
            authenticatorAttachment: cred.authenticatorAttachment || undefined,
            clientExtensionResults: cred.getClientExtensionResults ? cred.getClientExtensionResults() : {},
            response: { clientDataJSON: bufToB64u(r.clientDataJSON), authenticatorData: bufToB64u(r.authenticatorData),
              signature: bufToB64u(r.signature), userHandle: r.userHandle ? bufToB64u(r.userHandle) : undefined }
          };
          return api('/v2/auth/mfa/passkey', { mfa_token: mfaToken, state: o.state, credential: body, device_name: DEVICE });
        }, function (e) {
          if (e && e.name === 'NotAllowedError') { throw new Error('Passkey tasdiqlanmadi yoki bekor qilindi'); }
          throw new Error('Passkey ishlamadi — Authenticator kodidan foydalaning');
        }).then(outcome);
      });
    });
  });

  $('btn-enroll').addEventListener('click', function () {
    run(this, function () {
      var c = $('ecode').value.trim();
      if (!/^\d{6}$/.test(c)) { throw new Error('6 xonali kodni kiriting'); }
      return api('/v2/auth/mfa/totp/confirm', { mfa_token: mfaToken, code: c, device_name: DEVICE })
        .then(function (d) {
          if (!d.tokens || !d.tokens.access_token) { throw new Error('Kutilmagan javob'); }
          pendingAccess = d.tokens.access_token;
          $('codes').textContent = (d.recovery_codes || []).join('\n');
          show('s-recovery');
        });
    });
  });
  $('saved').addEventListener('change', function () { $('btn-done').disabled = !this.checked; });
  $('btn-done').addEventListener('click', function () {
    run(this, function () { var a = pendingAccess; pendingAccess = ''; return complete(a); });
  });

  // Enter — joriy bo'limdagi asosiy tugma.
  document.addEventListener('keydown', function (e) {
    if (e.key !== 'Enter') { return; }
    var sec = sections.map($).filter(function (s) { return !s.hidden; })[0];
    if (!sec) { return; }
    var btn = Array.prototype.filter.call(sec.querySelectorAll('button.primary'),
      function (b) { return b.offsetParent !== null && !b.disabled; })[0];
    if (btn) { e.preventDefault(); btn.click(); }
  });

  function restartAllowed() {
    try {
      var now = Date.now();
      var hits = JSON.parse(sessionStorage.getItem('ondexRestart') || '[]').filter(function (t) { return now - t < 120000; });
      if (hits.length >= 2) { return false; }
      hits.push(now);
      sessionStorage.setItem('ondexRestart', JSON.stringify(hits));
      return true;
    } catch (e) { return false; }
  }

  // ── so'rovni tekshirish ───────────────────────────────────────────────
  if (ACCOUNT) {
    $('client').textContent = 'Akkauntingizni boshqarish uchun';
    show('s-start');
    return;
  }
  if (!REQ) {
    showErr("Kirish so'rovi yo'q — ilovadan qaytadan boshlang.");
    $('s-start').hidden = true;
    return;
  }
  fetch('/v2/oauth/login/info?req=' + encodeURIComponent(REQ), { cache: 'no-store', credentials: 'same-origin' })
    .then(function (r) { return r.json().then(function (d) { return { ok: r.ok, d: d }; }); })
    .then(function (x) {
      // Eskirgan so'rov (eski tab, uzoq kutish) — ilova yangi so'rov bilan qayta
      // boshlaydi. Aylana himoyasi: 2 daqiqada 2 martadan ortiq emas.
      if (!x.ok && x.d.restart_url && /^https?:\/\//.test(x.d.restart_url) && restartAllowed()) {
        $('s-start').hidden = true;
        $('client').textContent = "Kirish yangilanmoqda…";
        location.replace(x.d.restart_url);
        return;
      }
      if (!x.ok) { throw new Error(x.d.error_description || "So'rov yaroqsiz"); }
      if (x.d.client_name) { $('client').textContent = x.d.client_name + ' uchun'; }
      show('s-start');
    })
    .catch(function (e) { $('s-start').hidden = true; showErr(e.message); });
})();
