var importPollTimer = null;
var oldDirectoryHandler = null;

document.addEventListener('DOMContentLoaded', function() {
    var uploadForm = document.getElementById('uploadForm');
    var directoryForm = document.getElementById('directoryForm');
    var bookFile = document.getElementById('bookFile');
    var uploadPreview = document.getElementById('uploadPreview');
    var uploadResult = document.getElementById('uploadResult');
    var directoryResult = document.getElementById('directoryResult');

    if (bookFile) {
        bookFile.addEventListener('change', function(e) {
            var file = e.target.files[0];
            if (file && uploadPreview) {
                var sizeMB = (file.size / (1024 * 1024)).toFixed(2);
                uploadPreview.innerHTML = '<div class="preview-item"><strong>' + escapeHtml(file.name) + '</strong><span>' + sizeMB + ' MB</span></div>';
                uploadPreview.style.display = 'block';
            }
        });
    }

    if (uploadForm) {
        uploadForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            var fileInput = document.getElementById('bookFile');
            if (!fileInput) return;
            var file = fileInput.files[0];
            if (!file) {
                if (uploadResult) uploadResult.innerHTML = '<div class="error">\u0412\u044b\u0431\u0435\u0440\u0438\u0442\u0435 \u0444\u0430\u0439\u043b</div>';
                return;
            }
            if (uploadResult) uploadResult.innerHTML = '<div class="loading">\u0417\u0430\u0433\u0440\u0443\u0437\u043a\u0430...</div>';
            var formData = new FormData();
            formData.append('file', file);
            try {
                var response = await apiFetch(API_BASE + '/import/file', { method: 'POST', body: formData });
                var data = await response.json();
                if (response.ok) {
                    var html;
                    if (data.duplicate) {
                        html = '<div class="warning"><h4>\u041a\u043d\u0438\u0433\u0430 \u0443\u0436\u0435 \u0435\u0441\u0442\u044c \u0432 \u0431\u0438\u0431\u043b\u0438\u043e\u0442\u0435\u043a\u0435!</h4><p>' + escapeHtml(data.message || '\u0414\u0443\u0431\u043b\u0438\u043a\u0430\u0442') + '</p></div>';
                    } else {
                        html = '<div class="success"><h4>\u041a\u043d\u0438\u0433\u0430 \u0443\u0441\u043f\u0435\u0448\u043d\u043e \u0438\u043c\u043f\u043e\u0440\u0442\u0438\u0440\u043e\u0432\u0430\u043d\u0430!</h4><ul>';
                        html += '<li><strong>\u041d\u0430\u0437\u0432\u0430\u043d\u0438\u0435:</strong> ' + escapeHtml(data.title || 'N/A') + '</li>';
                        var authorsList = Array.isArray(data.authors) ? data.authors : (data.authors ? [data.authors] : []);
                        html += '<li><strong>\u0410\u0432\u0442\u043e\u0440\u044b:</strong> ' + escapeHtml(authorsList.join(', ') || 'N/A') + '</li>';
                        html += '<li><strong>\u042f\u0437\u044b\u043a:</strong> ' + (data.language || 'N/A') + '</li>';
                        html += '</ul></div>';
                    }
                    if (uploadResult) uploadResult.innerHTML = html;
                    fileInput.value = '';
                    if (uploadPreview) uploadPreview.style.display = 'none';
                    if (!data.duplicate && typeof loadAuthorsWithState === 'function' && document.getElementById('authorsTree')) {
                        loadAuthorsWithState(saveExpandedState());
                    }
                } else {
                    if (uploadResult) uploadResult.innerHTML = '<div class="error">\u041e\u0448\u0438\u0431\u043a\u0430: ' + escapeHtml(data.error || '\u041d\u0435\u0438\u0437\u0432\u0435\u0441\u0442\u043d\u0430\u044f \u043e\u0448\u0438\u0431\u043a\u0430') + '</div>';
                }
            } catch (error) {
                if (uploadResult) uploadResult.innerHTML = '<div class="error">\u041e\u0448\u0438\u0431\u043a\u0430 \u0437\u0430\u0433\u0440\u0443\u0437\u043a\u0438: ' + escapeHtml(error.message) + '</div>';
            }
        });
    }

    if (directoryForm) {
        if (oldDirectoryHandler) {
            directoryForm.removeEventListener('submit', oldDirectoryHandler);
        }
        oldDirectoryHandler = async function(e) {
            e.preventDefault();
            var dirInput = document.getElementById('directoryPath');
            if (!dirInput) return;
            var dirPath = dirInput.value.trim();
            if (!dirPath) {
                if (directoryResult) directoryResult.innerHTML = '<div class="error">\u0412\u0432\u0435\u0434\u0438\u0442\u0435 \u043f\u0443\u0442\u044c \u043a \u043f\u0430\u043f\u043a\u0435</div>';
                return;
            }
            if (directoryResult) directoryResult.innerHTML = '<div class="loading">\u0418\u043c\u043f\u043e\u0440\u0442 \u0437\u0430\u043f\u0443\u0449\u0435\u043d...</div>';
            var formData = new FormData();
            formData.append('directory', dirPath);
            try {
                var response = await apiFetch(API_BASE + '/import/directory', { method: 'POST', body: formData });
                var data = await response.json();
                if (response.ok && data.started) {
                    showImportProgress(dirPath, data.total);
                } else {
                    if (directoryResult) directoryResult.innerHTML = '<div class="error">' + escapeHtml(data.error || '\u041e\u0448\u0438\u0431\u043a\u0430 \u0437\u0430\u043f\u0443\u0441\u043a\u0430 \u0438\u043c\u043f\u043e\u0440\u0442\u0430') + '</div>';
                }
            } catch (error) {
                if (directoryResult) directoryResult.innerHTML = '<div class="error">' + escapeHtml(error.message) + '</div>';
            }
        };
        directoryForm.addEventListener('submit', oldDirectoryHandler);
    }

    checkImportOnLoad();
});

function switchToImportTab() {
    var adminTabs = document.querySelectorAll('.admin-tab');
    if (adminTabs.length > 0) {
        document.querySelectorAll('.admin-tab').forEach(function(t) { t.classList.remove('active'); });
        document.querySelectorAll('.admin-content').forEach(function(c) { c.classList.remove('active'); });
        var importTab = document.querySelector('.admin-tab[data-tab="import"]');
        var importContent = document.getElementById('tab-import');
        if (importTab) importTab.classList.add('active');
        if (importContent) importContent.classList.add('active');
        return;
    }
    var tabs = document.querySelectorAll('.tab');
    var contents = document.querySelectorAll('.tab-content');
    tabs.forEach(function(t) { t.classList.remove('active'); });
    contents.forEach(function(c) { c.classList.remove('active'); });
    var importTab = document.querySelector('.tab[data-tab="import"]');
    var importContent = document.getElementById('import');
    if (importTab) {
        importTab.classList.add('active');
    }
    if (importContent) importContent.classList.add('active');
}

function showImportProgress(dirPath, total) {
    switchToImportTab();
    var container = document.getElementById('importProgressArea') || document.getElementById('directoryResult');
    var panel = document.getElementById('importProgressPanel');
    if (!panel) {
        panel = document.createElement('div');
        panel.className = 'import-progress-panel';
        panel.id = 'importProgressPanel';
        panel.innerHTML = '<div class="import-progress-header">' +
            '<h4>\u0418\u043c\u043f\u043e\u0440\u0442 \u043a\u043d\u0438\u0433</h4>' +
            '<span id="importProgressCount">0 / ' + total + '</span></div>' +
            '<div class="import-progress-bar"><div id="importProgressFill" class="import-progress-fill" style="width:0%"></div></div>' +
            '<div id="importCurrentFile" class="import-current-file"></div>' +
            '<div id="importResults" class="import-results"></div>' +
            '<button id="cancelImportBtn" class="btn btn-danger" style="display:none">\u041f\u0440\u0435\u0440\u0432\u0430\u0442\u044c \u0438\u043c\u043f\u043e\u0440\u0442</button>' +
            '<div id="importFinalSummary" style="display:none"></div>';
        container.innerHTML = '';
        container.appendChild(panel);
    } else {
        var countEl = document.getElementById('importProgressCount');
        if (countEl) countEl.textContent = '0 / ' + total;
    }
    container.style.display = '';

    document.getElementById('cancelImportBtn').addEventListener('click', async function() {
        try {
            await apiFetch(API_BASE + '/import/cancel', { method: 'POST' });
        } catch (e) {}
    });

    startImportPolling();
}

function startImportPolling() {
    if (importPollTimer) {
        clearInterval(importPollTimer);
    }
    importPollTimer = setInterval(pollImportStatus, 1000);
    pollImportStatus();
}

function pollImportStatus() {
    apiFetch(API_BASE + '/import/status')
        .then(function(r) { return r.json(); })
        .then(function(state) {
            updateImportUI(state);
            if (!state.running) {
                if (importPollTimer) {
                    clearInterval(importPollTimer);
                    importPollTimer = null;
                }
                finishImportUI(state);
            }
        })
        .catch(function() {
            if (importPollTimer) {
                clearInterval(importPollTimer);
                importPollTimer = null;
            }
        });
}

function updateImportUI(state) {
    var panel = document.getElementById('importProgressPanel');
    if (!panel) return;

    var countEl = document.getElementById('importProgressCount');
    var fillEl = document.getElementById('importProgressFill');
    var currentEl = document.getElementById('importCurrentFile');
    var resultsEl = document.getElementById('importResults');
    var cancelBtn = document.getElementById('cancelImportBtn');

    if (countEl) {
        countEl.textContent = (state.completed + state.errors) + ' / ' + state.total;
    }
    if (fillEl && state.total > 0) {
        var pct = Math.round(((state.completed + state.errors) / state.total) * 100);
        fillEl.style.width = pct + '%';
    }

    if (resultsEl) {
        var html = '';
        if (state.items && state.items.length > 0) {
            var lastItems = state.items.slice(-20);
            html = '<div class="import-items-list">';
            for (var i = 0; i < lastItems.length; i++) {
                var item = lastItems[i];
                var icon = '';
                var label = '';
                if (item.status === 'pending') { icon = '\u23F3'; label = '\u0432 \u043e\u0447\u0435\u0440\u0435\u0434\u0438'; }
                else if (item.status === 'processing') { icon = '\u23F3'; label = '\u043e\u0431\u0440\u0430\u0431\u0430\u0442\u044b\u0432\u0430\u0435\u0442\u0441\u044f'; }
                else if (item.status === 'done') { icon = '\u2713'; label = '\u0433\u043e\u0442\u043e\u0432\u043e'; }
                else if (item.status === 'error') { icon = '\u2717'; label = '\u043e\u0448\u0438\u0431\u043a\u0430'; }
                else if (item.status === 'skipped') { icon = '\u21BA'; label = '\u0434\u0443\u0431\u043b\u0438\u043a\u0430\u0442'; }
                else if (item.status === 'cancelled') { icon = '\u26A0'; label = '\u043E\u0442\u043C\u0435\u043D\u0435\u043D'; }
                html += '<div class="import-item import-item-' + item.status + '">' +
                    '<span class="import-icon">' + icon + '</span> ' +
                    escapeHtml(item.title || item.file || '') +
                    ' <span class="import-label">' + label + '</span></div>';
            }
            html += '</div>';
        }
        resultsEl.innerHTML = html;
    }

    if (currentEl) {
        if (state.running) {
            var processing = (state.items || []).filter(function(it) { return it.status === 'processing'; });
            if (processing.length > 0) {
                currentEl.textContent = '\u041E\u0431\u0440\u0430\u0431\u0430\u0442\u044B\u0432\u0430\u0435\u0442\u0441\u044F: ' + escapeHtml(processing[0].title || processing[0].file || '');
            } else {
                currentEl.textContent = '';
            }
        } else {
            currentEl.textContent = '';
        }
    }

    if (cancelBtn && state.running) {
        cancelBtn.style.display = 'inline-block';
    } else if (cancelBtn) {
        cancelBtn.style.display = 'none';
    }
}

function finishImportUI(state) {
    var panel = document.getElementById('importProgressPanel');
    if (!panel) return;

    var cancelBtn = document.getElementById('cancelImportBtn');
    if (cancelBtn) cancelBtn.style.display = 'none';

    var summaryEl = document.getElementById('importFinalSummary');
    if (summaryEl) {
        var done = (state.items || []).filter(function(it) { return it.status === 'done'; }).length;
        var skipped = (state.items || []).filter(function(it) { return it.status === 'skipped'; }).length;
        var errors = (state.items || []).filter(function(it) { return it.status === 'error'; }).length;
        var cancelled = (state.items || []).filter(function(it) { return it.status === 'cancelled'; }).length;
        var now = new Date();
        var dateStr = now.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit' });
        var durationStr = '';
        if (state.start_time) {
            var diffSec = Math.floor((now.getTime() / 1000) - state.start_time);
            if (diffSec < 60) {
                durationStr = diffSec + ' \u0441\u0435\u043A';
            } else if (diffSec < 3600) {
                durationStr = Math.floor(diffSec / 60) + ' \u043C\u0438\u043D ' + (diffSec % 60) + ' \u0441\u0435\u043A';
            } else {
                var h = Math.floor(diffSec / 3600);
                var m = Math.floor((diffSec % 3600) / 60);
                durationStr = h + ' \u0447 ' + m + ' \u043C\u0438\u043D';
            }
        }
        var html = '<div class="import-summary">' +
            '<h4>\u0418\u043C\u043F\u043E\u0440\u0442 \u0437\u0430\u0432\u0435\u0440\u0448\u0435\u043D</h4>' +
            '<p>\u0417\u0430\u0432\u0435\u0440\u0448\u0435\u043D: ' + dateStr + '</p>' +
            '<p>\u0414\u043B\u0438\u0442\u0435\u043B\u044C\u043D\u043E\u0441\u0442\u044C: ' + durationStr + '</p>' +
            '<p>\u0417\u0430\u0433\u0440\u0443\u0436\u0435\u043D\u043E: ' + done + '</p>' +
            '<p>\u0414\u0443\u0431\u043B\u0438\u043A\u0430\u0442\u043E\u0432: ' + skipped + '</p>' +
            '<p>\u041E\u0448\u0438\u0431\u043E\u043A: ' + errors + '</p>';
        if (cancelled > 0) {
            html += '<p>\u041F\u0440\u0435\u0440\u0432\u0430\u043D\u043E: ' + cancelled + '</p>';
        }
        html += '</div>';
        summaryEl.innerHTML = html;
        summaryEl.style.display = 'block';
    }

    if (typeof loadAuthorsWithState === 'function' && document.getElementById('authorsTree')) {
        loadAuthorsWithState(saveExpandedState());
    }
}

function checkImportOnLoad() {
    apiFetch(API_BASE + '/import/status')
        .then(function(r) { return r.json(); })
        .then(function(state) {
            if (state.running) {
                showImportProgress('', state.total);
                updateImportUI(state);
                startImportPolling();
            } else if (state.items && state.items.length > 0) {
                showImportProgress('', state.total);
                updateImportUI(state);
                finishImportUI(state);
            }
        })
        .catch(function() {});
}

function checkImportStatus() {
    apiFetch(API_BASE + '/import/status')
        .then(function(r) { return r.json(); })
        .then(function(state) {
            if (state.running) {
                showImportProgress('', state.total);
                updateImportUI(state);
                startImportPolling();
            } else if (state.items && state.items.length > 0) {
                var container = document.getElementById('importProgressArea') || document.getElementById('directoryResult');
                if (container) {
                    showImportProgress('', state.total);
                    updateImportUI(state);
                    finishImportUI(state);
                }
            }
        })
        .catch(function() {});
}

function escapeHtml(text) {
    if (!text) return '';
    var div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
