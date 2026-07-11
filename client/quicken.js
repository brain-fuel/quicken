// quicken client shim.
//
// swap: moves a streamed fill block (from Serve's floor) into its slot. With
// scripting off those fill blocks stay visible at the end of the document, so
// the page is still readable; swap only relocates them.
//
// reveal: dispatches a fill's swap by its declared strategy/trigger. init
// reveals every fill on load, then hands live regions to the live client.
(function () {
  function swap(id) {
    var fill = document.querySelector('[data-q-fill="' + id + '"]');
    var slot = document.getElementById('q-slot-' + id);
    if (!fill || !slot) return;
    while (slot.firstChild) slot.removeChild(slot.firstChild);
    while (fill.firstChild) slot.appendChild(fill.firstChild);
    slot.removeAttribute('data-q-pending');
    if (fill.parentNode) fill.parentNode.removeChild(fill);
  }

  function fillSlotHTML(id, html) {
    var slot = document.getElementById('q-slot-' + id);
    if (!slot) return;
    slot.innerHTML = html;
    slot.removeAttribute('data-q-pending');
  }

  // reveal: dispatches a streamed fill's swap by its declared strategy/
  // trigger (data-q-strategy/data-q-trigger, set by Serve's tail stream).
  // eager and onload swap immediately; onvisible defers behind an
  // IntersectionObserver on the slot; onhover defers behind a mouseover/
  // focusin listener on the slot; live is a no-op (the live client owns that
  // fill via the WebSocket/poll transport, not the fill/swap path). Guarded
  // by `revealed` so a fill is swapped at most once no matter how many times
  // reveal or its deferred trigger fires.
  var revealed = {};

  function fillOf(id) {
    return document.querySelector('[data-q-fill="' + id + '"]');
  }

  function revealNow(id) {
    if (revealed[id]) return;
    revealed[id] = true;
    swap(id);
  }

  function reveal(id) {
    var fill = fillOf(id);
    if (!fill) return;
    var strat = fill.getAttribute('data-q-strategy');
    var trig = fill.getAttribute('data-q-trigger');
    if (strat === 'live') return;
    if (trig === 'onvisible') {
      var slot = document.getElementById('q-slot-' + id);
      if (slot && typeof IntersectionObserver !== 'undefined') {
        var io = new IntersectionObserver(function (entries) {
          for (var i = 0; i < entries.length; i++) {
            if (entries[i].isIntersecting) { revealNow(id); io.disconnect(); return; }
          }
        });
        io.observe(slot);
        return;
      }
      revealNow(id);
      return;
    }
    if (trig === 'onhover') {
      var slotH = document.getElementById('q-slot-' + id);
      if (slotH) {
        var fire = function () { revealNow(id); };
        slotH.addEventListener('mouseover', fire, { once: true });
        slotH.addEventListener('focusin', fire, { once: true });
        return;
      }
      revealNow(id);
      return;
    }
    revealNow(id);
  }

  // Live client: reads the page's live manifest (a <script type=
  // "application/json" data-q-live> element with {ws, token, ids}), opens a
  // WebSocket to the server's LiveChannel, and falls back to an HTTP
  // long-poll transport when WebSocket is unavailable or the socket drops.
  //
  // Slot-addressed render: a dynamic value at index i is wrapped as
  // <q-d data-qi="i">value</q-d>. stitchLive is the JS mirror of the
  // server's renderLiveHTML and must produce byte-identical markup. A
  // "patch" message replaces only the innerHTML of the addressed <q-d>,
  // leaving every sibling node (and any focused input outside that slot)
  // untouched; this fine-grained morph is the v1 guarantee. A dynamic that
  // itself contains a focused input is out of scope for v1.
  function slotDynamic(region, idx) {
    var slot = document.getElementById('q-slot-' + region);
    if (!slot) return null;
    return slot.querySelector('q-d[data-qi="' + idx + '"]');
  }

  function stitchLive(statics, dynamics) {
    var out = '';
    for (var i = 0; i < dynamics.length; i++) {
      out += statics[i] + '<q-d data-qi="' + i + '">' + dynamics[i] + '</q-d>';
    }
    out += statics[statics.length - 1];
    return out;
  }

  function applyFirst(region, statics, dynamics) {
    var slot = document.getElementById('q-slot-' + region);
    if (!slot) return;
    slot.innerHTML = stitchLive(statics, dynamics);
    slot.removeAttribute('data-q-pending');
  }

  function applyFull(region, statics, dynamics) {
    applyFirst(region, statics, dynamics);
  }

  function applyPatch(region, changed) {
    for (var k in changed) {
      if (!changed.hasOwnProperty(k)) continue;
      var el = slotDynamic(region, parseInt(k, 10));
      if (el) el.innerHTML = changed[k];
    }
  }

  function applyError(region, message) {
    var slot = document.getElementById('q-slot-' + region);
    if (!slot) return;
    slot.innerHTML = '<div data-q-error>' + message + '</div>';
    slot.removeAttribute('data-q-pending');
  }

  function dispatchServer(msg) {
    if (msg.type === 'first') applyFirst(msg.region, msg.statics, msg.dynamics);
    else if (msg.type === 'full') applyFull(msg.region, msg.statics, msg.dynamics);
    else if (msg.type === 'patch') applyPatch(msg.region, msg.changed);
    else if (msg.type === 'error') applyError(msg.region, msg.message);
  }

  var LIVE_EVENTS = ['click', 'input', 'change', 'submit'];

  function closestSlot(node) {
    while (node && node.getAttribute) {
      var id = node.getAttribute('id');
      if (id && id.indexOf('q-slot-') === 0) return id.slice('q-slot-'.length);
      node = node.parentNode;
    }
    return null;
  }

  function wireLive(send) {
    for (var i = 0; i < LIVE_EVENTS.length; i++) {
      (function (evName) {
        document.addEventListener(evName, function (e) {
          var node = e.target;
          while (node && node.getAttribute) {
            var name = node.getAttribute('data-live-' + evName);
            if (name) {
              var slot = closestSlot(node);
              if (slot) {
                var payload = node.getAttribute('data-live-payload');
                send({
                  type: 'event',
                  region: slot,
                  event: name,
                  payload: payload ? JSON.parse(payload) : undefined
                });
              }
              return;
            }
            node = node.parentNode;
          }
        });
      })(LIVE_EVENTS[i]);
    }
  }

  // swapLiveSnapshots moves each live region's server-rendered first snapshot
  // (streamed into the floor as a fill tagged data-q-strategy="live", which
  // reveal deliberately leaves in place) into its slot before the transport
  // opens, so the region shows real content immediately instead of flashing
  // its skeleton until the first socket/poll message lands.
  function swapLiveSnapshots(manifest) {
    var ids = manifest.ids || [];
    for (var i = 0; i < ids.length; i++) swap(ids[i]);
  }

  function connectLive(manifest) {
    swapLiveSnapshots(manifest);
    if (typeof WebSocket === 'undefined') { pollLive(manifest); return; }
    var proto = (location.protocol === 'https:') ? 'wss://' : 'ws://';
    var ws;
    try {
      ws = new WebSocket(proto + location.host + manifest.ws);
    } catch (e) { pollLive(manifest); return; }
    var send = function (m) { ws.send(JSON.stringify(m)); };
    ws.onopen = function () {
      send({ type: 'resume', token: manifest.token });
      wireLive(send);
    };
    ws.onmessage = function (e) { dispatchServer(JSON.parse(e.data)); };
    ws.onclose = function () { pollLive(manifest); };
    ws.onerror = function () { try { ws.close(); } catch (e2) {} };
  }

  function pollLive(manifest) {
    swapLiveSnapshots(manifest);
    // The ws path is a sibling of poll and event, not a parent of them: for
    // a named page "demo" the server mounts /_live/demo/ws, /_live/demo/poll,
    // and /_live/demo/event (unnamed: /_live/ws, /_live/poll, /_live/event).
    // So the shared base is the ws path with its trailing /ws stripped, and
    // poll/event are appended to that base directly.
    var base = manifest.ws.replace(/\/ws$/, '');
    var send = function (m) {
      m.token = manifest.token;
      fetch(base + '/event', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(m)
      });
    };
    wireLive(send);
    var delay = 500;
    function loop() {
      fetch(base + '/poll?token=' + encodeURIComponent(manifest.token))
        .then(function (r) {
          if (r.status === 204) return null;
          return r.json();
        })
        .then(function (msg) {
          if (msg) { dispatchServer(msg); delay = 500; }
          setTimeout(loop, 0);
        })
        .catch(function () {
          setTimeout(loop, delay);
          delay = Math.min(delay * 2, 10000);
        });
    }
    loop();
  }

  function initLive() {
    var m = document.querySelector('[data-q-live]');
    if (!m) return;
    connectLive(JSON.parse(m.textContent));
  }

  function init() {
    var fills = document.querySelectorAll('[data-q-fill]');
    for (var j = 0; j < fills.length; j++) {
      reveal(fills[j].getAttribute('data-q-fill'));
    }
    initLive();
  }

  window.__quicken = {
    swap: swap,
    fillSlotHTML: fillSlotHTML,
    reveal: reveal,
    init: init,
    applyFirst: applyFirst,
    applyFull: applyFull,
    applyPatch: applyPatch,
    applyError: applyError,
    stitchLive: stitchLive,
    wireLive: wireLive,
    connectLive: connectLive,
    pollLive: pollLive
  };
  document.addEventListener('DOMContentLoaded', init);
})();
