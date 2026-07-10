// Exercises prefetch-on-intent against a minimal hand-rolled DOM under node.
// Driven by client_fetch_node_test.go, which skips when node is absent.
'use strict';
const fs = require('fs');
const path = require('path');

let all = [];
class El {
  constructor(tag) { this.tag = tag; this.attrs = {}; this.listeners = {}; all.push(this); }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  getAttribute(k) { return Object.prototype.hasOwnProperty.call(this.attrs, k) ? this.attrs[k] : null; }
  hasAttribute(k) { return Object.prototype.hasOwnProperty.call(this.attrs, k); }
  addEventListener(ev, fn) { (this.listeners[ev] = this.listeners[ev] || []).push(fn); }
  fire(ev) { (this.listeners[ev] || []).forEach(function (f) { f(); }); }
}
function matchAll(sel) {
  let m = sel.match(/^\[([\w-]+)="(.*)"\]$/);
  if (m) return all.filter((e) => e.getAttribute(m[1]) === m[2]);
  m = sel.match(/^\[([\w-]+)\]$/);
  if (m) return all.filter((e) => e.hasAttribute(m[1]));
  return [];
}
const document = {
  querySelector(sel) { const r = matchAll(sel); return r.length ? r[0] : null; },
  querySelectorAll(sel) { return matchAll(sel); },
  getElementById() { return null; },
  addEventListener() {},
};
const window = {};

let fetchCalls = [];
const fetch = function (url) { fetchCalls.push(url); return Promise.resolve({ text: function () { return Promise.resolve('x'); } }); };

// A controllable IntersectionObserver stub.
let lastObserver = null;
function IntersectionObserver(cb) { this.cb = cb; this.observed = []; lastObserver = this; }
IntersectionObserver.prototype.observe = function (el) { this.observed.push(el); };
IntersectionObserver.prototype.disconnect = function () { this.observed = []; };

global.document = document;
global.window = window;
global.IntersectionObserver = IntersectionObserver;

const src = fs.readFileSync(path.join(__dirname, 'quicken.js'), 'utf8');
eval(src);

function assert(cond, msg) { if (!cond) throw new Error('FAIL: ' + msg); }

// Default trigger (mouseover) warms the cache once, and the cache is shared
// with load (a later load of the same url does not fetch again).
(function () {
  all = []; fetchCalls = [];
  const el = new El('a'); el.setAttribute('data-q-prefetch', '/t/1');
  window.__quicken.wirePrefetch();
  assert(fetchCalls.length === 0, 'no fetch before the trigger');
  el.fire('mouseover');
  assert(fetchCalls.length === 1 && fetchCalls[0] === '/t/1', 'mouseover prefetched once');
  window.__quicken.load('/t/1');
  assert(fetchCalls.length === 1, 'load reused the prefetched cache entry');
})();

// A configured trigger (focusin) is honored instead of mouseover.
(function () {
  all = []; fetchCalls = [];
  const el = new El('a'); el.setAttribute('data-q-prefetch', '/t/2'); el.setAttribute('data-q-prefetch-on', 'focusin');
  window.__quicken.wirePrefetch();
  el.fire('mouseover');
  assert(fetchCalls.length === 0, 'mouseover ignored when trigger is focusin');
  el.fire('focusin');
  assert(fetchCalls.length === 1 && fetchCalls[0] === '/t/2', 'focusin prefetched');
})();

// The visible trigger uses IntersectionObserver and fires when intersecting.
(function () {
  all = []; fetchCalls = [];
  const el = new El('a'); el.setAttribute('data-q-prefetch', '/t/3'); el.setAttribute('data-q-prefetch-on', 'visible');
  window.__quicken.wirePrefetch();
  assert(lastObserver && lastObserver.observed.indexOf(el) >= 0, 'visible trigger observed the element');
  lastObserver.cb([{ isIntersecting: false }]);
  assert(fetchCalls.length === 0, 'not fetched while off screen');
  lastObserver.cb([{ isIntersecting: true }]);
  assert(fetchCalls.length === 1 && fetchCalls[0] === '/t/3', 'fetched once on screen');
})();

console.log('quicken.js prefetch-on-intent: all scenarios passed');
