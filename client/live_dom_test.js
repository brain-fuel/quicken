// Exercises the quicken live client functions (patch morph, first-render
// stitching, event delegation) against a minimal hand-rolled DOM under node,
// so the JavaScript behavior is asserted, not just that the file is served.
// Driven by the Go test TestLiveShimBehaviorViaNode, which skips when node
// is absent. node is a test-time tool here, not a module dependency.
'use strict';
const fs = require('fs');
const path = require('path');

let all = [];

function descendants(node) {
  var out = [];
  for (var i = 0; i < node.children.length; i++) {
    out.push(node.children[i]);
    out = out.concat(descendants(node.children[i]));
  }
  return out;
}

// Matches the small subset of CSS selectors the live functions use:
// tag[attr="value"], [attr="value"], [attr], or a bare tag name.
function selMatches(el, sel) {
  var m = sel.match(/^([\w-]+)?\[([\w-]+)="(.*)"\]$/);
  if (m) {
    if (m[1] && el.tag !== m[1]) return false;
    return el.getAttribute(m[2]) === m[3];
  }
  m = sel.match(/^([\w-]+)?\[([\w-]+)\]$/);
  if (m) {
    if (m[1] && el.tag !== m[1]) return false;
    return el.hasAttribute(m[2]);
  }
  return el.tag === sel;
}

function matchAll(sel) {
  return all.filter(function (e) { return selMatches(e, sel); });
}

class El {
  constructor(tag) {
    this.tag = tag;
    this.children = [];
    this.attrs = {};
    this.parentNode = null;
    this._innerHTML = '';
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
  querySelector(sel) {
    const d = descendants(this).filter(function (e) { return selMatches(e, sel); });
    return d.length ? d[0] : null;
  }
  querySelectorAll(sel) {
    return descendants(this).filter(function (e) { return selMatches(e, sel); });
  }
  get innerHTML() {
    return this._innerHTML;
  }
  set innerHTML(html) {
    // The live functions only ever read innerHTML back as a string (to
    // assert stitched markup) or write plain text into a single <q-d>
    // marker; no reparse into child elements is needed for the selectors
    // the shim uses.
    this._innerHTML = html;
    this.children = [];
  }
}

const liveListeners = {};
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
  addEventListener(type, fn) {
    if (!liveListeners[type]) liveListeners[type] = [];
    liveListeners[type].push(fn);
  },
};
const window = {};

function dispatch(type, target) {
  const fns = liveListeners[type] || [];
  for (let i = 0; i < fns.length; i++) fns[i]({ target: target });
}

// Load the shim into this lexical scope; its bare document/window references
// resolve to the consts above.
const src = fs.readFileSync(path.join(__dirname, 'quicken.js'), 'utf8');
eval(src);

function assert(cond, msg) {
  if (!cond) throw new Error('FAIL: ' + msg);
}

try {
  // Scenario 1: applyPatch replaces only the addressed <q-d> marker's
  // innerHTML, leaving its sibling marker untouched, AND leaves an
  // identically-addressed marker in a different region's slot untouched.
  // The second slot's data-qi="0" marker is built first (and its id sorts
  // after 'c' alphabetically) so a regression that widens the lookup from
  // the region's own slot to a document-wide `document.querySelector`
  // would grab this other slot's marker first and fail the assertions
  // below, rather than passing by accidental creation-order luck.
  (function () {
    all = [];
    const slotD = new El('div');
    slotD.setAttribute('id', 'q-slot-d');
    const dd0 = new El('q-d');
    dd0.setAttribute('data-qi', '0');
    dd0.innerHTML = 'Z';
    slotD.appendChild(dd0);

    const slot = new El('div');
    slot.setAttribute('id', 'q-slot-c');
    const d0 = new El('q-d');
    d0.setAttribute('data-qi', '0');
    d0.innerHTML = 'A';
    const d1 = new El('q-d');
    d1.setAttribute('data-qi', '1');
    d1.innerHTML = 'B';
    slot.appendChild(d0);
    slot.appendChild(d1);

    window.__quicken.applyPatch('c', { '0': 'NEW' });

    assert(d0.innerHTML === 'NEW', 'patched marker should read NEW, got ' + d0.innerHTML);
    assert(d1.innerHTML === 'B', 'untouched marker should still read B, got ' + d1.innerHTML);
    assert(
      dd0.innerHTML === 'Z',
      'colliding data-qi="0" marker in region d should be untouched, got ' + dd0.innerHTML
    );
  })();

  // Scenario 2: applyFirst stitches statics/dynamics into slot-addressed
  // markup and clears data-q-pending.
  (function () {
    all = [];
    const slot = new El('div');
    slot.setAttribute('id', 'q-slot-c');
    slot.setAttribute('data-q-pending', '');

    window.__quicken.applyFirst('c', ['<b>', '</b>'], ['X']);

    assert(
      slot.innerHTML === '<b><q-d data-qi="0">X</q-d></b>',
      'stitched markup mismatch, got ' + slot.innerHTML
    );
    assert(slot.getAttribute('data-q-pending') === null, 'data-q-pending should be removed');
  })();

  // Scenario 3: wireLive delegates a click on a data-live-click element to
  // the provided send function, resolving the enclosing q-slot region and
  // leaving payload undefined when no data-live-payload is present.
  (function () {
    all = [];
    for (const k in liveListeners) delete liveListeners[k];
    const slot = new El('div');
    slot.setAttribute('id', 'q-slot-c');
    const btn = new El('button');
    btn.setAttribute('data-live-click', 'inc');
    slot.appendChild(btn);

    let sent = null;
    window.__quicken.wireLive(function (m) { sent = m; });
    dispatch('click', btn);

    assert(sent !== null, 'send should have been called');
    assert(sent.type === 'event', 'type should be event, got ' + sent.type);
    assert(sent.region === 'c', 'region should be c, got ' + sent.region);
    assert(sent.event === 'inc', 'event should be inc, got ' + sent.event);
    assert(sent.payload === undefined, 'payload should be undefined, got ' + sent.payload);
  })();

  console.log('quicken.js live shim: all scenarios passed');
} catch (e) {
  console.log(e.message);
  process.exit(1);
}
