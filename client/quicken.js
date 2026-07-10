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
  }

  window.__quicken = {
    swap: swap,
    fillSlotHTML: fillSlotHTML,
    load: load,
    regionURL: regionURL,
    fetchRegion: fetchRegion,
    init: init
  };
  document.addEventListener('DOMContentLoaded', init);
})();
