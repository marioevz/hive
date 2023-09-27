import 'bootstrap';
import $ from 'jquery';

import * as routes from './routes.js';
import { makeLink } from './html.js';

// updateHeader populates the page header with version information from hive.json.
export function updateHeader() {
    $.ajax({
        type: 'GET',
        url: routes.resultsRoot + 'hive.json',
        dataType: 'json',
        cache: false,
        success: function(data) {
            console.log('hive.json:', data);
            $('#hive-instance-info').html(hiveInfoHTML(data));
        },
        error: function(xhr, status, error) {
            console.log('error fetching hive.json:', error);
        },
    });
}

function hiveInfoHTML(data) {
    var txt = '';
    if (data.buildDate) {
        let date = new Date(data.buildDate).toLocaleString();
        txt += '<span>binary built: ' + date + '</span>';
    }
    if (data.binaryCommit) {
        let url = 'https://github.com/ethereum/hive/commits/' + escape(data.binaryCommit);
        let link = makeLink(url, data.binaryCommit.substring(0, 8));
        txt += '<span>binary commit: ' + link.outerHTML + '</span>';
    }
    if (data.simulatorsCommit) {
        let url = 'https://github.com/ethereum/hive/commits/' + escape(data.simulatorsCommit);
        let link = makeLink(url, data.simulatorsCommit.substring(0, 8));
        txt += '<span>simulators commit: ' + link.outerHTML + '</span>';
    }
    return txt;
}

// newXhrWithProgressBar creates an XMLHttpRequest and shows its progress
// in the 'load-progress-bar-container' element.
export function newXhrWithProgressBar() {
    let xhr = new window.XMLHttpRequest();
    xhr.addEventListener('progress', function(evt) {
        if (evt.lengthComputable) {
            showLoadProgress(evt.loaded / evt.total);
        } else {
            showLoadProgress(true);
        }
    });
    xhr.addEventListener('loadend', function(evt) {
        showLoadProgress(false);
    });
    return xhr;
}

export function showLoadProgress(loadState, element) {
    if (!element) {
        element = $('#load-progress-bar-container');
    } else {
        element = $(element);
    }

    if (!loadState) {
        console.log('load finished');
        element.hide();
        return;
    }

    var animated = false;
    if (typeof loadState == 'boolean') {
        loadState = 1.0;
        animated = true;
    }
    let percent = Math.floor(loadState * 100);
    console.log('loading: ' + percent);

    element.show();
    let bar = $('.progress-bar', element);
    bar.toggleClass('progress-bar-animated', animated);
    bar.toggleClass('progress-bar-striped', animated);
    bar.attr('aria-valuenow', '' + percent);
    bar.width('' + percent + '%');
}
