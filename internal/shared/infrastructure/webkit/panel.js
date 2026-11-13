// Adds the version of the region to each htmx request as the param v.
document.addEventListener("htmx:configRequest", function (evt) {
  var elt = evt.detail && evt.detail.elt;
  if (elt && elt.dataset && elt.dataset.v) {
    evt.detail.parameters["v"] = elt.dataset.v;
  }
});

// The event stream sets the live flag. A region polls only while it is clear,
// and the body carries data-live for the live dot.
document.addEventListener("htmx:sseOpen", function () {
  window.harnessLive = true;
  document.body.dataset.live = "on";
});
document.addEventListener("htmx:sseError", function () {
  window.harnessLive = false;
  document.body.dataset.live = "off";
});

// A run-only swap re-renders a region with empty diagram divs, and view.js
// renders mermaid on load only. Keep a rendered diagram whose source is
// unchanged. A plan change answers HX-Refresh, so the sources match here.
function keepDiagram(oldNode, newNode) {
  var sel = ".mm[data-diagram]";
  if (oldNode.nodeType !== 1 || newNode.nodeType !== 1) return false;
  if (!oldNode.matches(sel) || !newNode.matches(sel)) return false;
  return !!(
    oldNode.hasChildNodes() &&
    oldNode.getAttribute("data-diagram") === newNode.getAttribute("data-diagram")
  );
}

// A node that the page script owns, such as the theme toggle whose label it
// set, carries this attribute. The morph leaves the old node as it is.
var KEEP_ATTR = "data-morph-keep";

if (typeof Idiomorph !== "undefined") {
  Idiomorph.defaults.callbacks.beforeNodeMorphed = function (oldNode, newNode) {
    if (oldNode.nodeType === 1 && oldNode.hasAttribute(KEEP_ATTR)) return false;
    if (keepDiagram(oldNode, newNode)) return false;
  };
  // Keep the open state of a details that the reader toggled.
  Idiomorph.defaults.callbacks.beforeAttributeUpdated = function (name, node) {
    if (name === "open" && node.tagName === "DETAILS") return false;
  };
}
