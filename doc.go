// Package quicken renders web pages as a fast shell plus deferred regions.
//
// A page paints its shell and lightweight skeletons immediately, then fills
// each region with its real content as that content becomes ready. The
// default transport streams over one HTTP response and stays readable with
// JavaScript disabled. Later phases add client-fetch and live transports
// behind the same Region and Transport seams.
package quicken
