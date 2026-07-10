// Package quicken renders web pages as a fast shell plus deferred regions.
//
// A page paints its shell and lightweight skeletons immediately, then fills
// each region with its real content as that content becomes ready. The
// default transport streams over one HTTP response and stays readable with
// JavaScript disabled. A ClientFetch transport fetches each region after
// load, and a LiveChannel transport keeps a region live over a WebSocket
// (falling back to HTTP long-poll), pushing fine-grained patches as its
// server-held state changes. Later phases add more authoring adapters behind
// the same Region and Transport seams.
package quicken
