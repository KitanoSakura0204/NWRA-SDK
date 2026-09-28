// "Aurora" example SDK login template. Talks to the user centre only through window.LoungeLogin
// (SDK/docs/LOGIN_TEMPLATES.md): no network, no storage; consent, captcha, the result screen and
// the hand-back to the game are done by the user centre after a call resolves with "done".
(async () => {
  const L = window.LoungeLogin;
  const cfg = await L.ready();
  const t = L.t;
  const $ = (s) => document.querySelector(s);
  const $$ = (s) => Array.from(document.querySelectorAll(s));
  const err = $('.au-error');
  let busy = false;

  const say = (msg) => {
    err.textContent = msg || '';
    err.hidden = !msg;
  };
  const step = (name) => {
    $$('section[data-step]').forEach((s) => (s.hidden = s.dataset.step !== name));
    const first = $$(`section[data-step="${name}"] input:not([type="checkbox"])`).find((i) => i.offsetParent !== null);
    if (first) {
      first.focus();
    }
  };
  const field = (form, name) => form.elements.namedItem(name);

  // Texts and branding (the host already chose the locale and checked every URL).
  $$('[data-t]').forEach((el) => (el.textContent = t(el.dataset.t)));
  const b = cfg.branding;
  if (/^#(?:[0-9a-f]{3}|[0-9a-f]{6})$/i.test(b.accent || '')) {
    document.documentElement.style.setProperty('--au-accent', b.accent);
  }
  $('.au-title').textContent = b.title || cfg.server.name;
  $('.au-sub').textContent = b.subtitle || cfg.client.name;
  const logo = b.logo || cfg.client.logoUrl || cfg.server.icon;
  if (logo) {
    const img = $('.au-logo');
    img.addEventListener('load', () => (img.hidden = false), { once: true });
    img.src = logo;
  }
  $('.au-secured').textContent = t('secured', { site: cfg.siteName });
  $('[data-do="cancel"]').hidden = !cfg.cancellable;
  $('[data-do="register"]').hidden = !cfg.registration;
  if (cfg.server.status === 2) {
    $('.au-notice').textContent = t('maintenance') + (cfg.server.maintenanceMessage ? ': ' + cfg.server.maintenanceMessage : '');
    $('.au-notice').hidden = false;
  }

  // Only what this page really offers (cfg.methods already accounts for the site and the browser).
  const has = (m) => cfg.methods.includes(m);
  const forms = ['password', 'code'].filter(has);
  const tab = (name) => {
    $$('[data-tab]').forEach((x) => x.setAttribute('aria-selected', String(x.dataset.tab === name)));
    $$('[data-form="password"], [data-form="code"]').forEach((f) => (f.hidden = f.dataset.form !== name));
    say('');
  };
  $$('[data-tab]').forEach((x) => {
    x.hidden = !forms.includes(x.dataset.tab);
    x.addEventListener('click', () => tab(x.dataset.tab));
  });
  $('.au-tabs').hidden = forms.length < 2;
  if (forms.length) {
    tab(forms[0]);
  }
  $('[data-do="passkey"]').hidden = !has('passkey');
  if (has('social') && cfg.providers.length) {
    const box = $('.au-providers');
    cfg.providers.forEach((p) => {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.textContent = p.name;
      if (p.color) {
        btn.style.setProperty('--brand', p.color);
      }
      btn.addEventListener('click', () => run(() => L.signInWithProvider({ provider: p.code })));
      box.appendChild(btn);
    });
    box.hidden = false;
  }
  // Every call resolves to { status }; only contract violations reject.
  const sendBtn = $('[data-do="send"]');
  let timer = 0;
  const cooldown = (s) => {
    clearInterval(timer);
    let left = s;
    const tick = () => {
      sendBtn.disabled = left > 0;
      sendBtn.textContent = left > 0 ? t('code.resend', { s: left }) : t('code.send');
      if (left-- <= 0) {
        clearInterval(timer);
      }
    };
    tick();
    timer = setInterval(tick, 1000);
  };
  const handle = (r) => {
    say(r.status === 'error' ? r.message : '');
    if (r.status === 'two_factor') {
      const check = $('.au-check');
      check.hidden = !r.trustedDeviceDays;
      check.querySelector('span').textContent = t('challenge.remember', { days: r.trustedDeviceDays || 0 });
      $('[data-do="tf-passkey"]').hidden = !(r.methods || []).includes('passkey');
      $('[data-form="twofactor"]').reset();
      step('twofactor');
    } else if (r.status === 'sent') {
      cooldown(r.cooldown || 60);
    } else if (r.status === 'signin') {
      step('signin');
    } else if (r.status === 'done') {
      step('wait');
    }
  };
  const run = async (fn) => {
    if (busy) {
      return;
    }
    busy = true;
    try {
      handle(await fn());
    } catch (e) {
      say(e && e.message);
    } finally {
      busy = false;
    }
  };
  const need = (input, key) => {
    if (input.value.trim()) {
      return true;
    }
    say(t('signin.required', { name: t(key) }));
    input.focus();
    return false;
  };

  $('[data-form="password"]').addEventListener('submit', (e) => {
    e.preventDefault();
    const f = e.currentTarget;
    if (need(field(f, 'username'), 'signin.username') && need(field(f, 'password'), 'signin.passwordPh')) {
      run(() => L.signInWithPassword({ username: field(f, 'username').value.trim(), password: field(f, 'password').value, remember: true }));
    }
  });
  $('[data-form="code"]').addEventListener('submit', (e) => {
    e.preventDefault();
    const f = e.currentTarget;
    if (need(field(f, 'account'), 'signin.target') && need(field(f, 'code'), 'signin.codePh')) {
      run(() => L.signInWithCode({ target: field(f, 'account').value.trim(), code: field(f, 'code').value.trim(), remember: true }));
    }
  });
  $('[data-form="twofactor"]').addEventListener('submit', (e) => {
    e.preventDefault();
    const f = e.currentTarget;
    if (need(field(f, 'code'), 'challenge.code')) {
      run(() => L.verifyTwoFactor({ code: field(f, 'code').value.trim(), rememberDevice: field(f, 'rememberDevice').checked }));
    }
  });
  const remember = () => field($('[data-form="twofactor"]'), 'rememberDevice').checked;
  const actions = {
    send: () => {
      const f = $('[data-form="code"]');
      if (need(field(f, 'account'), 'signin.target')) {
        run(() => L.sendCode({ target: field(f, 'account').value.trim() }));
      }
    },
    continue: () => run(() => L.continueAsViewer()),
    switch: () => run(() => L.switchAccount()),
    passkey: () => run(() => L.signInWithPasskey({ remember: true })),
    'tf-passkey': () => run(() => L.twoFactorWithPasskey({ rememberDevice: remember() })),
    back: () => {
      say('');
      step('signin');
    },
    cancel: () => run(() => L.cancel()),
    forget: () => L.openLink({ name: 'forget' }).catch((e) => say(e.message)),
    register: () => L.openLink({ name: 'register' }).catch((e) => say(e.message))
  };
  $$('[data-do]').forEach((el) => {
    if (Object.prototype.hasOwnProperty.call(actions, el.dataset.do)) {
      el.addEventListener('click', actions[el.dataset.do]);
    }
  });

  const showViewer = (v) => {
    if (!v) {
      step('signin');
      return;
    }
    $('.au-who').textContent = v.nickname + ' · UID ' + v.uid;
    $('[data-do="continue"]').textContent = t('cont.as', { name: v.nickname });
    step('continue');
  };
  // The user centre returns to sign-in (session expired, account switched, sign-in refused later).
  L.on('reset', (d) => {
    say((d && d.message) || '');
    showViewer(d && d.viewer);
  });
  showViewer(cfg.viewer);
})();
