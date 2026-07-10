// Exercises the quicken fetch runtime against a minimal hand-rolled DOM and a
// mocked fetch under node. Driven by client_fetch_node_test.go, which skips
// when node is absent. node is a test-time tool, not a module dependency.
'use strict';
const fs = require('fs');
const path = require('path');

let all = [];
class El {
  constructor(tag) { this.tag = tag; this.children = []; this.attrs = {}; this.parentNode = null; this._html = null; all.push(this); }
  get firstChild() { return this.children.length ? this.children[0] : null; }
  appendChild(c) { if (c.parentNode) c.parentNode.removeChild(c); c.parentNode = this; this.children.push(c); return c; }
  removeChild(c) { const i = this.children.indexOf(c); if (i >= 0) { this.children.splice(i, 1); c.parentNode = null; } return c; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  getAttribute(k) { return Object.prototype.hasOwnProperty.call(this.attrs, k) ? this.attrs[k] : null; }
  removeAttribute(k) { delete this.attrs[k]; }
  hasAttribute(k) { return Object.prototype.hasOwnProperty.call(this.attrs, k); }
  set innerHTML(v) { this._html = v; this.children = []; }
  get innerHTML() { return this._html; }
  get textContent() { return this._html; }
  set textContent(v) { this._html = v; }
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
  getElementById(id) { return all.find((e) => e.getAttribute('id') === id) || null; },
  addEventListener() {},
};
const window = {};

let fetchCalls = [];
let fetchImpl = function (url) {
  fetchCalls.push(url);
  return Promise.resolve({ text: function () { return Promise.resolve('<p class="real">FROM ' + url + '</p>'); } });
};
const fetch = function (url) { return fetchImpl(url); };

global.document = document;
global.window = window;

const src = fs.readFileSync(path.join(__dirname, 'quicken.js'), 'utf8');
eval(src);

function assert(cond, msg) { if (!cond) throw new Error('FAIL: ' + msg); }

(async function () {
  // fillSlotHTML sets content and clears the pending marker.
  (function () {
    all = [];
    const slot = new El('div'); slot.setAttribute('id', 'q-slot-a'); slot.setAttribute('data-q-pending', '');
    window.__quicken.fillSlotHTML('a', '<b>hi</b>');
    assert(slot.innerHTML === '<b>hi</b>', 'fillSlotHTML sets innerHTML');
    assert(slot.getAttribute('data-q-pending') === null, 'fillSlotHTML clears pending');
  })();

  // load memoizes by url: two loads of the same url call fetch once.
  (function () {
    all = []; fetchCalls = [];
    window.__quicken.load('/u');
    window.__quicken.load('/u');
    assert(fetchCalls.length === 1, 'load fetched once for repeated url, got ' + fetchCalls.length);
  })();

  // regionURL builds named and unnamed urls.
  assert(window.__quicken.regionURL('/_regions', 'demo', 'x') === '/_regions/demo/x', 'named regionURL');
  assert(window.__quicken.regionURL('/_regions', '', 'x') === '/_regions/x', 'unnamed regionURL');

  // fetchRegion fills the slot from the fetched html.
  await (async function () {
    all = []; fetchCalls = [];
    const slot = new El('div'); slot.setAttribute('id', 'q-slot-cards'); slot.setAttribute('data-q-pending', '');
    await window.__quicken.fetchRegion('cards', '/_regions/cards');
    assert(slot.innerHTML.indexOf('FROM /_regions/cards') >= 0, 'fetchRegion filled slot: ' + slot.innerHTML);
    assert(slot.getAttribute('data-q-pending') === null, 'fetchRegion cleared pending');
  })();

  // fetchRegion renders an error card when the fetch rejects.
  await (async function () {
    all = []; fetchCalls = [];
    const slot = new El('div'); slot.setAttribute('id', 'q-slot-bad'); slot.setAttribute('data-q-pending', '');
    fetchImpl = function () { return Promise.reject(new Error('net')); };
    await window.__quicken.fetchRegion('bad', '/_regions/bad');
    assert(slot.innerHTML.indexOf('data-q-error') >= 0, 'fetchRegion error card: ' + slot.innerHTML);
    fetchImpl = function (url) { fetchCalls.push(url); return Promise.resolve({ text: function () { return Promise.resolve('ok'); } }); };
  })();

  // init reads the manifest and fetches every listed region.
  await (async function () {
    all = []; fetchCalls = [];
    const manifest = new El('script'); manifest.setAttribute('data-q-manifest', '');
    manifest.textContent = JSON.stringify({ base: '/_regions', page: 'demo', ids: ['one', 'two'] });
    const s1 = new El('div'); s1.setAttribute('id', 'q-slot-one'); s1.setAttribute('data-q-pending', '');
    const s2 = new El('div'); s2.setAttribute('id', 'q-slot-two'); s2.setAttribute('data-q-pending', '');
    window.__quicken.init();
    await Promise.resolve(); await Promise.resolve();
    assert(fetchCalls.indexOf('/_regions/demo/one') >= 0, 'init fetched one');
    assert(fetchCalls.indexOf('/_regions/demo/two') >= 0, 'init fetched two');
  })();

  console.log('quicken.js fetch runtime: all scenarios passed');
})().catch(function (e) { console.error(e); process.exit(1); });
