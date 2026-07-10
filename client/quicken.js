// quicken swap shim. Moves each streamed fill block into its slot as the
// block arrives, so regions appear in place and out of order. With scripting
// off the fill blocks stay visible at the end of the document, so the page is
// still fully readable; this shim only relocates them.
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
  window.__quicken = { swap: swap };
  document.addEventListener('DOMContentLoaded', function () {
    var fills = document.querySelectorAll('[data-q-fill]');
    for (var i = 0; i < fills.length; i++) {
      swap(fills[i].getAttribute('data-q-fill'));
    }
  });
})();
