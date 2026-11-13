// Adds the version of the region to each htmx request as the param v.
document.addEventListener("htmx:configRequest", function (evt) {
  var elt = evt.detail && evt.detail.elt;
  if (elt && elt.dataset && elt.dataset.v) {
    evt.detail.parameters["v"] = elt.dataset.v;
  }
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

if (typeof Idiomorph !== "undefined") {
  Idiomorph.defaults.callbacks.beforeNodeMorphed = function (oldNode, newNode) {
    if (keepDiagram(oldNode, newNode)) return false;
  };
  // Keep the open state of a details that the reader toggled.
  Idiomorph.defaults.callbacks.beforeAttributeUpdated = function (name, node) {
    if (name === "open" && node.tagName === "DETAILS") return false;
  };
}
