// quicken client shim.
//
// swap: moves a streamed fill block (from the StreamHTML transport) into its
// slot. With scripting off those fill blocks stay visible at the end of the
// document, so the page is still readable; swap only relocates them.
//
// fetch runtime: for the ClientFetch transport, load(url) fetches a region's
// HTML once and memoizes it by url; fetchRegion fills a slot from it; init
// reads the page manifest and fetches every region. The url-keyed cache is
// shared with prefetch (see wirePrefetch), so a prefetched url makes the later
// fetch instant.
(function () {
  var cache = {};

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

  function load(url) {
    if (!cache[url]) {
      cache[url] = fetch(url).then(function (r) { return r.text(); });
    }
    return cache[url];
  }

  function regionURL(base, page, id) {
    return page ? base + '/' + page + '/' + id : base + '/' + id;
  }

  function fetchRegion(id, url) {
    return load(url).then(function (html) {
      fillSlotHTML(id, html);
    }).catch(function () {
      fillSlotHTML(id, '<div data-q-error>region failed to load</div>');
    });
  }

  function prefetch(url) {
    load(url);
  }

  function wirePrefetch() {
    var nodes = document.querySelectorAll('[data-q-prefetch]');
    for (var i = 0; i < nodes.length; i++) {
      (function (el) {
        var url = el.getAttribute('data-q-prefetch');
        var on = el.getAttribute('data-q-prefetch-on') || 'mouseover';
        if (on === 'visible') {
          if (typeof IntersectionObserver !== 'undefined') {
            var io = new IntersectionObserver(function (entries) {
              for (var j = 0; j < entries.length; j++) {
                if (entries[j].isIntersecting) { prefetch(url); io.disconnect(); }
              }
            });
            io.observe(el);
          } else {
            prefetch(url);
          }
        } else {
          el.addEventListener(on, function () { prefetch(url); }, { once: true });
        }
      })(nodes[i]);
    }
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

  function connectLive(manifest) {
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
    var m = document.querySelector('[data-q-manifest]');
    if (m) {
      var man = JSON.parse(m.textContent);
      var base = man.base, page = man.page, ids = man.ids || [];
      for (var i = 0; i < ids.length; i++) {
        fetchRegion(ids[i], regionURL(base, page, ids[i]));
      }
    }
    var fills = document.querySelectorAll('[data-q-fill]');
    for (var j = 0; j < fills.length; j++) {
      swap(fills[j].getAttribute('data-q-fill'));
    }
    wirePrefetch();
    initLive();
  }

  window.__quicken = {
    swap: swap,
    fillSlotHTML: fillSlotHTML,
    load: load,
    regionURL: regionURL,
    fetchRegion: fetchRegion,
    prefetch: prefetch,
    wirePrefetch: wirePrefetch,
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
