// Exercises the quicken swap shim against a minimal hand-rolled DOM under
// node, so the JavaScript behavior (not just that the file is served) is
// asserted. Supports only the DOM calls the shim makes. Driven by the Go test
// TestShimSwapBehaviorViaNode, which skips when node is absent. node is a
// test-time tool here, not a module dependency.
'use strict';
const fs = require('fs');
const path = require('path');

let all = [];

class El {
  constructor(tag) {
    this.tag = tag;
    this.children = [];
    this.attrs = {};
    this.parentNode = null;
    all.push(this);
  }
  get firstChild() {
    return this.children.length ? this.children[0] : null;
  }
  appendChild(c) {
    if (c.parentNode) c.parentNode.removeChild(c);
    c.parentNode = this;
    this.children.push(c);
    return c;
  }
  removeChild(c) {
    const i = this.children.indexOf(c);
    if (i >= 0) {
      this.children.splice(i, 1);
      c.parentNode = null;
    }
    return c;
  }
  setAttribute(k, v) {
    this.attrs[k] = String(v);
  }
  getAttribute(k) {
    return Object.prototype.hasOwnProperty.call(this.attrs, k) ? this.attrs[k] : null;
  }
  removeAttribute(k) {
    delete this.attrs[k];
  }
  hasAttribute(k) {
    return Object.prototype.hasOwnProperty.call(this.attrs, k);
  }
  addEventListener(type, handler, opts) {
    this._listeners = this._listeners || {};
    (this._listeners[type] = this._listeners[type] || []).push({
      handler,
      once: !!(opts && opts.once),
    });
  }
  // Test-only: fires all handlers registered for `type`, honoring `once`.
  // The shim never calls this; it is scaffolding to drive onhover.
  dispatch(type) {
    const list = ((this._listeners || {})[type] || []).slice();
    for (const entry of list) {
      entry.handler.call(this);
      if (entry.once) {
        const idx = this._listeners[type].indexOf(entry);
        if (idx >= 0) this._listeners[type].splice(idx, 1);
      }
    }
  }
}

function matchAll(sel) {
  let m = sel.match(/^\[([\w-]+)="(.*)"\]$/);
  if (m) return all.filter((e) => e.getAttribute(m[1]) === m[2]);
  m = sel.match(/^\[([\w-]+)\]$/);
  if (m) return all.filter((e) => e.hasAttribute(m[1]));
  return [];
}

const document = {
  querySelector(sel) {
    const r = matchAll(sel);
    return r.length ? r[0] : null;
  },
  querySelectorAll(sel) {
    return matchAll(sel);
  },
  getElementById(id) {
    return all.find((e) => e.getAttribute('id') === id) || null;
  },
  addEventListener() {
    // The load-time handler is not fired in these unit checks; swap is called
    // directly.
  },
};
const window = {};

// Test-only IntersectionObserver stub. The shim references the bare
// identifier `IntersectionObserver` (as browsers do via the global object),
// so it must be declared as a variable in this module's scope, visible to
// the eval'd shim code below via the normal scope chain. `ioRegistry` records
// every observed element so a scenario can simulate an intersection with
// `triggerIntersecting(id)`.
let ioRegistry = [];
class IntersectionObserver {
  constructor(cb) {
    this.cb = cb;
    this.disconnected = false;
  }
  observe(el) {
    ioRegistry.push({ el, io: this });
  }
  disconnect() {
    this.disconnected = true;
  }
}
function triggerIntersecting(id) {
  for (const entry of ioRegistry) {
    if (!entry.io.disconnected && entry.el.getAttribute('id') === id) {
      entry.io.cb([{ target: entry.el, isIntersecting: true }]);
    }
  }
}

// Load the shim into this lexical scope; its bare document/window references
// resolve to the consts above.
const src = fs.readFileSync(path.join(__dirname, 'quicken.js'), 'utf8');
eval(src);

function assert(cond, msg) {
  if (!cond) throw new Error('FAIL: ' + msg);
}

// Builds a pending slot (with a placeholder skeleton child) at
// id="q-slot-<id>", the shape swap()/reveal() operate on.
function makeSlot(id) {
  const slot = new El('div');
  slot.setAttribute('id', 'q-slot-' + id);
  slot.setAttribute('data-q-slot', '');
  slot.setAttribute('data-q-pending', '');
  slot.appendChild(new El('i'));
  return slot;
}

// Builds a streamed fill block tagged data-q-fill/data-q-strategy/
// data-q-trigger, attached to `body`, wrapping one real child node.
function makeFill(body, id, strategy, trigger) {
  const fill = new El('div');
  fill.setAttribute('data-q-fill', id);
  fill.setAttribute('data-q-strategy', strategy);
  fill.setAttribute('data-q-trigger', trigger);
  body.appendChild(fill);
  fill.appendChild(new El('p'));
  return fill;
}

function slotPending(id) {
  const slot = document.getElementById('q-slot-' + id);
  return slot !== null && slot.getAttribute('data-q-pending') === '';
}

// Scenario 1: happy path. swap moves the fill's children into the slot,
// clears the pending marker, and detaches the fill wrapper.
(function () {
  all = [];
  const slot = new El('div');
  slot.setAttribute('id', 'q-slot-alpha');
  slot.setAttribute('data-q-slot', '');
  slot.setAttribute('data-q-pending', '');
  const skeleton = new El('i');
  slot.appendChild(skeleton);
  const body = new El('body');
  const fill = new El('div');
  fill.setAttribute('data-q-fill', 'alpha');
  body.appendChild(fill);
  const real = new El('p');
  fill.appendChild(real);

  window.__quicken.swap('alpha');

  assert(slot.children.length === 1 && slot.children[0] === real, 'slot should hold the real node');
  assert(slot.getAttribute('data-q-pending') === null, 'data-q-pending should be removed');
  assert(fill.parentNode === null, 'fill wrapper should be detached');
})();

// Scenario 2: missing slot is a no-op, no throw, fill stays put.
(function () {
  all = [];
  const body = new El('body');
  const fill = new El('div');
  fill.setAttribute('data-q-fill', 'ghost');
  body.appendChild(fill);
  const real = new El('p');
  fill.appendChild(real);

  window.__quicken.swap('ghost');

  assert(fill.parentNode === body, 'fill stays put when the slot is missing');
})();

// Scenario 3: missing fill is a no-op, slot keeps its skeleton and stays pending.
(function () {
  all = [];
  const slot = new El('div');
  slot.setAttribute('id', 'q-slot-solo');
  slot.setAttribute('data-q-pending', '');
  const sk = new El('i');
  slot.appendChild(sk);

  window.__quicken.swap('solo');

  assert(slot.getAttribute('data-q-pending') === '', 'slot stays pending when the fill is missing');
  assert(slot.children.length === 1 && slot.children[0] === sk, 'slot keeps its skeleton when the fill is missing');
})();

// Scenario 4: reveal() with strategy=eager/trigger=onload swaps immediately.
(function () {
  all = [];
  const body = new El('body');
  makeSlot('a');
  makeFill(body, 'a', 'eager', 'onload');

  window.__quicken.reveal('a');

  assert(!slotPending('a'), 'eager/onload should swap immediately');
})();

// Scenario 5: reveal() with trigger=onvisible defers the swap behind an
// IntersectionObserver on the slot, and fires it only once the slot
// intersects.
(function () {
  all = [];
  ioRegistry = [];
  const body = new El('body');
  makeSlot('b');
  makeFill(body, 'b', 'deferred', 'onvisible');

  window.__quicken.reveal('b');
  assert(slotPending('b'), 'onvisible should defer the swap until intersecting');

  triggerIntersecting('q-slot-b');
  assert(!slotPending('b'), 'onvisible should swap once the slot intersects');
})();

// Scenario 6: reveal() with trigger=onhover defers the swap behind a
// mouseover/focusin listener on the slot.
(function () {
  all = [];
  const body = new El('body');
  const slot = makeSlot('c');
  makeFill(body, 'c', 'deferred', 'onhover');

  window.__quicken.reveal('c');
  assert(slotPending('c'), 'onhover should defer the swap until hovered');

  slot.dispatch('mouseover');
  assert(!slotPending('c'), 'onhover should swap on mouseover');
})();

// Scenario 7: reveal() with strategy=live is a no-op; the live client owns
// that fill and reveal must not touch it.
(function () {
  all = [];
  const body = new El('body');
  makeSlot('d');
  makeFill(body, 'd', 'live', 'live');

  window.__quicken.reveal('d');

  assert(slotPending('d'), 'live strategy must stay untouched by reveal');
})();

// Scenario 8: reveal() is idempotent per id. Calling it twice for an eager
// fill, or hovering twice after the fill is already gone, must not throw or
// double-swap.
(function () {
  all = [];
  const body = new El('body');
  const slot = makeSlot('e');
  makeFill(body, 'e', 'deferred', 'onhover');

  window.__quicken.reveal('e');
  slot.dispatch('mouseover');
  assert(!slotPending('e'), 'first hover should swap');
  slot.dispatch('mouseover'); // once:true means this handler is already gone
  window.__quicken.reveal('e'); // fill is detached now; must be a safe no-op
  assert(!slotPending('e'), 'repeat reveal/hover after swap must stay a no-op, not throw');
})();

console.log('quicken.js swap shim: all scenarios passed');
