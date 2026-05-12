(function () {
  var locale = navigator.language;
  var _dateNav = null;

  function formatDateBtns() {
    var fmtDay  = new Intl.DateTimeFormat(locale, { weekday: 'short', timeZone: 'UTC' });
    var fmtDate = new Intl.DateTimeFormat(locale, { month: 'short', day: 'numeric', timeZone: 'UTC' });
    document.querySelectorAll('.date-btn[data-date]').forEach(function (btn) {
      var d = new Date(btn.dataset.date + 'T12:00:00Z');
      var dayEl  = btn.querySelector('.date-btn-day');
      var dateEl = btn.querySelector('.date-btn-date');
      if (dayEl)  dayEl.textContent  = fmtDay.format(d);
      if (dateEl) dateEl.textContent = fmtDate.format(d);
    });
  }

  function formatSlotBtns(root) {
    if (!_dateNav) _dateNav = document.querySelector('.date-nav');
    var tz  = (_dateNav && _dateNav.dataset.timezone) || 'UTC';
    var fmt = new Intl.DateTimeFormat(locale, { hour: 'numeric', minute: '2-digit', timeZone: tz });
    (root || document).querySelectorAll('.slot-btn[data-time]').forEach(function (btn) {
      btn.textContent = fmt.format(new Date(btn.dataset.time));
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    document.documentElement.lang = locale;
    formatDateBtns();
    formatSlotBtns();
  });

  document.addEventListener('htmx:afterSwap', function (e) {
    formatSlotBtns(e.detail.elt);
  });
})();
