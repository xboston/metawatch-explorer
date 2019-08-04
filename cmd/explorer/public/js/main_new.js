$(document).ready(function () {

    const defaultHash = document.location.hash.replace('#', '').trim();

    if (defaultHash != "") {
        $('#search_q').val(defaultHash)
    }

    let statTable = $('#proxytable').DataTable({
        "ajax": "/api/v1/nodes.json",
        "oSearch": {
            "sSearch": defaultHash
        },
        sDom: 't',
        // searching: false,
        lengthChange: false,
        paging: false,
        searching: true,
        ordering: true,
        info: true,
        autoWidth: false,
        responsive: true,
        oLanguage: {
            sLoadingRecords: "<img style='max-height:432px' src='/img/cosmo.gif'>"
        },
        processing: true,
        "columns": [{
                "data": "name",
                render: function (data, type, row) {
                    if (type === "sort" || type === "type") {
                        return data;
                    }
                    const stripName = row.address.substr(0, 12) + "…";
                    const nodeName = row.name.substr(0, 47);
                    return '<a class="is-family-monospace ellipsis" href="/address/' + row.address + '/info">' + stripName + '</a> / ' + nodeName.linkify();
                }
            },
            {
                "data": "address",
                "visible": false,
            },
            {
                "data": "country_long"
            },
            {
                "data": "delegated_amount",
                render: function (data, type, row) {
                    let className = 'is-success';
                    if (10000000000000 - data <= 1000000000000) {
                        className = 'is-warning';
                    }
                    if (10000000000000 - data <= 0) {
                        className = 'is-danger';
                    }
                    return '<span class="tag ' + className + '">' + (data / 1e6).toLocaleString('en') + '</span>';
                }
            },

        ],
    });

    statTable.on('search.dt', function () {
        statTable.search() != "" && setTimeout(() => {
            document.location = "#" + statTable.search();
            document.title = 'MetaWat.ch - ' + statTable.search();
        }, 250)
    });

    $('#search_q').on('keyup', function () {
        statTable.search(this.value).draw();
    });

    String.prototype.linkify = function () {
        return this.replace(/(^|\s)@(\w{3,25})/g, "$1<a target=\"_blank\" href=\"https://t.me/$2\">@$2</a>");
    };
});