$(document).ready(function () {

    const defaultHash = document.location.hash.replace('#', '').trim();

    if (defaultHash != "") {
        $('#search_q').val(defaultHash)
    }

    let statTable = $('#proxytable').DataTable({
        "ajax": "https://api.metawat.ch/nodes.json",
        "order": [
            [5, "desc"]
        ],
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
        columnDefs: [{
            responsivePriority: -3,
            targets: -7
        }],
        "columns": [{
                "data": "timestamp",
                render: function (data, type, row) {
                    if (type === "sort" || type === "type") {
                        return data;
                    }
                    return formatDate(new Date(data * 1000))
                }
            },
            // {
            //     "data": "name",
            //     "visible": false,
            // },
            {
                "data": "to",
                render: function (data, type, row) {
                    if (type === "sort" || type === "type") {
                        return data;
                    }
                    const stripName = data.substr(0, 12) + "…";
                    const nodeName = row.name.substr(0, 47);
                    return '<a class="is-family-monospace ellipsis" href="/address/' + data + '/info">' + stripName + '</a> / ' + nodeName.linkify();
                }
            },
            {
                "data": "rate",
                "visible": false,
            },
            {
                "data": "qps"
            },
            {
                "data": "rps"
            },
            {
                "data": "rps_avg"
            },
            {
                "data": "trust"
            },
            {
                "data": "success",
                render: function (data, type, row) {
                    if (type === "sort" || type === "type") {
                        return data;
                    }
                    return data == "true" ? "online" : "offline";
                }
            },
        ],
        "createdRow": function (row, data, index) {
            if (data.success == "false") {
                $('td', row).eq(7).addClass('has-text-danger');
                $(row).addClass('has-text-danger');
            } else {
                $('td', row).eq(7).addClass('has-text-success');
            }
        }
    });

    statTable.on('search.dt', function () {
        statTable.search() != "" && setTimeout(() => {
            document.location = "#" + statTable.search();
            document.title = 'MetaWat.ch - ' + statTable.search();
        }, 250)
    });

    setInterval(function () {
        statTable.ajax.reload();
    }, 10000);

    $('#search_q').on('keyup', function () {
        statTable.search(this.value).draw();
    });

    function formatDate(date) {
        let diff = new Date() - date; // the difference in milliseconds

        if (diff < 1000) { // less than 1 second
            return 'right now';
        }

        let sec = Math.floor(diff / 1000); // convert diff to seconds

        if (sec < 60) {
            return sec + ' sec. ago';
        }

        let min = Math.floor(diff / 60000); // convert diff to minutes
        if (min < 60) {
            return min + ' min. ago';
        }

        // format the date
        // add leading zeroes to single-digit day/month/hours/minutes
        let d = date;
        d = [
            '0' + d.getDate(),
            '0' + (d.getMonth() + 1),
            '' + d.getFullYear(),
            '0' + d.getHours(),
            '0' + d.getMinutes()
        ].map(component => component.slice(-2)); // take last 2 digits of every component

        // join the components into date
        return d.slice(0, 3).join('.') + ' ' + d.slice(3).join(':');
    }

    String.prototype.linkify = function () {
        return this.replace(/(^|\s)@(\w{3,25})/g, "$1<a target=\"_blank\" href=\"https://t.me/$2\">@$2</a>");
    };
});