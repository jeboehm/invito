(function () {
  var script = document.currentScript;
  var user = script.dataset.user;
  var slug = script.dataset.slug;
  var baseUrl = new URL(script.src).origin;

  var iframe = document.createElement('iframe');
  iframe.src = baseUrl + '/widget/' + user + '/' + slug;
  iframe.style.cssText = 'width:100%;border:none;display:block;overflow:hidden;';
  iframe.setAttribute('scrolling', 'no');
  iframe.setAttribute('title', 'Booking');

  script.parentNode.insertBefore(iframe, script);

  window.addEventListener('message', function (e) {
    if (e.origin !== baseUrl) return;
    if (e.source !== iframe.contentWindow) return;
    if (e.data && e.data.type === 'invito-resize') {
      iframe.style.height = e.data.height + 'px';
    }
  });
})();
