// Resets the "add birthday" form after a successful submit. Done as a
// plain event listener rather than htmx's hx-on attribute, since hx-on
// compiles its attribute value with `new Function(...)`, which needs
// 'unsafe-eval' in script-src — this keeps the CSP strict.
document.body.addEventListener("htmx:afterRequest", function (evt) {
  var elt = evt.detail.elt;
  if (evt.detail.successful && elt && elt.id === "add-birthday-form") {
    elt.reset();
  }
});
