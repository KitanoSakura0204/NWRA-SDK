// Lounge plugin page bridge (SDK/plugins/ABI.md section 4). Copy this file into your plugin.
// The page runs in a sandboxed iframe with an opaque origin: no network, no cookies, no
// storage. Everything goes through the host page with postMessage.
(function () {
  var seq = 0;
  var pending = {};
  var listeners = {};

  window.addEventListener('message', function (ev) {
    if (ev.source !== window.parent) return;
    var m = ev.data;
    if (!m || m.lounge !== 1) return;
    if (m.event) {
      (listeners[m.event] || []).forEach(function (fn) {
        fn(m.data);
      });
      return;
    }
    var p = pending[m.id];
    if (!p) return;
    delete pending[m.id];
    if (m.ok) p.resolve(m.result);
    else p.reject(new Error(String(m.error)));
  });

  function call(method, params) {
    var id = ++seq;
    return new Promise(function (resolve, reject) {
      pending[id] = { resolve: resolve, reject: reject };
      window.parent.postMessage({ lounge: 1, id: id, method: method, params: params || {} }, '*');
    });
  }

  window.lounge = {
    /** Announces the page; resolves with the context (locale, theme, slot, plugin, context). */
    ready: function () {
      return call('ready');
    },
    context: function () {
      return call('context');
    },
    /** Asks the host to size the iframe (60-2000 px). */
    resize: function (height) {
      return call('resize', { height: height });
    },
    /** Runs the slot's handler in the plugin with {action, payload}; resolves with its JSON answer. */
    invoke: function (action, payload) {
      return call('invoke', { action: action, payload: payload });
    },
    /** In-site navigation (only /home/... from user-centre slots, /admin/... from admin slots). */
    navigate: function (path) {
      return call('navigate', { path: path });
    },
    toast: function (message, type) {
      return call('toast', { message: message, type: type || 'info' });
    },
    /** Opens an https URL in a new tab after the user confirms. */
    open: function (url) {
      return call('open', { url: url });
    },
    /** Host events: 'context' (theme or language changed). */
    on: function (event, fn) {
      (listeners[event] = listeners[event] || []).push(fn);
    }
  };
})();
