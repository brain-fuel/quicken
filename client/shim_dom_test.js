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

// Load the shim into this lexical scope; its bare document/window references
// resolve to the consts above.
const src = fs.readFileSync(path.join(__dirname, 'quicken.js'), 'utf8');
eval(src);

function assert(cond, msg) {
  if (!cond) throw new Error('FAIL: ' + msg);
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

console.log('quicken.js swap shim: all scenarios passed');
