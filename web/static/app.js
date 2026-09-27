document.addEventListener('DOMContentLoaded', function () {
  // Redirects can carry a one-off notification via ?toast=...&toast_type=...
  // Show it with toastr, then strip the params so a page refresh doesn't repeat it.
  var params = new URLSearchParams(window.location.search);
  var msg = params.get('toast');
  if (msg && window.toastr) {
    var type = params.get('toast_type') || 'success';
    toastr.options = {
      closeButton: true,
      progressBar: true,
      positionClass: 'toast-bottom-right',
      timeOut: 3500,
      newestOnTop: true,
    };
    (toastr[type] || toastr.info)(msg);

    params.delete('toast');
    params.delete('toast_type');
    var query = params.toString();
    var newUrl = window.location.pathname + (query ? '?' + query : '');
    window.history.replaceState({}, '', newUrl);
  }

  // Any form with data-confirm="..." asks via SweetAlert2 before submitting,
  // instead of the browser's native confirm() dialog.
  document.querySelectorAll('form[data-confirm]').forEach(function (form) {
    form.addEventListener('submit', function (e) {
      if (form.dataset.confirmed || !window.Swal) return;
      e.preventDefault();
      Swal.fire({
        title: 'Emin misiniz?',
        text: form.getAttribute('data-confirm'),
        icon: 'warning',
        showCancelButton: true,
        confirmButtonText: 'Evet',
        cancelButtonText: 'Vazgeç',
        confirmButtonColor: '#dc2626',
        cancelButtonColor: '#6b7280',
      }).then(function (result) {
        if (result.isConfirmed) {
          form.dataset.confirmed = '1';
          form.submit();
        }
      });
    });
  });
});
