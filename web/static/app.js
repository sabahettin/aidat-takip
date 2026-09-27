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

  // "Aidat Güncelle": ask from which month the new fee applies and the new
  // amount, then post them. The server splits the period at that month.
  document.querySelectorAll('form[data-fee-update]').forEach(function (form) {
    form.addEventListener('submit', function (e) {
      if (form.dataset.confirmed || !window.Swal) return;
      e.preventDefault();
      Swal.fire({
        title: 'Aidat Güncelle',
        html:
          '<p style="margin:0 0 12px;color:#6b7280;font-size:0.92rem">' +
          'Seçtiğiniz ay ve sonraki aylar yeni tutarla hesaplanır. ' +
          'Önceki aylar ve yapılmış ödemeler değişmez.</p>' +
          '<label class="swal-field">Hangi aydan itibaren geçerli?' +
          '<input id="swal-fee-month" type="month" class="swal2-input"></label>' +
          '<label class="swal-field">Yeni aylık aidat (₺)' +
          '<input id="swal-fee-amount" type="number" step="0.01" min="0" class="swal2-input"></label>',
        focusConfirm: false,
        showCancelButton: true,
        confirmButtonText: 'Güncelle',
        cancelButtonText: 'Vazgeç',
        confirmButtonColor: '#4f46e5',
        cancelButtonColor: '#6b7280',
        didOpen: function () {
          document.getElementById('swal-fee-month').value = form.dataset.currentMonth || '';
          document.getElementById('swal-fee-amount').value = form.dataset.currentFee || '';
        },
        preConfirm: function () {
          var month = document.getElementById('swal-fee-month').value;
          var amount = document.getElementById('swal-fee-amount').value;
          if (!month) {
            Swal.showValidationMessage('Geçerlilik ayını seçin');
            return false;
          }
          if (amount === '' || Number(amount) < 0) {
            Swal.showValidationMessage('Geçerli bir tutar girin');
            return false;
          }
          return { month: month, amount: amount };
        },
      }).then(function (result) {
        if (!result.isConfirmed) return;
        form.querySelector('input[name="effective_month"]').value = result.value.month;
        form.querySelector('input[name="monthly_fee"]').value = result.value.amount;
        form.dataset.confirmed = '1';
        form.submit();
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
        // Without this, DataTables writes each column's width to the
        // <colgroup> as a fractional pixel value (e.g. 146.719px). Chrome
        // then rounds each column boundary to the pixel grid independently,
        // which visibly staggers the row's bottom border by a pixel or two
        // right at that column edge — most noticeable on an empty last
        // column (e.g. a row with no action buttons yet). Turning autoWidth
        // off leaves sizing to the browser's own table layout, which keeps
        // every border in a row on one consistent line.
        autoWidth: false,
      });
    });
  }
});
