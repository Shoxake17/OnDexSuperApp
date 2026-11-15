// OnDex — `passkeys_web` plagini uchun `PasskeyAuthenticator` (RS-7 web panel).
//
// Plagin bu obyektni Corbado'ning tashqi `bundle.js` skriptidan kutadi; u
// bo'lmasa ilova ishga tushishdayoq yiqiladi (oq ekran). Uchinchi tomon
// skriptini yuklash o'rniga shu kichik fayl brauzerning STANDART WebAuthn
// API'si ustida o'sha interfeysni beradi: CSP `script-src 'self'` saqlanadi,
// begona kod yo'q. Kirish/chiqish — WebAuthn JSON (base64url maydonlar).
(function () {
  'use strict';

  function b64urlToBuf(s) {
    const pad = '='.repeat((4 - (s.length % 4)) % 4);
    const bin = atob((s + pad).replace(/-/g, '+').replace(/_/g, '/'));
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out.buffer;
  }

  function bufToB64url(buf) {
    if (!buf) return null;
    const bytes = new Uint8Array(buf);
    let bin = '';
    for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
    return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  }

  function creds(list) {
    return (list || []).map(function (c) {
      return { type: c.type || 'public-key', id: b64urlToBuf(c.id), transports: c.transports };
    });
  }

  function extensions(pk) {
    const ext = pk.extensions || pk.loginExtensions;
    if (!ext) return undefined;
    const out = {};
    if (ext.credProps) out.credProps = true;
    if (ext.prf && ext.prf.eval && ext.prf.eval.first) {
      out.prf = { eval: { first: b64urlToBuf(ext.prf.eval.first) } };
    }
    return out;
  }

  function prfOut(res) {
    const r = {};
    if (res && res.credProps) r.credProps = res.credProps;
    if (res && res.prf) {
      r.prf = { enabled: res.prf.enabled };
      if (res.prf.results && res.prf.results.first) r.prf.results = { first: bufToB64url(res.prf.results.first) };
    }
    return r;
  }

  // Xato — plagin `_parseException` kutgan JSON matn ({code,message,details}).
  function fail(e) {
    const name = (e && e.name) || 'Error';
    let code = 'unknown';
    if (name === 'NotAllowedError' || name === 'AbortError') code = 'cancelled';
    else if (name === 'InvalidStateError') code = 'exclude-credentials-match';
    else if (name === 'NotSupportedError') code = 'passkey-not-supported';
    else if (name === 'SecurityError') code = 'domain-not-associated';
    return Promise.reject(JSON.stringify({ code: code, message: String((e && e.message) || name), details: name }));
  }

  let controller = null;
  function signal() {
    if (controller) controller.abort();
    controller = new AbortController();
    return controller.signal;
  }

  const supported = typeof window.PublicKeyCredential === 'function' && !!navigator.credentials;

  window.PasskeyAuthenticator = {
    init: function () {},

    hasPasskeySupport: function () {
      return supported;
    },

    isUserVerifyingPlatformAuthenticatorAvailable: function () {
      if (!supported) return Promise.resolve(false);
      return PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable();
    },

    isConditionalMediationAvailable: function () {
      if (!supported || !PublicKeyCredential.isConditionalMediationAvailable) return Promise.resolve(false);
      return PublicKeyCredential.isConditionalMediationAvailable();
    },

    cancelCurrentAuthenticatorOperation: function () {
      if (controller) controller.abort();
      controller = null;
    },

    register: function (json) {
      let req;
      try {
        req = JSON.parse(json);
      } catch (e) {
        return fail(e);
      }
      const pk = req.publicKey;
      const opts = {
        publicKey: {
          rp: pk.rp,
          user: { id: b64urlToBuf(pk.user.id), name: pk.user.name, displayName: pk.user.displayName },
          challenge: b64urlToBuf(pk.challenge),
          pubKeyCredParams: pk.pubKeyCredParams,
          authenticatorSelection: pk.authenticatorSelection || undefined,
          excludeCredentials: creds(pk.excludeCredentials),
          timeout: pk.timeout || undefined,
          attestation: pk.attestation || 'none',
          extensions: extensions(pk),
        },
        signal: signal(),
      };
      if (req.mediation) opts.mediation = req.mediation;
      return navigator.credentials.create(opts).then(function (c) {
        const r = c.response;
        return JSON.stringify({
          id: c.id,
          rawId: bufToB64url(c.rawId),
          type: c.type,
          response: {
            clientDataJSON: bufToB64url(r.clientDataJSON),
            attestationObject: bufToB64url(r.attestationObject),
            transports: r.getTransports ? r.getTransports() : [],
          },
          clientExtensionResults: prfOut(c.getClientExtensionResults ? c.getClientExtensionResults() : {}),
        });
      }, fail);
    },

    login: function (json) {
      let req;
      try {
        req = JSON.parse(json);
      } catch (e) {
        return fail(e);
      }
      const pk = req.publicKey;
      const opts = {
        publicKey: {
          challenge: b64urlToBuf(pk.challenge),
          rpId: pk.rpId || undefined,
          allowCredentials: creds(pk.allowCredentials),
          userVerification: pk.userVerification || undefined,
          timeout: pk.timeout || undefined,
          extensions: extensions(pk),
        },
        signal: signal(),
      };
      if (req.mediation && req.mediation !== 'optional') opts.mediation = req.mediation;
      return navigator.credentials.get(opts).then(function (c) {
        const r = c.response;
        return JSON.stringify({
          id: c.id,
          rawId: bufToB64url(c.rawId),
          type: c.type,
          response: {
            clientDataJSON: bufToB64url(r.clientDataJSON),
            authenticatorData: bufToB64url(r.authenticatorData),
            signature: bufToB64url(r.signature),
            userHandle: bufToB64url(r.userHandle),
          },
          clientExtensionResults: prfOut(c.getClientExtensionResults ? c.getClientExtensionResults() : {}),
        });
      }, fail);
    },
  };
})();
