(function () {
  var texts = {
    en: { label: 'Sign-ins', refresh: 'Refresh', failed: 'Could not load' },
    zh_CN: { label: '登录次数', refresh: '刷新', failed: '加载失败' },
    zh_TW: { label: '登入次數', refresh: '重新整理', failed: '載入失敗' },
    ja: { label: 'ログイン回数', refresh: '更新', failed: '読み込めません' }
  };
  var tx = texts.en;

  function apply(ctx) {
    tx = texts[ctx.locale] || texts.en;
    document.documentElement.dataset.theme = ctx.theme;
    document.getElementById('label').textContent = tx.label;
    document.getElementById('refresh').textContent = tx.refresh;
  }

  function refresh() {
    lounge.invoke('stats').then(
      function (r) {
        document.getElementById('count').textContent = String(r.logins);
      },
      function () {
        document.getElementById('count').textContent = tx.failed;
      }
    );
  }

  lounge.on('context', apply);
  lounge.ready().then(function (ctx) {
    apply(ctx);
    refresh();
  });
  document.getElementById('refresh').addEventListener('click', refresh);
  new ResizeObserver(function () {
    lounge.resize(document.documentElement.scrollHeight);
  }).observe(document.body);
})();
