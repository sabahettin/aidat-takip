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

  // Turn every data table into a searchable/sortable DataTable, Turkish UI.
  if (window.jQuery && jQuery.fn.DataTable) {
    var turkish = {
      emptyTable: 'Kayıt bulunamadı',
      info: 'Toplam _TOTAL_ kayıttan _START_-_END_ arası gösteriliyor',
      infoEmpty: 'Kayıt yok',
      infoFiltered: '(_MAX_ kayıt içinden filtrelendi)',
      lengthMenu: 'Sayfada _MENU_ kayıt göster',
      loadingRecords: 'Yükleniyor...',
      processing: 'İşleniyor...',
      search: 'Ara:',
      zeroRecords: 'Eşleşen kayıt bulunamadı',
      paginate: { first: 'İlk', last: 'Son', next: 'Sonraki', previous: 'Önceki' },
      aria: {
        sortAscending: ': artan sıralamak için tıklayın',
        sortDescending: ': azalan sıralamak için tıklayın',
      },
    };
    jQuery('table.datatable').each(function () {
      jQuery(this).DataTable({
        language: turkish,
        pageLength: 10,
        order: [],
      });
    });
  }
});
