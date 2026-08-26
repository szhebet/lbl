const API = '/api/v1/admin';

function api(url, options) {
    if (!options) options = {};
    if (!options.headers) options.headers = {};
    options.headers['Authorization'] = 'Bearer ' + (typeof authToken !== 'undefined' ? authToken : '');
    return fetch(url, options);
}

async function checkAdminAccess() {
    if (!isAuthenticated() || !authUser) {
        document.body.innerHTML = '<div class="container"><h1>Доступ запрещён</h1><p>Необходимо авторизоваться.</p><a href="/" class="btn">На главную</a></div>';
        return false;
    }
    if (authUser.role === 'viewer' || authUser.role === 'server') {
        document.body.innerHTML = '<div class="container"><h1>Доступ запрещён</h1><p>У вас недостаточно прав для доступа к администрированию.</p><a href="/" class="btn">На главную</a></div>';
        return false;
    }
    return true;
}

var currentRole = authUser ? authUser.role : '';

document.querySelectorAll('.admin-tab').forEach(tab => {
    tab.addEventListener('click', () => {
        document.querySelectorAll('.admin-tab').forEach(t => t.classList.remove('active'));
        document.querySelectorAll('.admin-content').forEach(c => c.classList.remove('active'));
        tab.classList.add('active');
        document.getElementById('tab-' + tab.dataset.tab).classList.add('active');
        if (tab.dataset.tab === 'books' && typeof loadBooks === 'function') { enableDelete = true; loadBooks(); }
        if (tab.dataset.tab === 'import') { checkImportStatus(); }
        if (tab.dataset.tab === 'suggestions') { loadSuggestions(); }
        if (tab.dataset.tab === 'readlists') { loadChildren(); loadListNames(); loadReadlists(); }
        if (tab.dataset.tab === 'neighbours') { loadNeighbours(); }
        if (tab.dataset.tab === 'fedrequests') { loadFedRequests(); }
    });
});

function escapeHtml(text) {
    if (!text) return '';
    var div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function openAdminModal(title, content) {
    var adminModal = document.getElementById('adminModal');
    adminModal.classList.remove('rl-modal-wide', 'rl-modal-extra-wide', 'rl-modal-locked');
    document.getElementById('adminModalTitle').textContent = title;
    document.getElementById('adminModalBody').innerHTML = content;
    adminModal.style.display = 'flex';
    document.getElementById('adminForm').onsubmit = null;
    var footer = document.getElementById('adminForm').querySelector('.modal-footer');
    if (footer) footer.style.display = '';
    var submitBtn = document.getElementById('adminForm').querySelector('.modal-footer .btn[type="submit"]');
    if (submitBtn) { submitBtn.style.display = ''; submitBtn.textContent = 'Сохранить'; }
}

function closeAdminModal() {
    var adminModal = document.getElementById('adminModal');
    adminModal.classList.remove('rl-modal-wide', 'rl-modal-extra-wide', 'rl-modal-locked');
    adminModal.style.display = 'none';
}

document.getElementById('adminModal').addEventListener('click', function(e) {
    if (e.target === this && !this.classList.contains('rl-modal-locked')) closeAdminModal();
});

document.addEventListener('click', function(e) {
    var modal = document.getElementById('editModal');
    if (e.target === modal) closeModal();
});

var storeUsers = [];
var storeAuthors = [];
var storeGenres = [];
var storeTags = [];
var storeNeighbours = [];
var sortState = {};
const PAGE_SIZE = 50;
var usersPage = 1;
var genresPage = 1;
var tagsPage = 1;
var neighboursPage = 1;

function getSortKey(tableId) { return sortState[tableId] ? sortState[tableId].key : 'id'; }
function getSortDir(tableId) { return sortState[tableId] ? sortState[tableId].dir : 'asc'; }

function setupSorting(tableId) {
    var table = document.getElementById(tableId);
    if (!table) return;
    table.querySelectorAll('th[data-sort]').forEach(function(th) {
        th.style.cursor = 'pointer';
        th.addEventListener('click', function() {
            var key = th.dataset.sort;
            if (!sortState[tableId]) sortState[tableId] = { key: 'id', dir: 'asc' };
            if (sortState[tableId].key === key) {
                sortState[tableId].dir = sortState[tableId].dir === 'asc' ? 'desc' : 'asc';
            } else {
                sortState[tableId].key = key;
                sortState[tableId].dir = 'asc';
            }
            table.querySelectorAll('th[data-sort]').forEach(function(h) { h.classList.remove('sorted-asc', 'sorted-desc'); });
            th.classList.add('sorted-' + sortState[tableId].dir);
            applyFilters();
        });
    });
}

function filterData(arr, text, fields) {
    if (!text) return arr;
    var q = text.toLowerCase().replace(/ё/g, 'е');
    return arr.filter(function(item) {
        for (var i = 0; i < fields.length; i++) {
            var val = item[fields[i]];
            if (val && val.toString().toLowerCase().replace(/ё/g, 'е').indexOf(q) !== -1) return true;
        }
        return false;
    });
}

function sortData(arr, key, dir) {
    var sorted = arr.slice();
    sorted.sort(function(a, b) {
        var va = a[key], vb = b[key];
        if (va == null) va = '';
        if (vb == null) vb = '';
        if (typeof va === 'number' && typeof vb === 'number') {
            return dir === 'asc' ? va - vb : vb - va;
        }
        va = va.toString().toLowerCase();
        vb = vb.toString().toLowerCase();
        if (va < vb) return dir === 'asc' ? -1 : 1;
        if (va > vb) return dir === 'asc' ? 1 : -1;
        return 0;
    });
    return sorted;
}

function renderTable(tbody, rows, rowFn) {
    if (!tbody) return;
    if (!rows || rows.length === 0) {
        tbody.innerHTML = '<tr><td colspan="20" class="no-results">Нет данных</td></tr>';
        return;
    }
    tbody.innerHTML = rows.map(rowFn).join('');
}

function renderPagination(total, page, totalPages, tabName) {
    if (totalPages <= 1) return '<span class="page-info">' + total + ' записей</span>';
    var html = '<span class="page-info">' + total + ' записей, стр. ' + page + ' из ' + totalPages + '</span>';
    if (page > 1) {
        html += '<button class="pagination-btn" data-tab="' + tabName + '" data-page="1">«</button>';
        html += '<button class="pagination-btn" data-tab="' + tabName + '" data-page="' + (page - 1) + '">‹</button>';
    }
    var start = Math.max(1, page - 2);
    var end = Math.min(totalPages, page + 2);
    for (var i = start; i <= end; i++) {
        html += '<button class="pagination-btn' + (i === page ? ' active' : '') + '" data-tab="' + tabName + '" data-page="' + i + '">' + i + '</button>';
    }
    if (page < totalPages) {
        html += '<button class="pagination-btn" data-tab="' + tabName + '" data-page="' + (page + 1) + '">›</button>';
        html += '<button class="pagination-btn" data-tab="' + tabName + '" data-page="' + totalPages + '">»</button>';
    }
    return html;
}

function loadUsers() {
    usersPage = 1;
    api(API + '/users').then(r => r.json()).then(users => {
        storeUsers = users;
        applyFilters();
    });
}

function renderUsers() {
    var tbody = document.getElementById('usersTableBody');
    var pagEl = document.getElementById('usersPagination');
    var filterEl = document.getElementById('filter-users');
    if (!tbody || !filterEl) return;
    var filterText = filterEl.value;
    var filtered = filterData(storeUsers, filterText, ['username', 'email', 'role', 'parent_names']);
    var sorted = sortData(filtered, getSortKey('table-users'), getSortDir('table-users'));
    var total = sorted.length;
    var totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    if (usersPage > totalPages) usersPage = totalPages;
    var start = (usersPage - 1) * PAGE_SIZE;
    var pageItems = sorted.slice(start, start + PAGE_SIZE);
    renderTable(tbody, pageItems, function(u) {
        return '<tr>' +
            '<td>' + u.id + '</td>' +
            '<td>' + escapeHtml(u.username) + '</td>' +
            '<td>' + escapeHtml(u.email || '') + '</td>' +
            '<td><span class="badge-role ' + (u.role || '') + '">' + escapeHtml(u.role || '') + '</span></td>' +
            '<td>' + escapeHtml(u.parent_names || '') + '</td>' +
            '<td>' + (u.created_at || '') + '</td>' +
            '<td class="actions">' +
            '<button class="btn btn-small edit-user" data-id="' + u.id + '">✎</button> ' +
            '<button class="btn btn-small btn-secondary delete-user" data-id="' + u.id + '">Удалить</button>' +
            '</td></tr>';
    });
    var pagHtml = renderPagination(total, usersPage, totalPages, 'users');
    if (pagEl) pagEl.innerHTML = pagHtml;
    var pagTop = document.getElementById('usersPaginationTop');
    if (pagTop) pagTop.innerHTML = pagHtml;
}

// ─── Peer servers (api_neighbours) ────────────────────────────

function loadNeighbours() {
    neighboursPage = 1;
    api(API + '/neighbours').then(r => r.json()).then(list => {
        storeNeighbours = list;
        applyFilters();
    });
}

function renderNeighbours() {
    var tbody = document.getElementById('neighboursTableBody');
    var pagEl = document.getElementById('neighboursPagination');
    var filterEl = document.getElementById('filter-neighbours');
    if (!tbody || !filterEl) return;
    var filterText = filterEl.value;
    var filtered = filterData(storeNeighbours, filterText, ['url', 'username', 'server_cert', 'client_cert']);
    var sorted = sortData(filtered, getSortKey('table-neighbours'), getSortDir('table-neighbours'));
    var total = sorted.length;
    var totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    if (neighboursPage > totalPages) neighboursPage = totalPages;
    var start = (neighboursPage - 1) * PAGE_SIZE;
    var pageItems = sorted.slice(start, start + PAGE_SIZE);
    renderTable(tbody, pageItems, function(n) {
        var certs = [];
        if (n.server_cert) certs.push('серверный');
        if (n.client_cert) certs.push('клиентский');
        return '<tr' + (n.disabled ? ' style="opacity:0.5"' : '') + '>' +
            '<td>' + n.id + '</td>' +
            '<td>' + escapeHtml(n.url) + '</td>' +
            '<td>' + escapeHtml(n.username || '') + '</td>' +
            '<td>' + (n.has_password ? 'задан' : '') + '</td>' +
            '<td>' + escapeHtml(certs.join(', ')) + '</td>' +
            '<td>' + (n.disabled ? 'да' : '') + '</td>' +
            '<td class="actions">' +
            '<button class="btn btn-small edit-neighbour" data-id="' + n.id + '">✎</button> ' +
            '<button class="btn btn-small btn-secondary test-neighbour" data-id="' + n.id + '">Тест</button> ' +
            '<button class="btn btn-small btn-secondary delete-neighbour" data-id="' + n.id + '">Удалить</button>' +
            '</td></tr>';
    });
    var pagHtml = renderPagination(total, neighboursPage, totalPages, 'neighbours');
    if (pagEl) pagEl.innerHTML = pagHtml;
    var pagTop = document.getElementById('neighboursPaginationTop');
    if (pagTop) pagTop.innerHTML = pagHtml;
}

// ─── User relation picker (parents / children) ──────────────
// Mirrors the book-edit author picker: each selected user is a row with an
// autocomplete input + popup <select> of all users + a remove button, plus
// an "+ Add" button at the bottom. Selected ids live in hidden .user-row-id.

function userPickerLabel(id) {
    var u = (storeUsers || []).find(function(x){ return x.id === id; });
    return u ? (u.username + ' (#' + u.id + ')') : ('#' + id);
}

function userOptionsHtml(selectedId) {
    var opts = '<option value="">-- Выберите --</option>';
    (storeUsers || []).forEach(function(u) {
        var sel = (u.id === selectedId) ? ' selected' : '';
        opts += '<option value="' + u.id + '"' + sel + '>' + escapeHtml(u.username) + ' (#' + u.id + ')</option>';
    });
    return opts;
}

function userRowHtml(id, containerId, idx) {
    var label = id ? userPickerLabel(id) : '';
    return '<div class="user-picker-row author-row" data-idx="' + idx + '">' +
        '<input type="hidden" class="user-row-input" value="' + (id || '') + '">' +
        '<div class="author-autocomplete-group">' +
            '<input type="text" class="user-picker-autocomplete author-autocomplete" value="' + escapeHtml(label) + '" autocomplete="off" placeholder="Начните вводить имя пользователя...">' +
            '<select class="user-picker-popup author-popup" size="5" style="display:none;margin-top:2px;width:100%">' + userOptionsHtml(id) + '</select>' +
        '</div>' +
        '<button type="button" class="btn-remove-user-row btn-remove-author" data-container="' + containerId + '">✕</button>' +
    '</div>';
}

function addUserPickerRow(containerId) {
    var container = document.getElementById(containerId);
    if (!container) return;
    var idx = container.querySelectorAll('.user-picker-row').length;
    container.insertAdjacentHTML('beforeend', userRowHtml(0, containerId, idx));
    var rows = container.querySelectorAll('.user-picker-row');
    setupUserRow(rows[rows.length - 1]);
}

function initUserPicker(containerId, ids) {
    var container = document.getElementById(containerId);
    if (!container) return;
    ids = ids || [];
    var html = '';
    if (ids.length === 0) {
        html = userRowHtml(0, containerId, 0);
    } else {
        for (var i = 0; i < ids.length; i++) {
            html += userRowHtml(ids[i], containerId, i);
        }
    }
    container.innerHTML = html;
    container.querySelectorAll('.user-picker-row').forEach(function(row){ setupUserRow(row); });
}

function setupUserRow(row) {
    var input = row.querySelector('.user-picker-autocomplete');
    var popup = row.querySelector('.user-picker-popup');
    if (input && popup) setupUserPickerAutocomplete(input, popup);
}

function collectUserPick(containerId) {
    var container = document.getElementById(containerId);
    if (!container) return [];
    var ids = [];
    container.querySelectorAll('.user-row-input').forEach(function(h) {
        if (h.value) ids.push(parseInt(h.value, 10));
    });
    return ids;
}

function setupUserPickerAutocomplete(input, popup) {
    if (!input || !popup) return;
    var fill = function(opt) {
        opt.selected = true;
        var row = input.closest('.user-picker-row');
        var hid = row ? row.querySelector('.user-row-input') : null;
        if (hid) hid.value = opt.value;
        input.value = opt.textContent;
        popup.style.display = 'none';
    };
    input.addEventListener('input', function() {
        var val = this.value.toLowerCase();
        var matched = [];
        for (var i = 0; i < popup.options.length; i++) {
            if (popup.options[i].value === '') continue;
            var m = popup.options[i].textContent.toLowerCase().indexOf(val) !== -1;
            popup.options[i].style.display = m ? '' : 'none';
            if (m) matched.push(popup.options[i]);
        }
        var row = this.closest('.user-picker-row');
        var hid = row ? row.querySelector('.user-row-input') : null;
        if (hid) hid.value = '';
        popup.style.display = (val && matched.length > 0) ? '' : 'none';
    });
    input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
            var val = this.value.toLowerCase();
            if (!val) return;
            var matched = [];
            for (var i = 0; i < popup.options.length; i++) {
                if (popup.options[i].value === '') continue;
                if (popup.options[i].textContent.toLowerCase().indexOf(val) !== -1) matched.push(popup.options[i]);
            }
            if (matched.length === 1) { e.preventDefault(); fill(matched[0]); }
        }
    });
    input.addEventListener('blur', function() {
        var val = this.value.toLowerCase();
        if (!val) return;
        var matched = [];
        for (var i = 0; i < popup.options.length; i++) {
            if (popup.options[i].value === '') continue;
            if (popup.options[i].textContent.toLowerCase().indexOf(val) !== -1) matched.push(popup.options[i]);
        }
        if (matched.length === 1) fill(matched[0]);
    });
    popup.addEventListener('change', function() {
        if (this.value) fill(this.options[this.selectedIndex]);
        else {
            var row = input.closest('.user-picker-row');
            var hid = row ? row.querySelector('.user-row-input') : null;
            if (hid) hid.value = '';
        }
    });
}

// Delegate "add" and "remove" for user picker rows (CSP: no inline handlers)
document.addEventListener('click', function(e) {
    var add = e.target.closest('.add-user-picker-row');
    if (add) {
        e.preventDefault();
        addUserPickerRow(add.dataset.container);
        return;
    }
    var rm = e.target.closest('.btn-remove-user-row');
    if (rm) {
        e.preventDefault();
        var container = document.getElementById(rm.dataset.container);
        var row = rm.closest('.user-picker-row');
        if (container && row) row.remove();
        return;
    }
    var cadd = e.target.closest('.add-child-picker-row');
    if (cadd) {
        e.preventDefault();
        addChildPickerRow(cadd.dataset.container);
        return;
    }
});

function editUser(id) {
    api(API + '/users/' + id).then(r => r.json()).then(u => {
        openAdminModal('Редактировать пользователя #' + id, `
            <div class="form-group">
                <label>Имя пользователя:</label>
                <input type="text" id="f_username" value="${escapeHtml(u.username || '')}">
            </div>
            <div class="form-group">
                <label>Новый пароль (оставьте пустым, чтобы не менять):</label>
                <input type="password" id="f_password">
            </div>
            <div class="form-group">
                <label>Email:</label>
                <input type="email" id="f_email" value="${escapeHtml(u.email || '')}">
            </div>
            <div class="form-group">
                <label>Роль:</label>
                <select id="f_role">
                    <option value="">— не менять —</option>
                    <option value="viewer" ${u.role === 'viewer' ? 'selected' : ''}>viewer</option>
                    <option value="editor" ${u.role === 'editor' ? 'selected' : ''}>editor</option>
                    <option value="admin" ${u.role === 'admin' ? 'selected' : ''}>admin</option>
                    <option value="server" ${u.role === 'server' ? 'selected' : ''}>server</option>
                </select>
            </div>
            <div class="form-group">
                <label>Родители:</label>
                <div id="f_parents_picker" class="user-picker"></div>
                <button type="button" class="btn btn-secondary add-user-picker-row" data-container="f_parents_picker">+ Добавить родителя</button>
            </div>
            <div class="form-group">
                <label>Дети:</label>
                <div id="f_children_picker" class="user-picker"></div>
                <button type="button" class="btn btn-secondary add-user-picker-row" data-container="f_children_picker">+ Добавить ребёнка</button>
            </div>
        `);
        document.getElementById('adminModal').classList.add('rl-modal-locked');
        initUserPicker('f_parents_picker', u.parent_ids);
        initUserPicker('f_children_picker', u.child_ids);
        document.getElementById('adminForm').onsubmit = async function(e) {
            e.preventDefault();
            var body = {};
            var us = document.getElementById('f_username').value;
            var p = document.getElementById('f_password').value;
            var em = document.getElementById('f_email').value;
            var r = document.getElementById('f_role').value;
            if (us) body.username = us;
            if (p) body.password = p;
            if (em) body.email = em;
            if (r) body.role = r;
            body.parent_ids = collectUserPick('f_parents_picker');
            body.child_ids = collectUserPick('f_children_picker');
            var res = await api(API + '/users/' + id, {
                method: 'PUT',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(body)
            });
            if (res.ok) { closeAdminModal(); loadUsers(); }
            else { var d = await res.json(); alert(d.error || 'Error'); }
        };
    });
}

function deleteUser(id) {
    if (!confirm('Удалить пользователя #' + id + '?')) return;
    api(API + '/users/' + id, {method: 'DELETE'}).then(r => {
        if (r.ok || r.status === 204) loadUsers();
        else r.json().then(d => alert(d.error || 'Error'));
    });
}

var addUserBtn = document.getElementById('addUserBtn');
if (addUserBtn) addUserBtn.addEventListener('click', function() {
    openAdminModal('Создать пользователя', `
        <div class="form-group">
            <label>Имя пользователя:</label>
            <input type="text" id="f_username" required>
        </div>
        <div class="form-group">
            <label>Пароль:</label>
            <input type="password" id="f_password" required>
        </div>
        <div class="form-group">
            <label>Email:</label>
            <input type="email" id="f_email">
        </div>
        <div class="form-group">
            <label>Роль:</label>
            <select id="f_role">
                <option value="viewer">viewer</option>
                <option value="editor">editor</option>
                <option value="admin">admin</option>
                <option value="server">server</option>
            </select>
        </div>
        <div class="form-group">
            <label>Родители:</label>
            <div id="f_parents_picker" class="user-picker"></div>
            <button type="button" class="btn btn-secondary add-user-picker-row" data-container="f_parents_picker">+ Добавить родителя</button>
        </div>
        <div class="form-group">
            <label>Дети:</label>
            <div id="f_children_picker" class="user-picker"></div>
            <button type="button" class="btn btn-secondary add-user-picker-row" data-container="f_children_picker">+ Добавить ребёнка</button>
        </div>
    `);
    document.getElementById('adminModal').classList.add('rl-modal-locked');
    initUserPicker('f_parents_picker', []);
    initUserPicker('f_children_picker', []);
    document.getElementById('adminForm').onsubmit = async function(e) {
        e.preventDefault();
        var body = {
            username: document.getElementById('f_username').value,
            password: document.getElementById('f_password').value
        };
        var email = document.getElementById('f_email').value;
        var role = document.getElementById('f_role').value;
        if (email) body.email = email;
        if (role) body.role = role;
        body.parent_ids = collectUserPick('f_parents_picker');
        body.child_ids = collectUserPick('f_children_picker');
        var res = await api(API + '/users', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(body)
        });
        if (res.ok) { closeAdminModal(); loadUsers(); }
        else { var d = await res.json(); alert(d.error || 'Error'); }
    };
});

function editNeighbour(id) {
    api(API + '/neighbours/' + id).then(r => r.json()).then(n => {
        openAdminModal('Редактировать сервер #' + id, `
            <div class="form-group">
                <label>URL:</label>
                <input type="text" id="f_url" value="${escapeHtml(n.url || '')}" required>
            </div>
            <div class="form-group">
                <label>Имя пользователя:</label>
                <input type="text" id="f_username" value="${escapeHtml(n.username || '')}">
            </div>
            <div class="form-group">
                <label>Пароль (пусто — не менять):</label>
                <input type="password" id="f_password" placeholder="${n.has_password ? 'Пароль задан' : 'Пароль не задан'}">
            </div>
            <div class="form-group">
                <label><input type="checkbox" id="f_clear_password"> Очистить пароль</label>
            </div>
            <div class="form-group">
                <label>Сертификат сервера соседа (.crt/PEM):</label>
                <textarea id="f_server_cert" rows="3" placeholder="-----BEGIN CERTIFICATE-----...">${escapeHtml(n.server_cert || '')}</textarea>
            </div>
            <div class="form-group">
                <label>Клиентский сертификат (.crt/PEM):</label>
                <textarea id="f_client_cert" rows="3" placeholder="-----BEGIN CERTIFICATE-----...">${escapeHtml(n.client_cert || '')}</textarea>
            </div>
            <div class="form-group">
                <label><input type="checkbox" id="f_disabled" ${n.disabled ? 'checked' : ''}> Отключен (исходящие запросы не отправляются)</label>
            </div>
        `);
        document.getElementById('adminModal').classList.add('rl-modal-locked');
        document.getElementById('adminForm').onsubmit = async function(e) {
            e.preventDefault();
            var body = {
                url: document.getElementById('f_url').value,
                username: document.getElementById('f_username').value,
                server_cert: document.getElementById('f_server_cert').value,
                client_cert: document.getElementById('f_client_cert').value,
                clear_password: document.getElementById('f_clear_password').checked,
                disabled: document.getElementById('f_disabled').checked
            };
            var p = document.getElementById('f_password').value;
            if (p) body.password = p;
            var res = await api(API + '/neighbours/' + id, {
                method: 'PUT',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(body)
            });
            if (res.ok) { closeAdminModal(); loadNeighbours(); }
            else { var d = await res.json(); alert(d.error || 'Error'); }
        };
    });
}

function deleteNeighbour(id) {
    if (!confirm('Удалить сервер #' + id + '?')) return;
    api(API + '/neighbours/' + id, {method: 'DELETE'}).then(r => {
        if (r.ok || r.status === 204) loadNeighbours();
        else r.json().then(d => alert(d.error || 'Error'));
    });
}

async function testNeighbour(id, btn, silent) {
    if (btn) { btn.disabled = true; btn.textContent = '...'; }
    var ok = false;
    var errorMsg = '';
    try {
        var res = await api(API + '/federation/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ neighbour_id: id })
        });
        var data = {};
        try { data = await res.json(); } catch (e) {}
        ok = res.ok && data.ok;
        if (!ok) errorMsg = data.error || ('HTTP ' + res.status);
    } catch (err) {
        ok = false;
        errorMsg = err.message || String(err);
    }
    setTestNeighbourButton(btn, ok);
    if (!ok && errorMsg && !silent) {
        var n = (storeNeighbours || []).find(function(x) { return x.id === id; });
        var title = n ? n.url : '#' + id;
        alert('Ошибка тестирования «' + title + '»:\n\n' + errorMsg);
    }
    return ok;
}

function setTestNeighbourButton(btn, ok) {
    if (!btn) return;
    btn.disabled = false;
    if (ok) {
        btn.className = 'btn btn-small test-neighbour test-ok';
        btn.textContent = 'Тест: Ок';
    } else {
        btn.className = 'btn btn-small test-neighbour test-fail';
        btn.textContent = 'Тест: fail';
    }
}

function getNeighboursPageItems() {
    var filterEl = document.getElementById('filter-neighbours');
    var filterText = filterEl ? filterEl.value : '';
    var filtered = filterData(storeNeighbours || [], filterText, ['url', 'username', 'server_cert', 'client_cert']);
    var sorted = sortData(filtered, getSortKey('table-neighbours'), getSortDir('table-neighbours'));
    var total = sorted.length;
    var totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    var page = Math.min(neighboursPage, totalPages);
    var start = (page - 1) * PAGE_SIZE;
    return sorted.slice(start, start + PAGE_SIZE);
}

async function testAllNeighbours() {
    var items = getNeighboursPageItems();
    if (!items.length) {
        alert('На текущей странице нет серверов для тестирования.');
        return;
    }
    var btnAll = document.getElementById('testAllNeighboursBtn');
    if (btnAll) btnAll.disabled = true;
    var results = { ok: 0, fail: 0 };
    for (var i = 0; i < items.length; i++) {
        var n = items[i];
        var rowBtn = document.querySelector('.test-neighbour[data-id="' + n.id + '"]');
        if (rowBtn) { rowBtn.disabled = true; rowBtn.textContent = '...'; }
        var ok = await testNeighbour(n.id, rowBtn, true);
        if (ok) results.ok++; else results.fail++;
    }
    if (btnAll) btnAll.disabled = false;
}

var addNeighbourBtn = document.getElementById('addNeighbourBtn');
if (addNeighbourBtn) addNeighbourBtn.addEventListener('click', function() {
    openAdminModal('Добавить сервер', `
        <div class="form-group">
            <label>URL (только https://):</label>
            <input type="text" id="f_url" placeholder="https://example.com:9091" required>
        </div>
        <div class="form-group">
            <label>Имя пользователя:</label>
            <input type="text" id="f_username">
        </div>
        <div class="form-group">
            <label>Пароль:</label>
            <input type="password" id="f_password">
        </div>
        <div class="form-group">
            <label>Сертификат сервера соседа (.crt/PEM):</label>
            <textarea id="f_server_cert" rows="3" placeholder="-----BEGIN CERTIFICATE-----..."></textarea>
        </div>
        <div class="form-group">
            <label>Клиентский сертификат (.crt/PEM):</label>
            <textarea id="f_client_cert" rows="3" placeholder="-----BEGIN CERTIFICATE-----..."></textarea>
        </div>
        <div class="form-group">
            <label><input type="checkbox" id="f_disabled"> Отключен</label>
        </div>
    `);
    document.getElementById('adminModal').classList.add('rl-modal-locked');
    document.getElementById('adminForm').onsubmit = async function(e) {
        e.preventDefault();
        var body = {
            url: document.getElementById('f_url').value,
            username: document.getElementById('f_username').value,
            server_cert: document.getElementById('f_server_cert').value,
            client_cert: document.getElementById('f_client_cert').value,
            disabled: document.getElementById('f_disabled').checked
        };
        var p = document.getElementById('f_password').value;
        if (p) body.password = p;
        var res = await api(API + '/neighbours', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(body)
        });
        if (res.ok) { closeAdminModal(); loadNeighbours(); }
        else { var d = await res.json(); alert(d.error || 'Error'); }
    };
});

var testAllNeighboursBtn = document.getElementById('testAllNeighboursBtn');
if (testAllNeighboursBtn) testAllNeighboursBtn.addEventListener('click', testAllNeighbours);

function loadAuthors() {
    authorsPage = 1;
    api(API + '/persons').then(r => r.json()).then(persons => {
        storeAuthors = persons;
        applyFilters();
    });
}

function renderAuthors() {
    var tbody = document.getElementById('authorsTableBody');
    var pagEl = document.getElementById('authorsPagination');
    var filterEl = document.getElementById('filter-authors');
    if (!tbody || !filterEl) return;
    var filterText = filterEl.value;
    var filtered = filterData(storeAuthors, filterText, ['first_name', 'last_name', 'middle_name', 'pseudonym']);
    var sorted = sortData(filtered, getSortKey('table-authors'), getSortDir('table-authors'));
    var total = sorted.length;
    var totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    if (authorsPage > totalPages) authorsPage = totalPages;
    var start = (authorsPage - 1) * PAGE_SIZE;
    var pageItems = sorted.slice(start, start + PAGE_SIZE);
    renderTable(tbody, pageItems, function(p) {
        return '<tr>' +
            '<td>' + p.id + '</td>' +
            '<td>' + escapeHtml(p.last_name) + '</td>' +
            '<td>' + escapeHtml(p.first_name || '') + '</td>' +
            '<td>' + escapeHtml(p.middle_name || '') + '</td>' +
            '<td>' + escapeHtml(p.pseudonym || '') + '</td>' +
            '<td>' + (p.books_count || 0) + '</td>' +
            '<td class="actions">' +
            '<button class="btn btn-small edit-author" data-id="' + p.id + '">✎</button> ' +
            '<button class="btn btn-small btn-secondary delete-author" data-id="' + p.id + '">Удалить</button>' +
            '</td></tr>';
    });
    var pagHtml = renderPagination(total, authorsPage, totalPages, 'authors');
    if (pagEl) pagEl.innerHTML = pagHtml;
    var pagTop = document.getElementById('authorsPaginationTop');
    if (pagTop) pagTop.innerHTML = pagHtml;
}

var addAuthorBtn = document.getElementById('addAuthorBtn');
if (addAuthorBtn) addAuthorBtn.addEventListener('click', function() {
    openAdminModal('Создать автора', `
        <div class="form-group">
            <label>Фамилия:</label>
            <input type="text" id="f_last_name" required>
        </div>
        <div class="form-group">
            <label>Имя:</label>
            <input type="text" id="f_first_name">
        </div>
        <div class="form-group">
            <label>Отчество:</label>
            <input type="text" id="f_middle_name">
        </div>
        <div class="form-group">
            <label>Псевдоним:</label>
            <input type="text" id="f_pseudonym">
        </div>
        <div class="form-group">
            <label>Дата рождения (ГГГГ-ММ-ДД):</label>
            <input type="date" id="f_birth_date">
        </div>
        <div class="form-group">
            <label>Дата смерти (ГГГГ-ММ-ДД):</label>
            <input type="date" id="f_death_date">
        </div>
        <div class="form-group">
            <label>Биография:</label>
            <textarea id="f_biography" rows="3"></textarea>
        </div>
    `);
    document.getElementById('adminForm').onsubmit = async function(e) {
        e.preventDefault();
        var body = {
            last_name: document.getElementById('f_last_name').value,
            first_name: document.getElementById('f_first_name').value,
            middle_name: document.getElementById('f_middle_name').value
        };
        var p = document.getElementById('f_pseudonym').value;
        var bd = document.getElementById('f_birth_date').value;
        var dd = document.getElementById('f_death_date').value;
        var bg = document.getElementById('f_biography').value;
        if (p) body.pseudonym = p;
        if (bd) body.birth_date = bd;
        if (dd) body.death_date = dd;
        if (bg) body.biography = bg;
        var res = await api(API + '/persons', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(body)
        });
        if (res.ok) { closeAdminModal(); loadAuthors(); }
        else { var d = await res.json(); alert(d.error || 'Error'); }
    };
});

function editAuthor(id) {
    api(API + '/persons').then(r => r.json()).then(all => {
        var p = all.find(a => a.id === id) || {};
        openAdminModal('Редактировать автора #' + id, `
            <div class="form-group">
                <label>Фамилия:</label>
                <input type="text" id="f_last_name" value="${escapeHtml(p.last_name || '')}" required>
            </div>
            <div class="form-group">
                <label>Имя:</label>
                <input type="text" id="f_first_name" value="${escapeHtml(p.first_name || '')}">
            </div>
            <div class="form-group">
                <label>Отчество:</label>
                <input type="text" id="f_middle_name" value="${escapeHtml(p.middle_name || '')}">
            </div>
            <div class="form-group">
                <label>Псевдоним:</label>
                <input type="text" id="f_pseudonym" value="${escapeHtml(p.pseudonym || '')}">
            </div>
            <div class="form-group">
                <label>Дата рождения (ГГГГ-ММ-ДД):</label>
                <input type="date" id="f_birth_date" value="${p.birth_date || ''}">
            </div>
            <div class="form-group">
                <label>Дата смерти (ГГГГ-ММ-ДД):</label>
                <input type="date" id="f_death_date" value="${p.death_date || ''}">
            </div>
            <div class="form-group">
                <label>Биография:</label>
                <textarea id="f_biography" rows="3">${escapeHtml(p.biography || '')}</textarea>
            </div>
        `);
        document.getElementById('adminForm').onsubmit = async function(e) {
            e.preventDefault();
            var body = {
                last_name: document.getElementById('f_last_name').value,
                first_name: document.getElementById('f_first_name').value,
                middle_name: document.getElementById('f_middle_name').value
            };
            var ps = document.getElementById('f_pseudonym').value;
            var bd = document.getElementById('f_birth_date').value;
            var dd = document.getElementById('f_death_date').value;
            var bg = document.getElementById('f_biography').value;
            if (ps) body.pseudonym = ps; else body.pseudonym = null;
            if (bd) body.birth_date = bd; else body.birth_date = null;
            if (dd) body.death_date = dd; else body.death_date = null;
            if (bg) body.biography = bg; else body.biography = null;
            var res = await api(API + '/persons/' + id, {
                method: 'PUT',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(body)
            });
            if (res.ok) { closeAdminModal(); loadAuthors(); }
            else { var d = await res.json(); alert(d.error || 'Error'); }
        };
    });
}

function deleteAuthor(id) {
    if (!confirm('Удалить автора #' + id + '?')) return;
    api(API + '/persons/' + id, {method: 'DELETE'}).then(r => {
        if (r.ok || r.status === 204) loadAuthors();
        else r.json().then(d => alert(d.error || 'Error'));
    });
}

function loadGenres() {
    genresPage = 1;
    api(API + '/genres').then(r => r.json()).then(genres => {
        storeGenres = genres;
        applyFilters();
    });
}

function renderGenres() {
    var tbody = document.getElementById('genresTableBody');
    var pagEl = document.getElementById('genresPagination');
    var filterEl = document.getElementById('filter-genres');
    if (!tbody || !filterEl) return;
    var filterText = filterEl.value;
    var filtered = filterData(storeGenres, filterText, ['name', 'ru_name', 'parent_name', 'description']);
    var sorted = sortData(filtered, getSortKey('table-genres'), getSortDir('table-genres'));
    var total = sorted.length;
    var totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    if (genresPage > totalPages) genresPage = totalPages;
    var start = (genresPage - 1) * PAGE_SIZE;
    var pageItems = sorted.slice(start, start + PAGE_SIZE);
    renderTable(tbody, pageItems, function(g) {
        return '<tr>' +
            '<td>' + g.id + '</td>' +
            '<td>' + escapeHtml(g.name) + '</td>' +
            '<td>' + escapeHtml(g.ru_name || '') + '</td>' +
            '<td>' + escapeHtml(g.parent_name || '') + '</td>' +
            '<td>' + escapeHtml(g.description || '') + '</td>' +
            '<td>' + (g.books_count || 0) + '</td>' +
            '<td class="actions">' +
            '<button class="btn btn-small edit-genre" data-id="' + g.id + '">✎</button> ' +
            '<button class="btn btn-small btn-secondary delete-genre" data-id="' + g.id + '">Удалить</button>' +
            '</td></tr>';
    });
    var pagHtml = renderPagination(total, genresPage, totalPages, 'genres');
    if (pagEl) pagEl.innerHTML = pagHtml;
    var pagTop = document.getElementById('genresPaginationTop');
    if (pagTop) pagTop.innerHTML = pagHtml;
}

var addGenreBtn = document.getElementById('addGenreBtn');
if (addGenreBtn) addGenreBtn.addEventListener('click', function() {
    api(API + '/genres').then(r => r.json()).then(allGenres => {
        var options = '<option value="">Нет родителя</option>' +
            allGenres.map(g => '<option value="' + g.id + '">' + escapeHtml(g.name) + '</option>').join('');
        openAdminModal('Создать жанр', `
            <div class="form-group">
                <label>Название:</label>
                <input type="text" id="f_name" required>
            </div>
            <div class="form-group">
                <label>Наименование (рус.):</label>
                <input type="text" id="f_ru_name" placeholder="Русское наименование для отображения">
            </div>
            <div class="form-group">
                <label>Родительский жанр:</label>
                <select id="f_parent_id">${options}</select>
            </div>
            <div class="form-group">
                <label>Описание:</label>
                <textarea id="f_description" rows="3"></textarea>
            </div>
        `);
        document.getElementById('adminForm').onsubmit = async function(e) {
            e.preventDefault();
            var body = {name: document.getElementById('f_name').value};
            var ruName = document.getElementById('f_ru_name').value.trim();
            var pid = document.getElementById('f_parent_id').value;
            var desc = document.getElementById('f_description').value;
            if (ruName) body.ru_name = ruName;
            if (pid) body.parent_id = parseInt(pid);
            if (desc) body.description = desc;
            var res = await api('/api/v1/genres', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(body)
            });
            if (res.ok) { closeAdminModal(); loadGenres(); }
            else { var d = await res.json(); alert(d.error || 'Error'); }
        };
    });
});

function editGenre(id) {
    api(API + '/genres').then(r => r.json()).then(all => {
        var g = all.find(ge => ge.id === id) || {};
        api(API + '/genres').then(r2 => r2.json()).then(allGenres => {
            var options = '<option value="">Нет родителя</option>' +
                allGenres.filter(ge => ge.id !== id).map(ge => '<option value="' + ge.id + '" ' + (g.parent_id === ge.id ? 'selected' : '') + '>' + escapeHtml(ge.name) + '</option>').join('');
            openAdminModal('Редактировать жанр #' + id, `
                <div class="form-group">
                    <label>Название:</label>
                    <input type="text" id="f_name" value="${escapeHtml(g.name || '')}" required>
                </div>
                <div class="form-group">
                    <label>Наименование (рус.):</label>
                    <input type="text" id="f_ru_name" value="${escapeHtml(g.ru_name || '')}" placeholder="Русское наименование для отображения">
                </div>
                <div class="form-group">
                    <label>Родительский жанр:</label>
                    <select id="f_parent_id">${options}</select>
                </div>
                <div class="form-group">
                    <label>Описание:</label>
                    <textarea id="f_description" rows="3">${escapeHtml(g.description || '')}</textarea>
                </div>
            `);
            document.getElementById('adminForm').onsubmit = async function(e) {
                e.preventDefault();
                var body = {name: document.getElementById('f_name').value};
                var pid = document.getElementById('f_parent_id').value;
                var desc = document.getElementById('f_description').value;
                var ruName = document.getElementById('f_ru_name').value.trim();
                if (pid) body.parent_id = parseInt(pid); else body.parent_id = null;
                if (desc) body.description = desc; else body.description = null;
                if (ruName) body.ru_name = ruName;
                var res = await api('/api/v1/genres/' + id, {
                    method: 'PUT',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(body)
                });
                if (res.ok) { closeAdminModal(); loadGenres(); }
                else { var d = await res.json(); alert(d.error || 'Error'); }
            };
        });
    });
}

function deleteGenre(id) {
    if (!confirm('Удалить жанр #' + id + '?')) return;
    api('/api/v1/genres/' + id, {method: 'DELETE'}).then(r => {
        if (r.ok || r.status === 204) loadGenres();
        else r.json().then(d => alert(d.error || 'Error'));
    });
}

function loadTags() {
    tagsPage = 1;
    api(API + '/tags').then(r => r.json()).then(tags => {
        storeTags = tags;
        applyFilters();
    });
}

function renderTableWithPostProcess(tbody, rows, rowFn, postFn) {
    renderTable(tbody, rows, rowFn);
    if (postFn) postFn(tbody);
}

function renderTags() {
    var tbody = document.getElementById('tagsTableBody');
    var pagEl = document.getElementById('tagsPagination');
    var filterEl = document.getElementById('filter-tags');
    if (!tbody || !filterEl) return;
    var filterText = filterEl.value;
    var filtered = filterData(storeTags, filterText, ['name', 'description', 'color']);
    var sorted = sortData(filtered, getSortKey('table-tags'), getSortDir('table-tags'));
    var total = sorted.length;
    var totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    if (tagsPage > totalPages) tagsPage = totalPages;
    var start = (tagsPage - 1) * PAGE_SIZE;
    var pageItems = sorted.slice(start, start + PAGE_SIZE);
    renderTable(tbody, pageItems, function(t) {
        return '<tr>' +
            '<td>' + t.id + '</td>' +
            '<td>' + escapeHtml(t.name) + '</td>' +
            '<td>' + (t.color ? '<span class="color-swatch" data-color="' + escapeHtml(t.color) + '"></span>' + escapeHtml(t.color) : '') + '</td>' +
            '<td>' + escapeHtml(t.description || '') + '</td>' +
            '<td>' + (t.books_count || 0) + '</td>' +
            '<td class="actions">' +
            '<button class="btn btn-small edit-tag" data-id="' + t.id + '">✎</button> ' +
            '<button class="btn btn-small btn-secondary delete-tag" data-id="' + t.id + '">Удалить</button>' +
            '</td></tr>';
    });
    tbody.querySelectorAll('.color-swatch').forEach(function(el) {
        el.style.cssText = 'display:inline-block;width:16px;height:16px;background:' + el.dataset.color + ';border-radius:3px;vertical-align:middle;margin-right:4px';
    });
    var pagHtml = renderPagination(total, tagsPage, totalPages, 'tags');
    if (pagEl) pagEl.innerHTML = pagHtml;
    var pagTop = document.getElementById('tagsPaginationTop');
    if (pagTop) pagTop.innerHTML = pagHtml;
}

var addTagBtn = document.getElementById('addTagBtn');
if (addTagBtn) addTagBtn.addEventListener('click', function() {
    openAdminModal('Создать тег', `
        <div class="form-group">
            <label>Название:</label>
            <input type="text" id="f_name" required>
        </div>
        <div class="form-group">
            <label>Цвет (например #RRGGBB):</label>
            <input type="color" id="f_color">
        </div>
        <div class="form-group">
            <label>Описание:</label>
            <textarea id="f_description" rows="3"></textarea>
        </div>
    `);
    document.getElementById('adminForm').onsubmit = async function(e) {
        e.preventDefault();
        var body = {name: document.getElementById('f_name').value};
        var c = document.getElementById('f_color').value;
        var d = document.getElementById('f_description').value;
        if (c) body.color = c;
        if (d) body.description = d;
        var res = await api('/api/v1/tags', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(body)
        });
        if (res.ok) { closeAdminModal(); loadTags(); }
        else { var d2 = await res.json(); alert(d2.error || 'Error'); }
    };
});

function editTag(id) {
    api(API + '/tags').then(r => r.json()).then(all => {
        var t = all.find(tg => tg.id === id) || {};
        openAdminModal('Редактировать тег #' + id, `
            <div class="form-group">
                <label>Название:</label>
                <input type="text" id="f_name" value="${escapeHtml(t.name || '')}" required>
            </div>
            <div class="form-group">
                <label>Цвет (например #RRGGBB):</label>
                <input type="color" id="f_color" value="${escapeHtml(t.color || '#000000')}">
            </div>
            <div class="form-group">
                <label>Описание:</label>
                <textarea id="f_description" rows="3">${escapeHtml(t.description || '')}</textarea>
            </div>
        `);
        document.getElementById('adminForm').onsubmit = async function(e) {
            e.preventDefault();
            var body = {name: document.getElementById('f_name').value};
            var c = document.getElementById('f_color').value;
            var d = document.getElementById('f_description').value;
            if (c) body.color = c;
            if (d) body.description = d;
            var res = await api(API + '/tags/' + id, {
                method: 'PUT',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(body)
            });
            if (res.ok) { closeAdminModal(); loadTags(); }
            else { var d2 = await res.json(); alert(d2.error || 'Error'); }
        };
    });
}

function deleteTag(id) {
    if (!confirm('Удалить тег?')) return;
    api(API + '/tags/' + id, {method: 'DELETE'}).then(r => {
        if (r.ok || r.status === 204) loadTags();
        else r.json().then(d => alert(d.error || 'Error'));
    });
}

function applyFilters() {
    if (currentRole === 'admin') renderUsers();
    renderNeighbours();
    renderAuthors();
    renderGenres();
    renderTags();
}

function setupFilterInput(id, fn) {
    var el = document.getElementById(id);
    if (!el) return;
    el.addEventListener('keypress', function(e) {
        if (e.key === 'Enter') { e.preventDefault(); fn(); }
    });
    el.addEventListener('blur', fn);
    if (el.tagName === 'SELECT') {
        el.addEventListener('change', fn);
    }
}

setupFilterInput('filter-users', function() { usersPage = 1; applyFilters(); });
setupFilterInput('filter-authors', function() { authorsPage = 1; applyFilters(); });
setupFilterInput('filter-genres', function() { genresPage = 1; applyFilters(); });
setupFilterInput('filter-tags', function() { tagsPage = 1; applyFilters(); });
setupFilterInput('filter-neighbours', function() { neighboursPage = 1; applyFilters(); });

setupFilterInput('bookAuthorFilter', function() { booksPage = 1; loadBooks(); });
setupFilterInput('bookTitleFilter', function() { booksPage = 1; loadBooks(); });
setupFilterInput('bookGenreFilter', function() { booksPage = 1; loadBooks(); });
setupFilterInput('bookDateFrom', function() { booksPage = 1; loadBooks(); });
setupFilterInput('bookDateTo', function() { booksPage = 1; loadBooks(); });

document.getElementById('clearAdminBookFilters')?.addEventListener('click', function() {
    document.getElementById('bookAuthorFilter').value = '';
    document.getElementById('bookTitleFilter').value = '';
    document.getElementById('bookGenreFilter').value = '';
    var df = document.getElementById('bookDateFrom');
    var dt = document.getElementById('bookDateTo');
    if (df) df.value = '';
    if (dt) dt.value = '';
    var sf = document.getElementById('bookStatusFilter');
    if (sf) clearStatusDropdown();
    booksPage = 1;
    loadBooks();
});

// Event delegation for admin table action buttons + pagination
document.addEventListener('click', function(e) {
    var target = e.target.closest('button');
    if (!target) return;

    // Pagination
    if (target.classList.contains('pagination-btn')) {
        var tab = target.dataset.tab;
        var page = parseInt(target.dataset.page);
        if (!tab || !page) return;
        if (tab === 'users') { usersPage = page; renderUsers(); }
        else if (tab === 'authors') { authorsPage = page; renderAuthors(); }
        else if (tab === 'genres') { genresPage = page; renderGenres(); }
        else if (tab === 'tags') { tagsPage = page; renderTags(); }
        else if (tab === 'neighbours') { neighboursPage = page; renderNeighbours(); }
        else if (tab === 'readlists') { readlistsPage = page; loadReadlists(); }
        return;
    }

    if (target.classList.contains('edit-user')) {
        editUser(parseInt(target.dataset.id));
    } else if (target.classList.contains('delete-user')) {
        deleteUser(parseInt(target.dataset.id));
    } else if (target.classList.contains('edit-author')) {
        editAuthor(parseInt(target.dataset.id));
    } else if (target.classList.contains('delete-author')) {
        deleteAuthor(parseInt(target.dataset.id));
    } else if (target.classList.contains('edit-genre')) {
        editGenre(parseInt(target.dataset.id));
    } else if (target.classList.contains('delete-genre')) {
        deleteGenre(parseInt(target.dataset.id));
    } else if (target.classList.contains('edit-tag')) {
        editTag(parseInt(target.dataset.id));
    } else if (target.classList.contains('delete-tag')) {
        deleteTag(parseInt(target.dataset.id));
    } else if (target.classList.contains('edit-rl')) {
        editReadlist(target.dataset.id);
    } else if (target.classList.contains('delete-rl')) {
        deleteReadlist(target.dataset.id);
    } else if (target.classList.contains('rl-shelf-star')) {
        toggleReadlistShelf(target);
    } else if (target.classList.contains('edit-neighbour')) {
        editNeighbour(parseInt(target.dataset.id));
    } else if (target.classList.contains('test-neighbour')) {
        testNeighbour(parseInt(target.dataset.id), target);
    } else if (target.classList.contains('delete-neighbour')) {
        deleteNeighbour(parseInt(target.dataset.id));
    }
});

function setupSuggestionsFilters() {
    ['filter-sug-user', 'filter-sug-bookname', 'filter-sug-author', 'filter-sug-hidden'].forEach(function(id) {
        var el = document.getElementById(id);
        if (!el) return;
        el.addEventListener('change', loadSuggestions);
        el.addEventListener('keypress', function(e) { if (e.key === 'Enter') loadSuggestions(); });
    });
}

async function loadSuggestions() {
    var container = document.getElementById('suggestionsTableContainer');
    container.innerHTML = '<div class="loading">Загрузка...</div>';

    var user = document.getElementById('filter-sug-user').value;
    var bookname = document.getElementById('filter-sug-bookname').value;
    var author = document.getElementById('filter-sug-author').value;
    var hidden = document.getElementById('filter-sug-hidden').value;

    var params = new URLSearchParams();
    if (user) params.set('user', user);
    if (bookname) params.set('bookname', bookname);
    if (author) params.set('author', author);
    params.set('hidden', hidden);

    try {
        var res = await api(API + '/suggestions?' + params.toString());
        if (!res.ok) { container.innerHTML = '<div class="error">Ошибка загрузки</div>'; return; }
        var data = await res.json();
        renderSuggestions(data.items || [], data.total || 0);
    } catch(e) {
        container.innerHTML = '<div class="error">Ошибка: ' + escapeHtml(e.message) + '</div>';
    }
}

function fedDeliveryMarker(d) {
    var cls, label;
    if (d.state === 'fulfilled') { cls = 'sug-fed-blue'; label = 'книга получена'; }
    else if (d.state === 'delivered') { cls = 'sug-fed-green'; label = 'Доставлено'; }
    else if (d.state === 'error') { cls = 'sug-fed-red'; label = 'Ошибка доставки'; }
    else { cls = 'sug-fed-yellow'; label = 'Отправлено'; }
    var tip = d.error ? ' title="' + escapeHtml(d.error) + '"' : '';
    return '<div class="sug-server ' + cls + '"' + tip + '>' +
        '<span class="sug-dot"></span>' + escapeHtml(d.url || '') + ' — ' + escapeHtml(label) + '</div>';
}

function renderSuggestions(items, total) {
    var container = document.getElementById('suggestionsTableContainer');
    if (!items || items.length === 0) {
        container.innerHTML = '<p class="no-results">Нет запросов на книги.</p>';
        return;
    }

    // Deduplicate by read_list_id (multiple suggestion rows per read_list)
    var merged = {};
    for (var i = 0; i < items.length; i++) {
        var item = items[i];
        var rid = item.read_list_id;
        if (!merged[rid]) {
            merged[rid] = {
                read_list_id: rid,
                bookname: item.bookname,
                author: item.author,
                username: item.username,
                looking_for: item.looking_for,
                has_suggestion: false,
                has_edition: false,
                edition_title: '',
                sugg_hidden: true,
                fed_outgoing: !!item.fed_outgoing
            };
        }
        var m = merged[rid];
        if (item.fed_outgoing) m.fed_outgoing = true;
        if (item.fed_deliveries && item.fed_deliveries.length && !m.fed_deliveries) m.fed_deliveries = item.fed_deliveries;
        if (item.has_suggestion) {
            m.has_suggestion = true;
            if (item.sugg_edition_id) {
                m.has_edition = true;
                if (item.edition_title) m.edition_title = item.edition_title;
            }
            if (item.sugg_hidden === false) m.sugg_hidden = false;
        }
    }

    // Convert to array preserving order
    var mergedItems = [];
    var seen = {};
    for (var i = 0; i < items.length; i++) {
        var rid = items[i].read_list_id;
        if (!seen[rid]) {
            seen[rid] = true;
            mergedItems.push(merged[rid]);
        }
    }

    var html = '<p>Всего: ' + total + '</p>';
    html += '<div class="suggestions-list">';
    for (var i = 0; i < mergedItems.length; i++) {
        var item = mergedItems[i];
        var actionBtns = '';

        if (item.fed_outgoing) {
            actionBtns += '<span class="sug-label sug-done">Отправлено по федерации</span> ';
        } else {
            actionBtns += '<button class="btn btn-small suggest-federation" data-id="' + escapeHtml(item.read_list_id) + '">Запросить по федерации</button>';
        }

        if (item.has_suggestion) {
            actionBtns += '<button class="btn btn-small btn-secondary suggest-book" data-id="' + escapeHtml(item.read_list_id) + '">Предложить книгу</button>';
            if (item.has_edition) {
                actionBtns += ' <span class="sug-label sug-done">Предложена</span>';
                if (item.edition_title) {
                    actionBtns += ' <span class="sug-edition-info">' + escapeHtml(item.edition_title) + '</span>';
                }
            }
            actionBtns += ' <button class="btn btn-small btn-secondary suggest-import" data-id="' + escapeHtml(item.read_list_id) + '">Загрузить</button>';
            if (item.sugg_hidden) {
                actionBtns += ' <button class="btn btn-small btn-secondary suggest-show" data-id="' + escapeHtml(item.read_list_id) + '">Показать</button>';
            } else {
                actionBtns += ' <button class="btn btn-small btn-secondary suggest-hide" data-id="' + escapeHtml(item.read_list_id) + '">Скрыть</button>';
            }
        } else {
            actionBtns += '<button class="btn btn-small suggest-book" data-id="' + escapeHtml(item.read_list_id) + '">Предложить книгу</button>';
            actionBtns += ' <button class="btn btn-small suggest-import" data-id="' + escapeHtml(item.read_list_id) + '">Загрузить</button>';
            actionBtns += ' <button class="btn btn-small btn-secondary suggest-hide" data-id="' + escapeHtml(item.read_list_id) + '">Скрыть</button>';
        }

        var hasFed = item.fed_outgoing && item.fed_deliveries && item.fed_deliveries.length;
        html += '<div class="suggestion-card' + (hasFed ? ' has-delivery' : '') + '">' +
            '<div class="sug-main">' +
            '<div class="sug-field"><span class="sug-label">Книга:</span> ' + escapeHtml(item.bookname || '') + '</div>' +
            '<div class="sug-field"><span class="sug-label">Автор:</span> ' + escapeHtml(item.author || '') + '</div>' +
            '<div class="sug-field"><span class="sug-label">Пользователь:</span> ' + escapeHtml(item.username || '') + '</div>' +
            '<div class="sug-field"><span class="sug-label">Ищет:</span> ' + escapeHtml(item.looking_for || '') + '</div>' +
            '</div>' +
            (hasFed ?
                '<div class="sug-delivery"><div class="sug-label" style="margin-bottom:6px">Серверы доставки:</div>' +
                item.fed_deliveries.map(fedDeliveryMarker).join('') + '</div>' : '') +
            '<div class="sug-actions">' + actionBtns + '</div>' +
            '</div>';
    }
    html += '</div>';
    container.innerHTML = html;
}

function closeSuggestModal() {
    document.getElementById('suggestModal').style.display = 'none';
}

function closeSuggestImportModal() {
    document.getElementById('suggestImportModal').style.display = 'none';
    document.getElementById('suggestImportResult').innerHTML = '';
}

// ─── Federated search across neighbour servers ────────────────

function setupFederationSearch() {
    var btn = document.getElementById('federationSearchBtn');
    if (!btn) return;
    if (currentRole !== 'admin') {
        btn.style.display = 'none';
        return;
    }
    btn.addEventListener('click', openFederationSearchModal);
}

function openFederationSearchModal() {
    openAdminModal('Поиск 1й круг', `
        <p style="font-size:13px;color:#666">Поиск выполняется по каталогам соседних серверов, добавленных на вкладке «Серверы» (Администрирование). Если книга найдена на одном из серверов, поиск по остальным прекращается.</p>
        <div class="form-group"><label>Название или автор:</label>
            <input type="text" class="filter-input" id="fedQuery" placeholder="Беседы с Богом"></div>
        <div class="fed-extra">
            <div class="form-group"><label>Автор (уточнение):</label>
                <input type="text" class="filter-input" id="fedAuthor" placeholder="Уолш"></div>
            <div class="form-group"><label>Название (уточнение):</label>
                <input type="text" class="filter-input" id="fedTitle" placeholder="Беседы"></div>
            <div class="form-group"><label>Максимум результатов с сервера:</label>
                <input type="number" class="filter-input" id="fedLimit" value="20" min="1" max="100" style="width:100px"></div>
        </div>
        <div class="form-group">
            <button type="button" class="btn" id="fedSearchBtn">Искать</button>
        </div>
        <div class="fed-results-scroll"><div id="fedResults"></div></div>`);
    var adminModal = document.getElementById('adminModal');
    adminModal.classList.add('rl-modal-extra-wide', 'rl-modal-locked');
    var footer = document.getElementById('adminForm').querySelector('.modal-footer');
    if (footer) {
        var submitBtn = footer.querySelector('.btn[type="submit"]');
        if (submitBtn) submitBtn.style.display = 'none';
        var cancelBtn = footer.querySelector('.btn-secondary');
        if (cancelBtn) cancelBtn.textContent = 'Закрыть';
    }
    document.getElementById('adminForm').onsubmit = function(e) { e.preventDefault(); };
    document.getElementById('fedSearchBtn').addEventListener('click', doFederationSearch);
    document.getElementById('fedResults').addEventListener('click', function(e) {
        var btn = e.target.closest('[data-fed-import]');
        if (btn) federationImport(parseInt(btn.dataset.neighbour, 10), parseInt(btn.dataset.edition, 10), btn);
    });
}

async function doFederationSearch() {
    var resultsEl = document.getElementById('fedResults');
    var query = document.getElementById('fedQuery').value.trim();
    var author = document.getElementById('fedAuthor').value.trim();
    var title = document.getElementById('fedTitle').value.trim();
    if (!query && !author && !title) {
        resultsEl.innerHTML = '<div class="error">Укажите хотя бы одно поле поиска.</div>';
        return;
    }
    var limit = parseInt(document.getElementById('fedLimit').value, 10) || 20;
    resultsEl.innerHTML = '<div class="loading">Поиск по соседним серверам...</div>';
    try {
        var res = await api(API + '/federation/search?limit=' + limit + '&stop_on_first=1', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query: query, author: author, title: title })
        });
        var data = await res.json();
        if (!res.ok) {
            resultsEl.innerHTML = '<div class="error">' + escapeHtml(data.error || 'Ошибка') + '</div>';
            return;
        }
        renderFederationResults(resultsEl, data);
    } catch(err) {
        resultsEl.innerHTML = '<div class="error">Ошибка: ' + escapeHtml(err.message) + '</div>';
    }
}

function renderFederationResults(el, data) {
    var results = data.results || [];
    if (!data.neighbours) {
        el.innerHTML = '<p class="no-results">Не добавлено ни одного соседнего сервера. Добавьте их на вкладке «Серверы» в Администрировании.</p>';
        return;
    }
    var errors = [];
    var rows = '';
    var found = 0;
    for (var i = 0; i < results.length; i++) {
        var r = results[i];
        if (r.error) {
            errors.push(escapeHtml(r.url) + ' — ' + escapeHtml(r.error));
            continue;
        }
        for (var j = 0; j < (r.books || []).length; j++) {
            var b = r.books[j];
            found++;
            rows += '<tr>' +
                '<td class="fed-result-num">' + found + '</td>' +
                '<td>' + escapeHtml(r.url) + '</td>' +
                '<td>' + escapeHtml(b.title) + '</td>' +
                '<td>' + escapeHtml(b.author) + '</td>' +
                '<td><button type="button" class="btn btn-small" data-fed-import="1" ' +
                'data-neighbour="' + r.neighbour_id + '" data-edition="' + b.edition_id + '">Загрузить</button></td>' +
                '</tr>';
        }
    }
    if (found === 0) {
        var html = '<p class="no-results">Книга не найдена ни на одном из серверов.</p>';
        if (errors.length) html += renderFederationErrors(errors);
        el.innerHTML = html;
        return;
    }
    var html = '<table class="admin-table fed-result-table"><thead><tr>' +
        '<th class="fed-result-num">#</th><th>УРЛ сервера</th><th>Книга</th><th>Автор</th><th></th>' +
        '</tr></thead><tbody>' + rows + '</tbody></table>';
    if (errors.length) html += renderFederationErrors(errors);
    el.innerHTML = html;
}

function renderFederationErrors(errors) {
    var html = '<div class="fed-errors">';
    for (var i = 0; i < errors.length; i++) html += '<div class="fed-error">' + errors[i] + '</div>';
    html += '</div>';
    return html;
}

var fedConflictState = null;

async function federationImport(neighbourID, editionID, btn, mode) {
    mode = mode || '';
    if (btn) {
        btn.disabled = true;
        btn.textContent = 'Загрузка...';
    }
    var status = document.createElement('div');
    status.className = 'fed-import-status';
    if (btn && btn.parentNode) btn.parentNode.appendChild(status);
    try {
        var res = await api(API + '/federation/import', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ neighbour_id: neighbourID, edition_id: editionID, mode: mode })
        });
        var data = await res.json();
        if (res.status === 409 && data.conflict) {
            if (status.parentNode) status.parentNode.removeChild(status);
            if (btn) {
                btn.textContent = 'Загрузить';
                btn.disabled = false;
            }
            openFedConflictModal(neighbourID, editionID, data, btn);
            return;
        }
        if (!res.ok) {
            status.className = 'fed-import-status fed-import-error';
            status.textContent = 'Ошибка: ' + escapeHtml(data.error || res.status);
            if (btn) { btn.textContent = 'Загрузить'; btn.disabled = false; }
            return;
        }
        if (data.duplicate) {
            status.className = 'fed-import-status fed-import-dup';
            status.textContent = escapeHtml(data.message || 'Книга уже существует');
        } else {
            status.className = 'fed-import-status fed-import-ok';
            var action = data.mode === 'overwritten' ? 'Перезаписано: ' :
                (data.mode === 'created_new' ? 'Создано новое: ' : 'Импортировано: ');
            status.textContent = action + escapeHtml(data.title || '') +
                (data.authors && data.authors.length ? ' — ' + escapeHtml(data.authors) : '');
        }
        if (btn) btn.textContent = 'Готово';
    } catch(err) {
        status.className = 'fed-import-status fed-import-error';
        status.textContent = 'Ошибка: ' + escapeHtml(err.message);
        if (btn) {
            btn.textContent = 'Загрузить';
            btn.disabled = false;
        }
    }
}

function openFedConflictModal(neighbourID, editionID, data, btn) {
    fedConflictState = { neighbourID: neighbourID, editionID: editionID, btn: btn || null };
    var message = document.getElementById('fedConflictMessage');
    var foundEl = document.getElementById('fedConflictFound');
    var remote = data.remote || {};
    var html = '<p>На домашнем сервере уже есть записи с такими же идентификаторами.</p>' +
        '<table class="admin-table"><tbody>' +
        '<tr><td>Книга на удалённом сервере</td><td>' + escapeHtml(remote.title || '') +
        (remote.author ? ' — ' + escapeHtml(remote.author) : '') + '</td></tr>' +
        '<tr><td>ID произведения (work)</td><td>' + escapeHtml(String(remote.work_id)) + '</td></tr>' +
        '<tr><td>ID издания (edition)</td><td>' + escapeHtml(String(remote.edition_id)) + '</td></tr>' +
        '</tbody></table>';
    var c = data.conflicts || {};
    var collisions = [];
    if (c.authors && c.authors.length) collisions.push('автор с ID ' + c.authors.join(', '));
    if (c.work) collisions.push('произведение (work) с ID ' + escapeHtml(String(remote.work_id)));
    if (c.edition) collisions.push('издание (edition) с ID ' + escapeHtml(String(remote.edition_id)));
    if (collisions.length) html += '<p class="fed-conflict-collisions">Конфликтуют: ' + collisions.join(', ') + '.</p>';
    message.innerHTML = html;

    var found = data.found;
    if (found) {
        foundEl.innerHTML = '<div class="fed-conflict-found-box">' +
            '<strong>Найдено на домашнем сервере:</strong> ' +
            escapeHtml((found.author ? found.author + ' — ' : '') + found.title) +
            ' (ID издания ' + escapeHtml(String(found.edition_id)) + ')' +
            '</div>';
    } else {
        foundEl.innerHTML = '';
    }

    document.getElementById('fedConflictModal').style.display = 'block';
}

function closeFedConflictModal() {
    document.getElementById('fedConflictModal').style.display = 'none';
    if (fedConflictState && fedConflictState.btn) {
        fedConflictState.btn.textContent = 'Загрузить';
        fedConflictState.btn.disabled = false;
    }
    fedConflictState = null;
}

async function fedConflictResolve(mode) {
    if (!fedConflictState) return;
    var state = fedConflictState;
    var nb = document.getElementById('fedConflictOverwriteBtn');
    var cb = document.getElementById('fedConflictCreateBtn');
    nb.disabled = true;
    cb.disabled = true;
    document.getElementById('fedConflictModal').style.display = 'none';
    fedConflictState = null;
    // Re-run import with the chosen mode; the status appears under the
    // originating row's button.
    await federationImport(state.neighbourID, state.editionID, state.btn, mode);
    nb.disabled = false;
    cb.disabled = false;
}

async function openSuggestModal(readListId) {
    document.getElementById('suggestModalTitle').textContent = 'Предложить книгу';

    // Load existing suggestions only (books are fetched on-demand)
    var existingSuggestions = [];
    var delivered = null;
    try {
        var sugRes = await api(API + '/suggestions/readlist/' + encodeURIComponent(readListId)).catch(function() { return {ok: false}; });
        if (sugRes.ok) {
            var sugData = await sugRes.json();
            existingSuggestions = sugData.items || [];
            if (sugData.delivered && sugData.delivered.edition_id) delivered = sugData.delivered;
        }
    } catch(e) {}

    var html = '';
    // A book received from a remote server in response to this request.
    if (delivered) {
        html += '<div class="sug-delivered-block">' +
            '<div class="sug-label" style="color:#2980b9;font-weight:600;margin-bottom:4px">Получено с удалённого сервера:</div>' +
            '<div class="sug-delivered-book"><span class="sug-delivered-source">' + escapeHtml(delivered.fulfilled_by_url || '') + '</span> — ' +
            escapeHtml(delivered.title || '') +
            (delivered.author ? ' <span class="sug-delivered-author">(' + escapeHtml(delivered.author) + ')</span>' : '') +
            ' <span class="sug-delivered-ed">(edition #' + delivered.edition_id + ')</span></div>' +
            '</div>';
    }

    html += '<div class="form-group"><label>ID записи (не редактируется):</label>' +
        '<input type="text" id="sugReadListId" value="' + escapeHtml(readListId) + '" readonly class="readonly-input"></div>';
    html += '<div id="sugSlotsContainer">';
    for (var i = 0; i < existingSuggestions.length; i++) {
        html += buildSuggestionSlot(existingSuggestions[i]);
    }
    html += buildSuggestionSlot(null);
    html += '</div>';

    document.getElementById('suggestModalBody').innerHTML = html;

    attachSuggestionAutocomplete();
    setupSlotAutoAdd();

    document.getElementById('suggestModal').style.display = 'flex';

    document.getElementById('suggestForm').onsubmit = async function(e) {
        e.preventDefault();
        await saveSuggestions(readListId);
    };
}

function buildSuggestionSlot(s) {
    var isExisting = s && s.id;
    var editionId = s && s.edition_id || '';
    var editionTitle = s && s.edition_title || '';
    var hidden = !s || s.hidden;
    var html = '<div class="sug-slot form-group"' + (isExisting ? ' data-sug-id="' + s.id + '"' : '') + '>' +
        '<div class="sug-slot-header">' +
        '<span class="sug-slot-title">' + (isExisting ? 'Предложение #' + s.id : 'Новое предложение') + '</span>' +
        (isExisting ? '<button type="button" class="btn btn-small btn-secondary sug-slot-delete" data-sug-id="' + s.id + '">Удалить</button>' : '') +
        '</div>' +
        '<label>Книга из библиотеки:</label>' +
        '<div class="sug-book-search">' +
        '<input type="hidden" class="sug-edition-id" value="' + editionId + '">' +
        '<input type="text" class="sug-bookname form-input" autocomplete="off" placeholder="Начните вводить название..." value="' + escapeHtml(editionTitle) + '">' +
        '<div class="search-results" style="display:none"></div>' +
        '</div>' +
        '<label><input type="checkbox" class="sug-hidden-chk" ' + (hidden ? 'checked' : '') + '> Скрыть запрос</label>' +
        '</div>';
    return html;
}

function addEmptySlot() {
    var container = document.getElementById('sugSlotsContainer');
    if (!container) return;
    var div = document.createElement('div');
    div.innerHTML = buildSuggestionSlot(null);
    var slot = div.firstElementChild;
    container.appendChild(slot);
    var group = slot.querySelector('.sug-book-search');
    if (group) attachGroupAutocomplete(group);
    setupSlotAutoAdd();
}

function setupSlotAutoAdd() {
    var container = document.getElementById('sugSlotsContainer');
    if (!container) return;
    container.addEventListener('change', function(e) {
        if (e.target.classList.contains('sug-edition-id') && e.target.value) {
            scheduleSlotAutoAdd();
        }
    });
}

function scheduleSlotAutoAdd() {
    setTimeout(function() {
        var container = document.getElementById('sugSlotsContainer');
        if (!container) return;
        var slots = container.querySelectorAll('.sug-slot');
        var lastSlot = slots[slots.length - 1];
        if (!lastSlot) return;
        var lastEdition = lastSlot.querySelector('.sug-edition-id');
        if (lastEdition && lastEdition.value) {
            addEmptySlot();
        }
    }, 50);
}



function attachGroupAutocomplete(group) {
    var input = group.querySelector('.sug-bookname');
    var results = group.querySelector('.search-results');
    var editionInput = group.querySelector('.sug-edition-id');
    if (!input || !results || !editionInput) return;

    var fetchTimer = null;

    input.addEventListener('input', function() {
        var val = this.value.trim();
        editionInput.value = '';

        if (val.length < 1) {
            results.innerHTML = '';
            results.style.display = 'none';
            return;
        }

        results.innerHTML = '<div class="search-result-item" style="color:#999;cursor:default">поиск…</div>';
        results.style.display = '';

        if (fetchTimer) clearTimeout(fetchTimer);
        fetchTimer = setTimeout(function() {
            fetch('/api/v1/books?book=' + encodeURIComponent(val) + '&limit=10', {
                headers: {'Authorization': 'Bearer ' + authToken}
            }).then(function(res) {
                if (!res.ok) return null;
                return res.json();
            }).then(function(data) {
                if (!data || !data.books || data.books.length === 0) {
                    results.innerHTML = '<div class="search-result-item" style="color:#999;cursor:default">— ничего не найдено —</div>';
                    return;
                }
                results.innerHTML = '';
                var seen = {};
                for (var i = 0; i < data.books.length; i++) {
                    var b = data.books[i];
                    var title = (b.edition_title || b.original_title || '').trim();
                    if (!title || seen[title]) continue;
                    seen[title] = true;
                    var author = '';
                    if (typeof b.authors === 'string') author = b.authors;
                    else if (b.authors && typeof b.authors.String === 'string') author = b.authors.String;
                    var el = document.createElement('div');
                    el.className = 'search-result-item';
                    el.dataset.id = b.edition_id;
                    el.dataset.title = title;
                    el.textContent = title + (author ? ' (' + author + ')' : '');
                    (function(item) {
                        // mousedown + preventDefault so the input keeps focus and
                        // the change that triggers slot auto-add registers reliably.
                        item.addEventListener('mousedown', function(e) {
                            e.preventDefault();
                            fillSuggSelection(input, results, editionInput, item);
                        });
                    })(el);
                    results.appendChild(el);
                }
            }).catch(function() {
                results.innerHTML = '<div class="search-result-item" style="color:#999;cursor:default">— ошибка —</div>';
            });
        }, 300);
    });

    input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
            var val = this.value.trim();
            if (val.length < 1) return;
            var items = results.querySelectorAll('.search-result-item');
            var matched = [];
            for (var i = 0; i < items.length; i++) {
                if (!items[i].dataset.id) continue;
                if (items[i].textContent.toLowerCase().indexOf(val.toLowerCase()) !== -1) {
                    matched.push(items[i]);
                }
            }
            if (matched.length === 1) {
                e.preventDefault();
                fillSuggSelection(input, results, editionInput, matched[0]);
            }
        }
    });
}

function attachSuggestionAutocomplete() {
    document.querySelectorAll('.sug-book-search').forEach(function(group) {
        attachGroupAutocomplete(group);
    });
}

function fillSuggSelection(input, results, editionInput, item) {
    editionInput.value = item.dataset.id || '';
    input.value = item.dataset.title || item.textContent;
    results.style.display = 'none';
    // Trigger change so the slot auto-add (new empty slot) fires
    var evt = new Event('change', {bubbles: true});
    editionInput.dispatchEvent(evt);
}

async function saveSuggestions(readListId) {
    var items = [];

    document.querySelectorAll('#sugSlotsContainer .sug-slot').forEach(function(el) {
        var sugId = el.dataset.sugId ? parseInt(el.dataset.sugId) : null;
        var editionInput = el.querySelector('.sug-edition-id');
        var chk = el.querySelector('.sug-hidden-chk');
        var editionId = editionInput ? parseInt(editionInput.value) || null : null;
        // Skip empty new slots
        if (!sugId && !editionId) return;

        var item = {};
        if (sugId) item.id = sugId;
        if (editionId) item.edition_id = editionId;
        item.hidden = chk ? chk.checked : false;
        item._delete = false;
        items.push(item);
    });

    try {
        var res = await api(API + '/suggestions', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({read_list_id: readListId, items: items})
        });
        if (res.ok) {
            closeSuggestModal();
            loadSuggestions();
        } else {
            var err = await res.json();
            alert(err.error || 'Ошибка сохранения');
        }
    } catch(e) {
        alert('Ошибка: ' + e.message);
    }
}

// Event delegation for suggestion actions
document.addEventListener('click', function(e) {
    var target = e.target.closest('button');
    if (!target) return;

    // Suggest book
    if (target.classList.contains('suggest-book')) {
        openSuggestModal(target.dataset.id);
        return;
    }

    // Approve request for federation distribution
    if (target.classList.contains('suggest-federation')) {
        approveFederationRequest(target.dataset.id, target);
        return;
    }

    // Hide suggestion
    if (target.classList.contains('suggest-hide')) {
        hideSuggestion(target.dataset.id);
        return;
    }

    // Show suggestion (unhide)
    if (target.classList.contains('suggest-show')) {
        showSuggestion(target.dataset.id);
        return;
    }

    // Import & suggest
    if (target.classList.contains('suggest-import')) {
        openSuggestImportModal(target.dataset.id);
        return;
    }

    // Delete suggestion slot inside modal
    if (target.classList.contains('sug-slot-delete')) {
        var sugId = parseInt(target.dataset.sugId);
        if (sugId && confirm('Удалить предложение?')) {
            api(API + '/suggestions/' + sugId, {method: 'DELETE'}).then(function(r) {
                if (r.ok) {
                    var slot = target.closest('.sug-slot');
                    if (slot) slot.remove();
                } else {
                    alert('Ошибка удаления');
                }
            });
        }
        return;
    }

    // Delete suggestion from edit section (legacy)
    if (target.classList.contains('sug-edit-delete')) {
        var sugId = parseInt(target.dataset.sugId);
        if (!sugId || !confirm('Удалить предложение?')) return;
        api(API + '/suggestions/' + sugId, {method: 'DELETE'}).then(function(r) {
            if (r.ok) loadSuggestions();
        });
    }
});

async function approveFederationRequest(readListId, btn) {
    if (currentRole !== 'admin') return;
    try {
        if (btn) { btn.disabled = true; btn.textContent = 'Отправка...'; }
        var res = await api(API + '/fed/outgoing', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({read_list_id: readListId})
        });
        if (res.ok) {
            // Push immediately so the request reaches neighbours now instead of
            // waiting for the background distributor's next 5-minute tick.
            api(API + '/fed/push-now', {method: 'POST'}).catch(function(){});
            loadSuggestions();
        } else {
            var err = await res.json();
            alert(err.error || 'Ошибка отправки');
            if (btn) { btn.disabled = false; btn.textContent = 'Запросить по федерации'; }
        }
    } catch(e) {
        alert('Ошибка: ' + e.message);
        if (btn) { btn.disabled = false; btn.textContent = 'Запросить по федерации'; }
    }
}

async function hideSuggestion(readListId) {
    try {
        var resp = await api(API + '/suggestions/readlist/' + encodeURIComponent(readListId));
        if (!resp.ok) { alert('Ошибка загрузки предложений'); return; }
        var data = await resp.json();
        // Endpoint returns {items, delivered} since migration 5.6
        var existing = Array.isArray(data) ? data : (data.items || []);
        var items = existing.map(function(s) {
            var item = {id: s.id, hidden: true};
            if (s.edition_id) item.edition_id = s.edition_id;
            return item;
        });
        if (items.length === 0) {
            // No existing suggestions, create new hidden one
            items = [{edition_id: null, hidden: true}];
        }
        var res = await api(API + '/suggestions', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                read_list_id: readListId,
                items: items
            })
        });
        if (res.ok) {
            loadSuggestions();
        } else {
            var err = await res.json();
            alert(err.error || 'Ошибка');
        }
    } catch(e) {
        alert('Ошибка: ' + e.message);
    }
}

async function showSuggestion(readListId) {
    try {
        var resp = await api(API + '/suggestions/readlist/' + encodeURIComponent(readListId));
        if (!resp.ok) { alert('Ошибка загрузки предложений'); return; }
        var data = await resp.json();
        // Endpoint returns {items, delivered} since migration 5.6
        var existing = Array.isArray(data) ? data : (data.items || []);
        // Build items with hidden=false while preserving edition_id
        var items = existing.map(function(s) {
            var item = {id: s.id, hidden: false};
            if (s.edition_id) item.edition_id = s.edition_id;
            return item;
        });
        if (items.length === 0) { alert('Нет предложений для отображения'); return; }
        var res = await api(API + '/suggestions', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                read_list_id: readListId,
                items: items
            })
        });
        if (res.ok) {
            loadSuggestions();
        } else {
            var err = await res.json();
            alert(err.error || 'Ошибка');
        }
    } catch(e) {
        alert('Ошибка: ' + e.message);
    }
}

async function openSuggestImportModal(readListId) {
    document.getElementById('suggestImportModalTitle').textContent = 'Загрузить книгу';
    document.getElementById('suggestImportFile').value = '';
    document.getElementById('suggestImportResult').innerHTML = '';
    document.getElementById('suggestImportForm').dataset.readListId = readListId;
    document.getElementById('suggestImportModal').style.display = 'flex';
    document.getElementById('suggestImportBtn').disabled = false;

    document.getElementById('suggestImportForm').onsubmit = async function(e) {
        e.preventDefault();
        await doSuggestImport(readListId);
    };
}

async function doSuggestImport(readListId) {
    var fileInput = document.getElementById('suggestImportFile');
    var file = fileInput.files[0];
    if (!file) { alert('Выберите файл'); return; }

    var btn = document.getElementById('suggestImportBtn');
    btn.disabled = true;
    btn.textContent = 'Загрузка...';
    document.getElementById('suggestImportResult').innerHTML = '';

    var formData = new FormData();
    formData.append('file', file);
    formData.append('read_list_id', readListId);

    try {
        var res = await fetch(API + '/suggestions/import', {
            method: 'POST',
            headers: {'Authorization': 'Bearer ' + authToken},
            body: formData
        });
        var result = await res.json();
        if (res.ok || res.status === 201) {
            document.getElementById('suggestImportResult').innerHTML = '<div class="success">' +
                escapeHtml(result.message || 'Книга импортирована') + '</div>';
            setTimeout(function() {
                closeSuggestImportModal();
                loadSuggestions();
            }, 1500);
        } else if (res.status === 409 && result.duplicate) {
            document.getElementById('suggestImportResult').innerHTML = '<div class="warning">' +
                escapeHtml(result.error) + '</div>';
            btn.disabled = false;
            btn.textContent = 'Загрузить';
        } else {
            document.getElementById('suggestImportResult').innerHTML = '<div class="error">' +
                escapeHtml(result.error || 'Ошибка импорта') + '</div>';
            btn.disabled = false;
            btn.textContent = 'Загрузить';
        }
    } catch(e) {
        document.getElementById('suggestImportResult').innerHTML = '<div class="error">' +
            escapeHtml(e.message) + '</div>';
        btn.disabled = false;
        btn.textContent = 'Загрузить';
    }
}

var readlistsPage = 1;
var storeReadlists = [];
var readlistsTotal = 0;
var readlistsSortKey = 'created_at';
var readlistsSortDir = 'desc';
var storeChildren = [];
var selectedChildren = [];
var storeListNames = [];
var selectedListNames = [];
var storeStatuses = ['Не заполнено', 'Прочитано', 'Читаю', 'Отложил', 'Бросил'];
var selectedStatuses = [];

function setupReadlistsFilters() {
    ['filter-rl-bookname', 'filter-rl-author'].forEach(function(id) {
        var el = document.getElementById(id);
        if (!el) return;
        el.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') { readlistsPage = 1; loadReadlists(); }
        });
    });
    var btn = document.getElementById('rlApplyBtn');
    if (btn) btn.addEventListener('click', function() { readlistsPage = 1; loadReadlists(); });
    var createBtn = document.getElementById('rlCreateBtn');
    if (createBtn) createBtn.addEventListener('click', openCreateReadlistModal);
    var createFromTextBtn = document.getElementById('rlCreateFromTextBtn');
    if (createFromTextBtn) createFromTextBtn.addEventListener('click', openCreateFromTextModal);
    var shelfBtn = document.getElementById('rlBulkShelfBtn');
    if (shelfBtn) shelfBtn.addEventListener('click', bulkShelfReadlists);
    var statusBtn = document.getElementById('rlBulkStatusBtn');
    if (statusBtn) statusBtn.addEventListener('click', bulkStatusReadlists);
    var deleteBtn = document.getElementById('rlBulkDeleteBtn');
    if (deleteBtn) deleteBtn.addEventListener('click', bulkDeleteReadlists);
    var filterBtn = document.getElementById('rlListFilterBtn');
    if (filterBtn) filterBtn.addEventListener('click', toggleListDropdown);
    var allBtn = document.getElementById('rlListAllBtn');
    if (allBtn) allBtn.addEventListener('click', function() {
        selectedListNames = storeListNames.slice();
        renderListFilter();
        readlistsPage = 1;
        loadReadlists();
    });
    var clearBtn = document.getElementById('rlListClearBtn');
    if (clearBtn) clearBtn.addEventListener('click', function() {
        selectedListNames = [];
        renderListFilter();
        readlistsPage = 1;
        loadReadlists();
    });
    var chFilterBtn = document.getElementById('rlChildrenFilterBtn');
    if (chFilterBtn) chFilterBtn.addEventListener('click', toggleChildrenDropdown);
    var chAllBtn = document.getElementById('rlChildrenAllBtn');
    if (chAllBtn) chAllBtn.addEventListener('click', function() {
        selectedChildren = (storeChildren || []).map(function(c) { return c.id; });
        renderChildrenFilter();
        readlistsPage = 1;
        loadReadlists();
    });
    var chClearBtn = document.getElementById('rlChildrenClearBtn');
    if (chClearBtn) chClearBtn.addEventListener('click', function() {
        selectedChildren = [];
        renderChildrenFilter();
        readlistsPage = 1;
        loadReadlists();
    });
    var stFilterBtn = document.getElementById('rlStatusFilterBtn');
    if (stFilterBtn) stFilterBtn.addEventListener('click', toggleStatusDropdown);
    var stAllBtn = document.getElementById('rlStatusAllBtn');
    if (stAllBtn) stAllBtn.addEventListener('click', function() {
        selectedStatuses = storeStatuses.slice();
        renderStatusFilter();
        readlistsPage = 1;
        loadReadlists();
    });
    var stClearBtn = document.getElementById('rlStatusClearBtn');
    if (stClearBtn) stClearBtn.addEventListener('click', function() {
        selectedStatuses = [];
        renderStatusFilter();
        readlistsPage = 1;
        loadReadlists();
    });
    document.addEventListener('click', function(e) {
        var wrap = document.getElementById('rlListFilterWrap');
        if (wrap && !wrap.contains(e.target)) {
            var dd = document.getElementById('rlListDropdown');
            if (dd) dd.style.display = 'none';
        }
        var chWrap = document.getElementById('rlChildrenFilterWrap');
        if (chWrap && !chWrap.contains(e.target)) {
            var cdd = document.getElementById('rlChildrenDropdown');
            if (cdd) cdd.style.display = 'none';
        }
        var stWrap = document.getElementById('rlStatusFilterWrap');
        if (stWrap && !stWrap.contains(e.target)) {
            var sdd = document.getElementById('rlStatusDropdown');
            if (sdd) sdd.style.display = 'none';
        }
    });
}

async function loadListNames() {
    try {
        var res = await api(API + '/readlists/names');
        if (!res.ok) return;
        var data = await res.json();
        storeListNames = data.items || [];
        renderListFilter();
    } catch(e) { /* ignore */ }
}

function renderListFilter() {
    var btn = document.getElementById('rlListFilterBtn');
    if (!btn) return;
    if (selectedListNames.length === 0) {
        btn.textContent = 'Списки: все';
    } else {
        btn.textContent = 'Списки: ' + selectedListNames.length;
    }
    var list = document.getElementById('rlListList');
    if (!list) return;
    var html = '';
    (storeListNames || []).forEach(function(n) {
        var checked = selectedListNames.indexOf(n) !== -1;
        html += '<label class="rl-children-list-item"><input type="checkbox" class="rl-list-check" data-name="' + escapeHtml(n) + '"' + (checked ? ' checked' : '') + '> ' +
            escapeHtml(n) + '</label>';
    });
    if (!html) html = '<div class="no-results">Нет списков</div>';
    list.innerHTML = html;
    list.querySelectorAll('.rl-list-check').forEach(function(cb) {
        cb.addEventListener('change', function() {
            var name = cb.dataset.name;
            if (cb.checked) {
                if (selectedListNames.indexOf(name) === -1) selectedListNames.push(name);
            } else {
                selectedListNames = selectedListNames.filter(function(x) { return x !== name; });
            }
            renderListFilter();
            readlistsPage = 1;
            loadReadlists();
        });
    });
}

function toggleListDropdown() {
    var dd = document.getElementById('rlListDropdown');
    if (!dd) return;
    dd.style.display = dd.style.display === 'none' ? '' : 'none';
}

function renderStatusFilter() {
    var btn = document.getElementById('rlStatusFilterBtn');
    if (!btn) return;
    if (selectedStatuses.length === 0) {
        btn.textContent = 'Статус: все';
    } else if (selectedStatuses.length === storeStatuses.length && storeStatuses.length > 0) {
        btn.textContent = 'Статус: все (' + storeStatuses.length + ')';
    } else {
        btn.textContent = 'Статус: ' + selectedStatuses.length;
    }
    var list = document.getElementById('rlStatusList');
    if (!list) return;
    var html = '';
    (storeStatuses || []).forEach(function(s) {
        var checked = selectedStatuses.indexOf(s) !== -1;
        html += '<label class="rl-children-list-item"><input type="checkbox" class="rl-status-check" data-name="' + escapeHtml(s) + '"' + (checked ? ' checked' : '') + '> ' +
            escapeHtml(s) + '</label>';
    });
    if (!html) html = '<div class="no-results">Нет статусов</div>';
    list.innerHTML = html;
    list.querySelectorAll('.rl-status-check').forEach(function(cb) {
        cb.addEventListener('change', function() {
            var name = cb.dataset.name;
            if (cb.checked) {
                if (selectedStatuses.indexOf(name) === -1) selectedStatuses.push(name);
            } else {
                selectedStatuses = selectedStatuses.filter(function(x) { return x !== name; });
            }
            renderStatusFilter();
            readlistsPage = 1;
            loadReadlists();
        });
    });
}

function toggleStatusDropdown() {
    var dd = document.getElementById('rlStatusDropdown');
    if (!dd) return;
    dd.style.display = dd.style.display === 'none' ? '' : 'none';
}

async function loadChildren() {
    try {
        var res = await api(API + '/readlists/children');
        if (!res.ok) return;
        var data = await res.json();
        storeChildren = data.items || [];
        renderChildrenFilter();
    } catch(e) { /* ignore */ }
}

function renderChildrenFilter() {
    var btn = document.getElementById('rlChildrenFilterBtn');
    if (!btn) return;
    if (selectedChildren.length === 0) {
        btn.textContent = 'Дети: все';
    } else if (selectedChildren.length === storeChildren.length && storeChildren.length > 0) {
        btn.textContent = 'Дети: все (' + storeChildren.length + ')';
    } else {
        btn.textContent = 'Дети: ' + selectedChildren.length;
    }
    var list = document.getElementById('rlChildrenList');
    if (!list) return;
    var html = '';
    (storeChildren || []).forEach(function(c) {
        var checked = selectedChildren.indexOf(c.id) !== -1;
        html += '<label class="rl-children-list-item"><input type="checkbox" class="rl-child-check" data-id="' + c.id + '"' + (checked ? ' checked' : '') + '> ' +
            escapeHtml(c.username) + ' (#' + c.id + ')</label>';
    });
    if (!html) html = '<div class="no-results">Нет детей</div>';
    list.innerHTML = html;
    list.querySelectorAll('.rl-child-check').forEach(function(cb) {
        cb.addEventListener('change', function() {
            var id = parseInt(cb.dataset.id, 10);
            if (cb.checked) {
                if (selectedChildren.indexOf(id) === -1) selectedChildren.push(id);
            } else {
                selectedChildren = selectedChildren.filter(function(x) { return x !== id; });
            }
            renderChildrenFilter();
            readlistsPage = 1;
            loadReadlists();
        });
    });
}

function toggleChildrenDropdown() {
    var dd = document.getElementById('rlChildrenDropdown');
    if (!dd) return;
    dd.style.display = dd.style.display === 'none' ? '' : 'none';
}

function setupReadlistsSorting() {
    var table = document.getElementById('table-readlists');
    if (!table) return;
    table.querySelectorAll('th[data-sort]').forEach(function(th) {
        th.style.cursor = 'pointer';
        th.addEventListener('click', function() {
            var key = th.dataset.sort;
            if (readlistsSortKey === key) {
                readlistsSortDir = readlistsSortDir === 'asc' ? 'desc' : 'asc';
            } else {
                readlistsSortKey = key;
                readlistsSortDir = 'asc';
            }
            table.querySelectorAll('th[data-sort]').forEach(function(h) { h.classList.remove('sorted-asc', 'sorted-desc'); });
            th.classList.add('sorted-' + readlistsSortDir);
            renderReadlists();
        });
    });
}

function readlistFilterParams() {
    var bookname = document.getElementById('filter-rl-bookname') ? document.getElementById('filter-rl-bookname').value : '';
    var author = document.getElementById('filter-rl-author') ? document.getElementById('filter-rl-author').value : '';
    var params = new URLSearchParams();
    if (selectedChildren.length > 0) params.set('user_ids', selectedChildren.join(','));
    if (selectedListNames.length > 0) params.set('listnames', selectedListNames.join(','));
    if (selectedStatuses.length > 0) params.set('statuses', selectedStatuses.join(','));
    if (bookname) params.set('bookname', bookname);
    if (author) params.set('author', author);
    return params;
}

async function loadReadlists() {
    var body = document.getElementById('readlistsTableBody');
    if (body) body.innerHTML = '<tr><td colspan="9" class="loading">Загрузка...</td></tr>';

    var params = readlistFilterParams();
    params.set('limit', PAGE_SIZE);
    params.set('offset', (readlistsPage - 1) * PAGE_SIZE);

    try {
        var res = await api(API + '/readlists?' + params.toString());
        if (!res.ok) {
            if (body) body.innerHTML = '<tr><td colspan="9" class="error">Ошибка загрузки</td></tr>';
            return;
        }
        var data = await res.json();
        storeReadlists = data.items || [];
        readlistsTotal = data.total || 0;
        renderReadlists();
    } catch(e) {
        if (body) body.innerHTML = '<tr><td colspan="9" class="error">Ошибка: ' + escapeHtml(e.message) + '</td></tr>';
    }
}

function readlistShelfValue(item) {
    return item.book_id ? (item.on_shelf ? 2 : 1) : 0;
}

function renderReadlists() {
    var body = document.getElementById('readlistsTableBody');
    if (!body) return;

    var sorted = storeReadlists.slice();
    sorted.sort(function(a, b) {
        var va = a[readlistsSortKey], vb = b[readlistsSortKey];
        if (readlistsSortKey === 'created_at') {
            va = va ? va : '';
            vb = vb ? vb : '';
        }
        if (readlistsSortKey === 'on_shelf') {
            var sa = readlistShelfValue(a), sb = readlistShelfValue(b);
            return readlistsSortDir === 'asc' ? sa - sb : sb - sa;
        }
        if (va == null) va = '';
        if (vb == null) vb = '';
        if (typeof va === 'number' && typeof vb === 'number') {
            return readlistsSortDir === 'asc' ? va - vb : vb - va;
        }
        va = va.toString().toLowerCase();
        vb = vb.toString().toLowerCase();
        if (va < vb) return readlistsSortDir === 'asc' ? -1 : 1;
        if (va > vb) return readlistsSortDir === 'asc' ? 1 : -1;
        return 0;
    });

    if (!sorted || sorted.length === 0) {
        body.innerHTML = '<tr><td colspan="9" class="no-results">Нет данных</td></tr>';
    } else {
        body.innerHTML = sorted.map(function(item) {
            var status = item.status ? escapeHtml(item.status) : '';
            var star = item.book_id
                ? '<button class="rl-shelf-star" data-id="' + item.book_id + '" title="На полку" ' + (item.on_shelf ? 'aria-pressed="true"' : '') + '>' + (item.on_shelf ? '★' : '☆') + '</button>'
                : '';
            return '<tr>' +
                '<td>' + escapeHtml(item.username || '') + '</td>' +
                '<td>' + escapeHtml(item.listname || '') + '</td>' +
                '<td>' + escapeHtml(item.author || '') + '</td>' +
                '<td>' + escapeHtml(item.bookname || '') + '</td>' +
                '<td>' + status + '</td>' +
                '<td>' + escapeHtml(item.created_by_username || '') + '</td>' +
                '<td>' + escapeHtml(formatDateTime(item.created_at)) + '</td>' +
                '<td class="center">' + star + '</td>' +
                '<td class="actions">' +
                '<button class="btn btn-small edit-rl" data-id="' + escapeHtml(item.id) + '">✏️</button> ' +
                '<button class="btn btn-small btn-danger delete-rl" data-id="' + escapeHtml(item.id) + '">🗑️</button>' +
                '</td>' +
                '</tr>';
        }).join('');
    }

    var totalPages = Math.max(1, Math.ceil(readlistsTotal / PAGE_SIZE));
    var pagHtml = renderPagination(readlistsTotal, readlistsPage, totalPages, 'readlists');
    var pt = document.getElementById('readlistsPaginationTop');
    var pb = document.getElementById('readlistsPagination');
    if (pt) pt.innerHTML = pagHtml;
    if (pb) pb.innerHTML = pagHtml;
}

// ─── Children picker for read-list creation (mirrors author picker) ──────
// Rows are sourced from storeChildren (only current user's children).

function childPickerLabel(id) {
    var c = (storeChildren || []).find(function(x){ return x.id === id; });
    return c ? (c.username + ' (#' + c.id + ')') : ('#' + id);
}

function childOptionsHtml(selectedId) {
    var opts = '<option value="">-- Выберите --</option>';
    (storeChildren || []).forEach(function(c) {
        var sel = (c.id === selectedId) ? ' selected' : '';
        opts += '<option value="' + c.id + '"' + sel + '>' + escapeHtml(c.username) + ' (#' + c.id + ')</option>';
    });
    return opts;
}

function childRowHtml(id, containerId, idx) {
    var label = id ? childPickerLabel(id) : '';
    return '<div class="user-picker-row author-row" data-idx="' + idx + '">' +
        '<input type="hidden" class="user-row-input" value="' + (id || '') + '">' +
        '<div class="author-autocomplete-group">' +
            '<input type="text" class="user-picker-autocomplete author-autocomplete" value="' + escapeHtml(label) + '" autocomplete="off" placeholder="Начните вводить имя ребёнка...">' +
            '<select class="user-picker-popup author-popup" size="5" style="display:none;margin-top:2px;width:100%">' + childOptionsHtml(id) + '</select>' +
        '</div>' +
        '<button type="button" class="btn-remove-user-row btn-remove-author" data-container="' + containerId + '">✕</button>' +
    '</div>';
}

function addChildPickerRow(containerId) {
    var container = document.getElementById(containerId);
    if (!container) return;
    var idx = container.querySelectorAll('.user-picker-row').length;
    container.insertAdjacentHTML('beforeend', childRowHtml(0, containerId, idx));
    var rows = container.querySelectorAll('.user-picker-row');
    setupUserRow(rows[rows.length - 1]);
}

function initChildPicker(containerId, ids) {
    var container = document.getElementById(containerId);
    if (!container) return;
    ids = ids || [];
    var html = '';
    if (ids.length === 0) {
        html = childRowHtml(0, containerId, 0);
    } else {
        for (var i = 0; i < ids.length; i++) {
            html += childRowHtml(ids[i], containerId, i);
        }
    }
    container.innerHTML = html;
    container.querySelectorAll('.user-picker-row').forEach(function(row){ setupUserRow(row); });
}

function collectChildPick(containerId) {
    var container = document.getElementById(containerId);
    if (!container) return [];
    var ids = [];
    container.querySelectorAll('.user-row-input').forEach(function(h) {
        if (h.value) ids.push(parseInt(h.value, 10));
    });
    return ids;
}

function openCreateReadlistModal() {
    var statuses = ['Не заполнено', 'Прочитано', 'Читаю', 'Отложил', 'Бросил'];
    var statusOptions = statuses.map(function(s) {
        return '<option value="' + s + '">' + s + '</option>';
    }).join('');
    var prefill = selectedChildren.slice();
    var prefillListname = selectedListNames.length === 1 ? selectedListNames[0] : '';
    openAdminModal('Создать список', `
        <div class="form-group">
            <label>Дети:</label>
            <div id="f_rl_children_picker" class="user-picker"></div>
            <button type="button" class="btn btn-secondary add-child-picker-row" data-container="f_rl_children_picker">+ Добавить ребёнка</button>
        </div>
        <div class="form-group">
            <label>Название списка:</label>
            <input type="text" id="f_rl_new_listname" value="${escapeHtml(prefillListname)}" required>
        </div>
        <div class="form-group">
            <label>Название книги:</label>
            <input type="hidden" id="f_rl_new_book_id" value="">
            <input type="text" id="f_rl_new_bookname" autocomplete="off" placeholder="Начните вводить название...">
            <div id="f_rl_new_book_select" class="search-results" style="display:none"></div>
        </div>
        <div class="form-group">
            <label>Автор:</label>
            <input type="hidden" id="f_rl_new_author_id" value="">
            <input type="text" id="f_rl_new_author" autocomplete="off" placeholder="Начните вводить автора...">
            <div id="f_rl_new_author_select" class="search-results" style="display:none"></div>
        </div>
        <div class="form-group">
            <label>Загрузить книгу:</label>
            <input type="file" id="f_rl_new_bookfile" accept=".fb2,.fb2.zip,.epub,.zip,.pdf,.doc,.docx" class="file-input">
            <button type="button" class="btn btn-secondary" id="f_rl_new_upload_btn">Загрузить файл</button>
            <div id="f_rl_new_upload_msg"></div>
        </div>
        <div class="form-group">
            <label>Статус:</label>
            <select id="f_rl_new_status">${statusOptions}</select>
        </div>
        <div class="form-group">
            <label>Комментарий:</label>
            <textarea id="f_rl_new_comment" rows="3"></textarea>
        </div>
    `);
    initChildPicker('f_rl_children_picker', prefill);
    setupRlBookSearch('f_rl_new_bookname', 'f_rl_new_book_select', 'f_rl_new_book_id', 'f_rl_new_author', 'f_rl_new_author_id', 'f_rl_new_author_select');
    setupRlAuthorSearch('f_rl_new_author', 'f_rl_new_author_select', 'f_rl_new_author_id');
    setupRlBookUpload('f_rl_new_bookfile', 'f_rl_new_upload_btn', 'f_rl_new_upload_msg', 'f_rl_new_bookname', 'f_rl_new_book_id', 'f_rl_new_author', 'f_rl_new_author_id');
    document.getElementById('adminForm').onsubmit = async function(e) {
        e.preventDefault();
        var user_ids = collectChildPick('f_rl_children_picker');
        if (!user_ids.length) { alert('Выберите хотя бы одного ребёнка'); return; }
        var bookIdVal = document.getElementById('f_rl_new_book_id').value;
        var authorIdVal = document.getElementById('f_rl_new_author_id').value;
        var body = {
            user_ids: user_ids,
            listname: document.getElementById('f_rl_new_listname').value,
            bookname: document.getElementById('f_rl_new_bookname').value,
            author: document.getElementById('f_rl_new_author').value,
            book_id: bookIdVal ? parseInt(bookIdVal) : null,
            author_id: authorIdVal ? parseInt(authorIdVal) : null,
            status: document.getElementById('f_rl_new_status').value,
            comment: document.getElementById('f_rl_new_comment').value
        };
        var res = await api(API + '/readlists', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(body)
        });
        if (res.ok) { closeAdminModal(); loadReadlists(); }
        else { var d = await res.json(); alert(d.error || 'Error'); }
    };
}

// Removes all quote/markup characters from a single field so only the plain
// value (title, author) ends up in the column.
function stripBookFieldQuotes(s) {
    return String(s || '')
        .replace(/[«»„“”‘’‚‛`´ʼ'"*_~#|]/g, '')
        .trim();
}

// Parse one pasted line into {author, bookname}. Supported formats:
//   "Автор — Название", "Автор - Название", "Название"
function parseBookFromTextLine(line) {
    line = stripBookFieldQuotes(line);
    if (!line) return null;
    var sepMatch = line.match(/^(.*?)\s+[—–-]\s+(.*)$/);
    if (sepMatch) {
        var author = (sepMatch[1] || '').trim();
        var bookname = (sepMatch[2] || '').trim();
        if (bookname) return { author: author, bookname: bookname };
    }
    return { author: '', bookname: line };
}

// Normalization: one line ("Строка") = one work; fields ("Поля") are the
// author (ФИО or team) and the book/work title. Splits the pasted text into
// lines and parses each into {author, bookname}.
function normalizeBooksText(text) {
    var rows = [];
    String(text || '').split(/\r?\n/).forEach(function(line) {
        var parsed = parseBookFromTextLine(line);
        if (parsed) rows.push(parsed);
    });
    return rows;
}

// Rows currently shown in the works table (after "Применить").
var rlTextRows = [];
var rlTextSuggestTimers = {};
// Library persons used to suggest authors in the works table (mirrors the
// readlist create/edit form).
var rlTextPersons = [];
var rlTextPersonMap = {};

// Loads library book options into the 3rd-column select of row `idx`,
// filtered by the row's author (by id when picked from the suggestion list)
// and title. Debounced when typing.
function loadRlTextBookOptions(idx, debounce, cb) {
    var row = rlTextRows[idx];
    var select = document.querySelector('.rl-text-book-select[data-idx="' + idx + '"]');
    if (!row || !select) return;
    if (debounce) {
        if (rlTextSuggestTimers[idx]) clearTimeout(rlTextSuggestTimers[idx]);
        rlTextSuggestTimers[idx] = setTimeout(function() { loadRlTextBookOptions(idx, false, cb); }, 300);
        return;
    }
    var params = [];
    if (row.author_id) params.push('author_id=' + encodeURIComponent(row.author_id));
    else if (row.author) params.push('author=' + encodeURIComponent(row.author));
    if (row.bookname) params.push('book=' + encodeURIComponent(row.bookname));
    select.innerHTML = '<option value="">поиск…</option>';
    if (!params.length) {
        var emptyHtml = '<option value="">— введите автора/название —</option>';
        select.innerHTML = emptyHtml;
        row._bookSearch = { html: emptyHtml, found: false };
        row._rlFound = false;
        if (cb) cb(false);
        return;
    }
    api('/api/v1/books?' + params.join('&') + '&limit=10').then(function(res) {
        return res.ok ? res.json() : null;
    }).then(function(data) {
        var options = '<option value="">— выбрать книгу —</option>';
        var anyFound = false;
        var autoSelected = false;
        var firstID = null;
        if (data && data.books && data.books.length) {
            data.books.forEach(function(b) {
                var title = (b.edition_title || b.original_title || '').trim();
                if (!title) return;
                var a = typeof b.authors === 'string' ? b.authors
                    : (b.authors && typeof b.authors.String === 'string' ? b.authors.String : '');
                var label = title + (a ? ' (' + a + ')' : '');
                var selected = String(rlTextRows[idx].book_id) === String(b.edition_id) ? ' selected' : '';
                if (!selected && !autoSelected) {
                    selected = ' selected';
                    autoSelected = true;
                    firstID = b.edition_id;
                }
                anyFound = true;
                var safeTitle = escapeHtml(title).replace(/"/g, '&quot;');
                var safeAuthor = escapeHtml(a).replace(/"/g, '&quot;');
                options += '<option value="' + b.edition_id + '" data-title="' + safeTitle + '" data-author="' + safeAuthor + '"' + selected + '>' + escapeHtml(label) + '</option>';
            });
        }
        if (autoSelected) {
            rlTextRows[idx].book_id = firstID;
        }
        if (!anyFound) {
            options = '<option value="">-</option>';
        }
        select.innerHTML = options;
        row._bookSearch = { html: options, found: anyFound };
        row._rlFound = anyFound;
        if (cb) cb(anyFound);
    }).catch(function() {
        select.innerHTML = '<option value="">— ошибка —</option>';
        row._bookSearch = { html: '<option value="">— ошибка —</option>', found: false };
        row._rlFound = false;
        if (cb) cb(false);
    });
}

// Loads the library person list so author fields can offer the exact names
// known to the library (by analogy with the readlist create/edit form).
async function loadRlTextPersons() {
    rlTextPersons = [];
    rlTextPersonMap = {};
    try {
        var res = await api(API + '/persons');
        if (!res.ok) return;
        var persons = await res.json();
        persons.forEach(function(p) {
            var name = ((p.last_name || '') + ' ' + (p.first_name || '')).trim();
            if (!name) return;
            rlTextPersons.push({ id: p.id, name: name });
            rlTextPersonMap[name.toLowerCase()] = p.id;
        });
        document.querySelectorAll('.rl-text-author-select').forEach(function(div) {
            populateRlTextAuthorSelect(parseInt(div.dataset.idx, 10));
        });
    } catch(e) { /* ignore */ }
}

// Fills the suggestion div of row `idx` with all library persons (filtering
// happens on input).
function populateRlTextAuthorSelect(idx) {
    var div = document.querySelector('.rl-text-author-select[data-idx="' + idx + '"]');
    if (!div) return;
    div.innerHTML = '';
    rlTextPersons.forEach(function(p) {
        var el = document.createElement('div');
        el.className = 'search-result-item';
        el.dataset.id = p.id;
        el.dataset.name = p.name;
        el.textContent = p.name;
        div.appendChild(el);
    });
}

// Author text field → suggestion list: typing filters library persons, blur
// fills the only match, clicking a person pins its author_id.
function setupRlTextAuthorSearch(idx) {
    var input = document.querySelector('.rl-text-author-input[data-idx="' + idx + '"]');
    var select = document.querySelector('.rl-text-author-select[data-idx="' + idx + '"]');
    if (!input || !select) return;
    populateRlTextAuthorSelect(idx);
    input.addEventListener('input', function() {
        var row = rlTextRows[idx];
        if (!row) return;
        row.author = this.value;
        row.author_id = null;
        row.book_id = null;
        loadRlTextBookOptions(idx, true);
        var val = this.value.toLowerCase();
        var matched = [];
        Array.prototype.forEach.call(select.querySelectorAll('.search-result-item'), function(item) {
            if (!item.dataset.id) return;
            var m = item.textContent.toLowerCase().indexOf(val) !== -1;
            item.style.display = m ? '' : 'none';
            if (m) matched.push(item);
        });
        select.style.display = (val && matched.length > 0) ? '' : 'none';
    });
    input.addEventListener('blur', function() {
        var row = rlTextRows[idx];
        if (!row) return;
        var val = this.value.toLowerCase();
        var matched = [];
        Array.prototype.forEach.call(select.querySelectorAll('.search-result-item'), function(item) {
            if (!item.dataset.id) return;
            if (item.textContent.toLowerCase().indexOf(val) !== -1) matched.push(item);
        });
        if (matched.length === 1) {
            var item = matched[0];
            input.value = item.dataset.name;
            row.author = item.dataset.name;
            row.author_id = item.dataset.id;
            loadRlTextBookOptions(idx, false);
        }
        select.style.display = 'none';
    });
    // mousedown + preventDefault so the input keeps focus and the blur handler
    // does not hide the list before the pick registers.
    select.addEventListener('mousedown', function(e) {
        var row = rlTextRows[idx];
        if (!row) return;
        var item = e.target.closest('.search-result-item');
        if (!item || !item.dataset.id) return;
        e.preventDefault();
        input.value = item.dataset.name;
        row.author = item.dataset.name;
        row.author_id = item.dataset.id;
        row.book_id = null;
        select.style.display = 'none';
        loadRlTextBookOptions(idx, false);
    });
}

// Renders the normalized works table below the separator: №, editable author
// (with library author suggestions), editable title, and a 3rd column offering
// matching library books (restricted to the selected author when one is picked).
// When `onAllSearched` is given it is invoked once every row's library search
// has finished (rows that already have a cached search result are reused).
function renderRlTextResultTable(onAllSearched) {
    var container = document.getElementById('f_rl_text_result');
    if (!container) return;
    if (!rlTextRows.length) {
        container.innerHTML = '<div class="form-group rl-text-empty">Не найдено ни одной строки</div>';
        return;
    }
    var html = '<table class="admin-table rl-text-result-table"><thead><tr>' +
        '<th class="rl-text-num">№</th>' +
        '<th>Автор</th>' +
        '<th>Название</th>' +
        '<th>Книга из библиотеки</th>' +
        '</tr></thead><tbody>';
    rlTextRows.forEach(function(row, i) {
        html += '<tr>' +
            '<td class="rl-text-num">' + (i + 1) + '</td>' +
            '<td><div class="rl-text-author-cell">' +
            '<input type="text" class="rl-text-author-input" data-idx="' + i + '" autocomplete="off" placeholder="Начните вводить автора..." value="' + escapeHtml(row.author) + '">' +
            '<div class="search-results rl-text-author-select" data-idx="' + i + '" style="display:none"></div>' +
            '</div></td>' +
            '<td><input type="text" class="rl-text-title-input" data-idx="' + i + '" value="' + escapeHtml(row.bookname) + '"></td>' +
            '<td class="rl-text-work"><select class="rl-text-book-select" data-idx="' + i + '"><option value="">поиск…</option></select></td>' +
            '</tr>';
    });
    html += '</tbody></table>';
    container.innerHTML = html;
    container.querySelectorAll('.rl-text-author-input').forEach(function(inp) {
        var idx = parseInt(inp.dataset.idx, 10);
        setupRlTextAuthorSearch(idx);
    });
    container.querySelectorAll('.rl-text-title-input').forEach(function(inp) {
        inp.addEventListener('input', function() {
            var idx = parseInt(this.dataset.idx, 10);
            if (rlTextRows[idx]) {
                rlTextRows[idx].bookname = this.value;
                rlTextRows[idx].book_id = null;
                loadRlTextBookOptions(idx, true);
            }
        });
    });
    container.querySelectorAll('.rl-text-book-select').forEach(function(sel) {
        sel.addEventListener('change', function() {
            var idx = parseInt(this.dataset.idx, 10);
            var row = rlTextRows[idx];
            if (!row) return;
            var opt = this.options[this.selectedIndex];
            row.book_id = this.value ? parseInt(this.value, 10) : null;
            if (row.book_id && opt) {
                var title = opt.dataset.title || '';
                if (title) {
                    row.bookname = title;
                    var titleInput = document.querySelector('.rl-text-title-input[data-idx="' + idx + '"]');
                    if (titleInput) titleInput.value = title;
                }
                var author = opt.dataset.author || '';
                if (author) {
                    var firstAuthor = author.split(',')[0].trim();
                    row.author = firstAuthor;
                    var authorInput = document.querySelector('.rl-text-author-input[data-idx="' + idx + '"]');
                    if (authorInput) authorInput.value = firstAuthor;
                    var aid = rlTextPersonMap[firstAuthor.toLowerCase()];
                    row.author_id = aid || null;
                    loadRlTextBookOptions(idx, false);
                }
            }
        });
    });
    var pending = 0, completed = 0;
    rlTextRows.forEach(function(row, i) {
        if (row._bookSearch) {
            var cachedSel = document.querySelector('.rl-text-book-select[data-idx="' + i + '"]');
            if (cachedSel) cachedSel.innerHTML = row._bookSearch.html;
            return;
        }
        pending++;
        loadRlTextBookOptions(i, false, function() {
            completed++;
            if (onAllSearched && completed === pending) onAllSearched();
        });
    });
    if (onAllSearched && pending === 0) onAllSearched();
}

// Sorts the rows so works with a matching library book come first (stable:
// the relative order inside each group is preserved), then re-renders using
// the cached search results.
function reorderRlTextRowsFoundFirst() {
    rlTextRows.sort(function(a, b) {
        var fa = !!a._rlFound, fb = !!b._rlFound;
        if (fa !== fb) return fa ? -1 : 1;
        return 0;
    });
    renderRlTextResultTable();
}

// Opens the "Создать из текста" modal: paste multiple book titles, one per
// line, optionally in "Автор — Название" format. "Применить" runs the
// normalization and shows a works table below the separator; a record is
// created for every row for the selected children.
function openCreateFromTextModal() {
    rlTextRows = [];
    var prefill = selectedChildren.slice();
    var prefillListname = selectedListNames.length === 1 ? selectedListNames[0] : '';
    openAdminModal('Создать из текста', `
        <div class="form-group">
            <label>Дети:</label>
            <div id="f_rl_text_children_picker" class="user-picker"></div>
            <button type="button" class="btn btn-secondary add-child-picker-row" data-container="f_rl_text_children_picker">+ Добавить ребёнка</button>
        </div>
        <div class="form-group">
            <label>Название списка:</label>
            <input type="text" id="f_rl_text_listname" value="${escapeHtml(prefillListname)}" required>
        </div>
        <div class="form-group">
            <label>Книги (по одной на строку, формат «Автор — Название»):
                <button type="button" class="btn btn-secondary rl-llm-label-btn" id="rlTextPromptBtn">LLM-промпт</button>
                <button type="button" class="btn btn-secondary rl-llm-label-btn" id="rlTextConvertBtn">LLM-преобразовать</button>
                <span class="rl-llm-tip" id="rlTextLlmTip"></span>
            </label>
            <textarea id="f_rl_text_books" rows="8" placeholder="Лев Толстой — Война и мир&#10;Антуан де Сент-Экзюпери — Маленький принц"></textarea>
        </div>
        <hr class="rl-text-sep">
        <div class="form-group">
            <button type="button" class="btn" id="rlTextApplyBtn">Применить</button>
        </div>
        <div id="f_rl_text_result"></div>
    `);
    var adminModal = document.getElementById('adminModal');
    adminModal.classList.add('rl-modal-wide', 'rl-modal-locked');
    initChildPicker('f_rl_text_children_picker', prefill);
    loadRlTextPersons();
    var tip = document.getElementById('rlTextLlmTip');
    var promptTextArea = document.getElementById('f_rl_text_books');

    function rlLlmTip(msg, isError) {
        if (!tip) return;
        tip.textContent = msg;
        tip.classList.toggle('rl-llm-error', !!isError);
        setTimeout(function() { tip.textContent = ''; }, isError ? 6000 : 3000);
    }

    document.getElementById('rlTextPromptBtn').addEventListener('click', function() {
        var text = promptTextArea.value || '';
        api('/api/v1/config').then(function(r) { return r.json(); }).then(function(cfg) {
            var prompt = (cfg && cfg.llm_prompt_convert) || 'Преобразуй к формату Автор - Название произведения следующий текст: \n';
            var promptText = prompt + text;
            function copyDone(ok) {
                rlLlmTip(ok ? 'промпт скопирован в буфер обмена' : 'не удалось скопировать промпт');
            }
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(promptText).then(function() { copyDone(true); }, function() { copyDone(false); });
            } else {
                var ta = document.createElement('textarea');
                ta.value = promptText;
                ta.style.position = 'fixed';
                ta.style.left = '-9999px';
                document.body.appendChild(ta);
                ta.select();
                try { copyDone(document.execCommand('copy')); } catch (e) { copyDone(false); }
                document.body.removeChild(ta);
            }
        }).catch(function() {
            rlLlmTip('не удалось получить промпт');
        });
    });

    document.getElementById('rlTextConvertBtn').addEventListener('click', function() {
        var text = promptTextArea.value.trim();
        if (!text) { rlLlmTip('введите текст для преобразования'); return; }
        rlLlmTip('обработка...');
        var btn = document.getElementById('rlTextConvertBtn');
        var prev = btn.textContent;
        btn.disabled = true;
        btn.textContent = 'Обработка...';
        api('/api/v1/llm/convert', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({text: text})
        }).then(function(res) {
            if (!res.ok) {
                return res.json().then(function(d) {
                    throw new Error((d && d.error) || 'ошибка LLM');
                });
            }
            return res.json();
        }).then(function(data) {
            if (!data || !data.result) { rlLlmTip('LLM вернул пустой ответ'); return; }
            // Strip quote/markup chars line-by-line so each line parses cleanly
            // into author/title columns on "Применить".
            var cleaned = String(data.result || '').split(/\r?\n/).map(function(l) {
                return stripBookFieldQuotes(l);
            }).filter(function(l) { return l !== ''; }).join('\n');
            promptTextArea.value = cleaned;
            // Разносим результат по столбцам таблицы сразу.
            rlTextRows = normalizeBooksText(cleaned);
            renderRlTextResultTable(reorderRlTextRowsFoundFirst);
            rlLlmTip('текст преобразован и разнесён по столбцам');
        }).catch(function(err) {
            var msg = err && err.message ? err.message : 'ошибка LLM';
            rlLlmTip(msg, true);
            alert('Ошибка LLM: ' + msg);
        }).then(function() {
            btn.disabled = false;
            btn.textContent = prev;
        });
    });

    document.getElementById('rlTextApplyBtn').addEventListener('click', function() {
        rlTextRows = normalizeBooksText(document.getElementById('f_rl_text_books').value);
        renderRlTextResultTable(reorderRlTextRowsFoundFirst);
    });
    document.getElementById('adminForm').onsubmit = async function(e) {
        e.preventDefault();
        var user_ids = collectChildPick('f_rl_text_children_picker');
        if (!user_ids.length) { alert('Выберите хотя бы одного ребёнка'); return; }
        var listname = document.getElementById('f_rl_text_listname').value.trim();
        if (!listname) { alert('Укажите название списка'); return; }
        if (!rlTextRows.length) { alert('Нажмите «Применить», чтобы разобрать текст'); return; }
        var created = 0, errors = 0, errMsg = '';
        for (var i = 0; i < rlTextRows.length; i++) {
            var body = {
                user_ids: user_ids,
                listname: listname,
                bookname: rlTextRows[i].bookname,
                author: rlTextRows[i].author,
                book_id: rlTextRows[i].book_id || null,
                author_id: rlTextRows[i].author_id || null,
                status: 'Не заполнено',
                comment: ''
            };
            try {
                var res = await api(API + '/readlists', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(body)
                });
                if (res.ok) { created++; }
                else {
                    errors++;
                    if (!errMsg) { var d = await res.json(); errMsg = d.error || ''; }
                }
            } catch (err) {
                errors++;
                if (!errMsg) errMsg = 'Ошибка сети';
            }
        }
        closeAdminModal();
        if (errors === 0) alert('Создано записей: ' + created);
        else alert('Создано: ' + created + ', ошибок: ' + errors + (errMsg ? ' — ' + errMsg : ''));
        loadReadlists();
    };
}

// ─── Book/author search in the readlist create/edit modal ────────────────
// Mirrors the "Списки чтения" editor on the main page (index.html):
// author results come from the persons list, book results from /books search.

var rlPersonMap = {};

async function setupRlAuthorSearch(inputId, selectId, hiddenId) {
    var input = document.getElementById(inputId);
    var select = document.getElementById(selectId);
    if (!input || !select) return;
    rlPersonMap = {};
    try {
        var res = await api(API + '/persons');
        if (res.ok) {
            var persons = await res.json();
            select.innerHTML = '';
            persons.forEach(function(p) {
                var name = (p.last_name || '') + ' ' + (p.first_name || '');
                rlPersonMap[name.trim().toLowerCase()] = p.id;
                var el = document.createElement('div');
                el.className = 'search-result-item';
                el.dataset.id = p.id;
                el.textContent = name.trim();
                select.appendChild(el);
            });
        }
    } catch(e) { /* ignore */ }
    input.addEventListener('input', function() {
        var val = this.value.toLowerCase();
        var matched = [];
        Array.prototype.forEach.call(select.querySelectorAll('.search-result-item'), function(item) {
            if (!item.dataset.id) return;
            var m = item.textContent.toLowerCase().indexOf(val) !== -1;
            item.style.display = m ? '' : 'none';
            if (m) matched.push(item);
        });
        document.getElementById(hiddenId).value = '';
        select.style.display = (val && matched.length > 0) ? '' : 'none';
    });
    input.addEventListener('blur', function() {
        var val = this.value.toLowerCase();
        var matched = [];
        Array.prototype.forEach.call(select.querySelectorAll('.search-result-item'), function(item) {
            if (!item.dataset.id) return;
            if (item.textContent.toLowerCase().indexOf(val) !== -1) matched.push(item);
        });
        if (matched.length === 1) {
            input.value = matched[0].textContent;
            document.getElementById(hiddenId).value = matched[0].dataset.id;
            select.style.display = 'none';
        }
    });
    select.addEventListener('click', function(e) {
        var item = e.target.closest('.search-result-item');
        if (!item || !item.dataset.id) return;
        input.value = item.textContent;
        document.getElementById(hiddenId).value = item.dataset.id;
        select.style.display = 'none';
    });
}

function setupRlBookSearch(inputId, selectId, hiddenId, authorInputId, authorHiddenId, authorSelectId) {
    var input = document.getElementById(inputId);
    var select = document.getElementById(selectId);
    if (!input || !select) return;
    var timer = null;
    input.addEventListener('input', function() {
        var val = this.value.trim();
        document.getElementById(hiddenId).value = '';
        if (val.length < 1) {
            select.innerHTML = '';
            select.style.display = 'none';
            return;
        }
        select.innerHTML = '<div class="search-result-item" style="color:#999;cursor:default">поиск…</div>';
        select.style.display = '';
        if (timer) clearTimeout(timer);
        timer = setTimeout(function() {
            api('/api/v1/books?book=' + encodeURIComponent(val) + '&limit=10').then(function(res) {
                if (!res.ok) return null;
                return res.json();
            }).then(function(data) {
                if (!data || !data.books || data.books.length === 0) {
                    select.innerHTML = '<div class="search-result-item" style="color:#999;cursor:default">— ничего не найдено —</div>';
                    return;
                }
                select.innerHTML = '';
                for (var i = 0; i < data.books.length; i++) {
                    var b = data.books[i];
                    var title = (b.edition_title || b.original_title || '').trim();
                    if (!title) continue;
                    var author = '';
                    if (typeof b.authors === 'string') author = b.authors;
                    else if (b.authors && typeof b.authors.String === 'string') author = b.authors.String;
                    var firstAuthor = author ? author.split(',')[0].trim() : '';
                    var el = document.createElement('div');
                    el.className = 'search-result-item';
                    el.dataset.id = b.edition_id;
                    el.dataset.title = title;
                    el.dataset.firstAuthor = firstAuthor;
                    el.textContent = title + (author ? ' (' + author + ')' : '');
                    el.addEventListener('click', function(e) {
                        var item = e.currentTarget;
                        document.getElementById(inputId).value = item.dataset.title;
                        document.getElementById(hiddenId).value = item.dataset.id;
                        var fa = item.dataset.firstAuthor || '';
                        if (fa && authorInputId) {
                            document.getElementById(authorInputId).value = fa;
                            var aid = rlPersonMap[fa.toLowerCase()];
                            document.getElementById(authorHiddenId).value = aid || '';
                            var as = document.getElementById(authorSelectId);
                            if (as) as.style.display = 'none';
                        }
                        select.style.display = 'none';
                    });
                    select.appendChild(el);
                }
            }).catch(function() {
                select.innerHTML = '<div class="search-result-item" style="color:#999;cursor:default">— ошибка —</div>';
            });
        }, 300);
    });
}

function setupRlBookUpload(fileId, btnId, msgId, booknameId, bookId, authorId, authorHiddenId) {
    var btn = document.getElementById(btnId);
    if (!btn) return;
    btn.addEventListener('click', async function() {
        var fileInput = document.getElementById(fileId);
        if (!fileInput.files || !fileInput.files.length) { alert('Выберите файл'); return; }
        var msg = document.getElementById(msgId);
        if (msg) msg.textContent = 'Загрузка...';
        var fd = new FormData();
        fd.append('file', fileInput.files[0]);
        try {
            var res = await api('/api/v1/import/file', { method: 'POST', body: fd });
            var data = await res.json();
            if (!res.ok || data.duplicate) {
                if (msg) msg.textContent = (data.message || data.error || 'Ошибка загрузки');
                return;
            }
            document.getElementById(booknameId).value = data.title || '';
            document.getElementById(bookId).value = data.edition_id || '';
            var authors = Array.isArray(data.authors) ? data.authors.join(', ') : (data.authors || '');
            if (authors) {
                var firstAuthor = authors.split(',')[0].trim();
                document.getElementById(authorId).value = firstAuthor;
                var aid = rlPersonMap[firstAuthor.toLowerCase()];
                document.getElementById(authorHiddenId).value = aid || '';
            }
            if (msg) msg.textContent = 'Книга загружена: ' + (data.title || '');
        } catch(e) {
            if (msg) msg.textContent = 'Ошибка: ' + e.message;
        }
    });
}

function formatDateTime(s) {
    if (!s) return '';
    var d = new Date(s);
    if (isNaN(d.getTime())) return escapeHtml(s);
    var dd = String(d.getDate()).padStart(2, '0');
    var mm = String(d.getMonth() + 1).padStart(2, '0');
    var yyyy = d.getFullYear();
    var hh = String(d.getHours()).padStart(2, '0');
    var min = String(d.getMinutes()).padStart(2, '0');
    return dd + '.' + mm + '.' + yyyy + ' ' + hh + ':' + min;
}

function editReadlist(id) {
    var item = storeReadlists.find(function(i) { return i.id === id; });
    if (!item) { alert('Элемент не найден'); return; }
    var statuses = ['Не заполнено', 'Прочитано', 'Читаю', 'Отложил', 'Бросил'];
    var statusOptions = statuses.map(function(s) {
        return '<option value="' + s + '" ' + (item.status === s ? 'selected' : '') + '>' + s + '</option>';
    }).join('');
    openAdminModal('Редактировать элемент списка', `
        <div class="form-group">
            <label>Пользователь:</label>
            <input type="text" value="${escapeHtml(item.username || '')}" disabled>
        </div>
        <div class="form-group">
            <label>Название списка:</label>
            <input type="text" id="f_rl_listname" value="${escapeHtml(item.listname || '')}">
        </div>
        <div class="form-group">
            <label>Название книги:</label>
            <input type="hidden" id="f_rl_book_id" value="${item.book_id || ''}">
            <input type="text" id="f_rl_bookname" autocomplete="off" placeholder="Начните вводить название..." value="${escapeHtml(item.bookname || '')}">
            <div id="f_rl_book_select" class="search-results" style="display:none"></div>
        </div>
        <div class="form-group">
            <label>Автор:</label>
            <input type="hidden" id="f_rl_author_id" value="${item.author_id || ''}">
            <input type="text" id="f_rl_author" autocomplete="off" placeholder="Начните вводить автора..." value="${escapeHtml(item.author || '')}">
            <div id="f_rl_author_select" class="search-results" style="display:none"></div>
        </div>
        <div class="form-group">
            <label>Загрузить книгу:</label>
            <input type="file" id="f_rl_bookfile" accept=".fb2,.fb2.zip,.epub,.zip,.pdf,.doc,.docx" class="file-input">
            <button type="button" class="btn btn-secondary" id="f_rl_upload_btn">Загрузить файл</button>
            <div id="f_rl_upload_msg"></div>
        </div>
        <div class="form-group">
            <label>Статус:</label>
            <select id="f_rl_status">${statusOptions}</select>
        </div>
        <div class="form-group">
            <label>Комментарий:</label>
            <textarea id="f_rl_comment" rows="3">${escapeHtml(item.comment || '')}</textarea>
        </div>
    `);
    setupRlBookSearch('f_rl_bookname', 'f_rl_book_select', 'f_rl_book_id', 'f_rl_author', 'f_rl_author_id', 'f_rl_author_select');
    setupRlAuthorSearch('f_rl_author', 'f_rl_author_select', 'f_rl_author_id');
    setupRlBookUpload('f_rl_bookfile', 'f_rl_upload_btn', 'f_rl_upload_msg', 'f_rl_bookname', 'f_rl_book_id', 'f_rl_author', 'f_rl_author_id');
    document.getElementById('adminForm').onsubmit = async function(e) {
        e.preventDefault();
        var bookIdVal = document.getElementById('f_rl_book_id').value;
        var authorIdVal = document.getElementById('f_rl_author_id').value;
        var body = {
            listname: document.getElementById('f_rl_listname').value,
            bookname: document.getElementById('f_rl_bookname').value,
            author: document.getElementById('f_rl_author').value,
            book_id: bookIdVal ? parseInt(bookIdVal) : null,
            author_id: authorIdVal ? parseInt(authorIdVal) : null,
            status: document.getElementById('f_rl_status').value,
            comment: document.getElementById('f_rl_comment').value
        };
        var res = await api(API + '/readlists/' + encodeURIComponent(id), {
            method: 'PUT',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(body)
        });
        if (res.ok) { closeAdminModal(); loadReadlists(); }
        else { var d = await res.json(); alert(d.error || 'Error'); }
    };
}

function deleteReadlist(id) {
    if (!confirm('Удалить элемент списка?')) return;
    api(API + '/readlists/' + encodeURIComponent(id), {method: 'DELETE'}).then(r => {
        if (r.ok) loadReadlists();
        else r.json().then(d => alert(d.error || 'Error'));
    });
}

function toggleReadlistShelf(starBtn) {
    var bookId = starBtn.dataset.id;
    var onShelf = starBtn.getAttribute('aria-pressed') === 'true';
    api('/api/v1/books/' + bookId + '/shelf', {
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({on_shelf: !onShelf})
    }).then(function(r) {
        if (r.ok) {
            if (onShelf) {
                starBtn.removeAttribute('aria-pressed');
                starBtn.textContent = '☆';
            } else {
                starBtn.setAttribute('aria-pressed', 'true');
                starBtn.textContent = '★';
            }
            var bookIdNum = parseInt(bookId, 10);
            (storeReadlists || []).forEach(function(it) {
                if (it.book_id === bookIdNum || it.edition_id === bookIdNum) {
                    it.on_shelf = !onShelf;
                }
            });
        } else {
            alert('Ошибка при изменении полки');
        }
    }).catch(function() { alert('Ошибка сети'); });
}

function bulkShelfReadlists() {
    if (!confirm('Выложить на полку все книги, прикреплённые к отфильтрованным записям?')) return;
    var params = readlistFilterParams();
    api(API + '/readlists/bulk/shelf?' + params.toString(), {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({})
    }).then(function(r) {
        return r.json().then(function(d) {
            if (r.ok) { alert('На полку: ' + (d.total != null ? d.total : 0)); loadReadlists(); }
            else { alert(d.error || 'Ошибка'); }
        });
    }).catch(function() { alert('Ошибка сети'); });
}

function bulkDeleteReadlists() {
    if (!confirm('Удалить все отфильтрованные записи, созданные вами? Записи других пользователей останутся.')) return;
    var params = readlistFilterParams();
    api(API + '/readlists/bulk/delete?' + params.toString(), {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({})
    }).then(function(r) {
        return r.json().then(function(d) {
            if (r.ok) { alert('Удалено: ' + (d.edited != null ? d.edited : 0)); loadReadlists(); }
            else { alert(d.error || 'Ошибка'); }
        });
    }).catch(function() { alert('Ошибка сети'); });
}

function bulkStatusReadlists() {
    var statuses = ['Не заполнено', 'Прочитано', 'Читаю', 'Отложил', 'Бросил'];
    var options = statuses.map(function(s) {
        return '<option value="' + s + '">' + s + '</option>';
    }).join('');
    openAdminModal('Установить статус', `
        <div class="form-group">
            <label>Статус прочтения:</label>
            <select id="f_rl_bulk_status">${options}</select>
        </div>
        <p style="color:#666;font-size:13px;">Статус будет установлен всем записям, отображаемым в списке.</p>
    `);
    var submitBtn = document.getElementById('adminForm').querySelector('.modal-footer .btn[type="submit"]');
    if (submitBtn) submitBtn.textContent = 'ОК';
    document.getElementById('adminForm').onsubmit = async function(e) {
        e.preventDefault();
        var status = document.getElementById('f_rl_bulk_status').value;
        var params = readlistFilterParams();
        var res = await api(API + '/readlists/bulk/status?' + params.toString(), {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({status: status})
        });
        var d = await res.json();
        if (res.ok) { closeAdminModal(); alert('Обновлено: ' + (d.edited != null ? d.edited : 0)); loadReadlists(); }
        else { alert(d.error || 'Ошибка'); }
    };
}

function bindAdminBackLink() {
    var backLink = document.getElementById('adminBackLink');
    if (!backLink) return;
    backLink.addEventListener('click', function(e) {
        e.preventDefault();
        if (window.history.length > 1) {
            window.history.back();
        } else if (document.referrer) {
            window.location.href = document.referrer;
        } else {
            window.location.href = this.getAttribute('data-back') || '/';
        }
    });
}

document.addEventListener('DOMContentLoaded', async function() {
    if (!await checkAdminAccess()) return;

    // Page "Администрирование" (/administer) — users management, admin only
    if (document.getElementById('tab-users')) {
        if (currentRole !== 'admin') {
            document.body.innerHTML = '<div class="container"><h1>Доступ запрещён</h1>' +
                '<p>Только администратор может управлять пользователями.</p>' +
                '<a href="/admin" class="btn">К управлению</a></div>';
            return;
        }
        setupSorting('table-users');
        setupSorting('table-neighbours');
        loadUsers();
        loadNeighbours();
        bindAdminBackLink();
        return;
    }

    // Page "Управление" (/admin) — catalog management (editor + admin)
    if (currentRole !== 'admin') {
        var adminNav = document.querySelector('.admin-nav-link');
        if (adminNav) adminNav.remove();
        var fedTab = document.querySelector('.admin-tab-fed');
        if (fedTab) fedTab.remove();
        var fedContent = document.getElementById('tab-fedrequests');
        if (fedContent) fedContent.remove();
    }
    setupSorting('table-authors');
    setupSorting('table-genres');
    setupSorting('table-tags');
    loadAuthors();
    loadBooks();
    loadGenres();
    loadTags();
    setupSuggestionsFilters();
    setupFederationSearch();
    setupReadlistsFilters();
    setupReadlistsSorting();
    setupFedRequests();
    bindAdminBackLink();
});

// ─── Requests from neighbouring servers (federated, admin-only) ──
var fedRequestsItems = [];

function setupFedRequests() {
    var btn = document.getElementById('fedRequestsRefreshBtn');
    if (btn) btn.addEventListener('click', loadFedRequests);
}

async function loadFedRequests() {
    var container = document.getElementById('fedRequestsTableContainer');
    if (!container) return;
    container.innerHTML = '<div class="loading">Загрузка...</div>';
    try {
        var resp = await api(API + '/fed/requests');
        if (!resp.ok) { container.innerHTML = '<div class="error">Ошибка загрузки</div>'; return; }
        var data = await resp.json();
        fedRequestsItems = data.items || [];
        renderFedRequests();
    } catch (e) {
        container.innerHTML = '<div class="error">Ошибка загрузки: ' + escapeHtml(e.message || e) + '</div>';
    }
}

function fedStatusLabel(status) {
    return { new: 'Новый', done: 'Обработан', hidden: 'Скрыт' }[status] || escapeHtml(status || '');
}

function renderFedRequests() {
    var container = document.getElementById('fedRequestsTableContainer');
    if (!container) return;
    if (!fedRequestsItems.length) {
        container.innerHTML = '<div class="note">Запросов от соседних серверов пока нет.</div>';
        return;
    }
    var html = '<table class="admin-table" id="table-fedrequests"><thead><tr>' +
        '<th>Дата</th><th>Сервер</th><th>Название</th><th>Автор</th><th>Предложено</th><th>Статус</th><th class="actions">Действия</th>' +
        '</tr></thead><tbody>';
    fedRequestsItems.forEach(function(it) {
        var offeredHtml = '';
        if (it.offered_title) {
            var dl = fedDeliveryLabel(it.delivery_status);
            offeredHtml = escapeHtml(it.offered_title) +
                (it.offered_authors ? '<div class="fed-offered-auth">' + escapeHtml(it.offered_authors) + '</div>' : '') +
                (dl ? '<div class="' + (it.delivery_status === 'delivered' ? 'sug-fed-ok' : 'sug-fed-fail') + '">' + escapeHtml(dl) + (it.delivered_at ? ' ' + escapeHtml(formatDateTime(it.delivered_at)) : '') + '</div>' : '');
        } else {
            offeredHtml = '<span style="color:#999">—</span>';
        }
        html += '<tr>' +
            '<td>' + escapeHtml(it.created_at || '') + '</td>' +
            '<td>' + escapeHtml(it.source_url || '') + '</td>' +
            '<td>' + escapeHtml(it.bookname || '') + '</td>' +
            '<td>' + escapeHtml(it.author || '') + '</td>' +
            '<td>' + offeredHtml + '</td>' +
            '<td><span class="badge-role">' + fedStatusLabel(it.status) + '</span></td>' +
            '<td class="actions">' +
                '<button class="btn btn-small fed-req-offer" data-id="' + it.id + '">Предложить книгу</button> ' +
                '<button class="btn btn-small fed-req-done" data-id="' + it.id + '" ' + (it.status === 'done' ? 'disabled' : '') + '>Обработан</button> ' +
                '<button class="btn btn-small fed-req-hide" data-id="' + it.id + '" ' + (it.status === 'hidden' ? 'disabled' : '') + '>Скрыть</button> ' +
                '<button class="btn btn-small btn-danger fed-req-del" data-id="' + it.id + '">Удалить</button>' +
            '</td></tr>';
    });
    html += '</tbody></table>';
    container.innerHTML = html;
}

function fedSetStatus(id, status) {
    api(API + '/fed/requests/' + id + '/status', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: status })
    }).then(function(r) {
        if (r.ok) loadFedRequests();
        else r.json().then(function(d) { alert(d.error || 'Ошибка'); });
    });
}

function fedDeleteReq(id) {
    api(API + '/fed/requests/' + id, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' }
    }).then(function(r) {
        if (r.ok) loadFedRequests();
        else r.json().then(function(d) { alert(d.error || 'Ошибка'); });
    });
}

document.addEventListener('click', function(e) {
    var t = e.target.closest('button');
    if (!t) return;
    if (t.classList.contains('fed-req-done')) fedSetStatus(parseInt(t.dataset.id), 'done');
    else if (t.classList.contains('fed-req-hide')) fedSetStatus(parseInt(t.dataset.id), 'hidden');
    else if (t.classList.contains('fed-req-del')) fedDeleteReq(parseInt(t.dataset.id));
    else if (t.classList.contains('fed-req-offer')) openFedOfferModal(parseInt(t.dataset.id));
});

// ─── Offer a book back to the requesting neighbour ─────────────

function fedDeliveryLabel(s) {
    return { sent: 'Отправлено', delivered: 'Доставлено', failed: 'Ошибка доставки' }[s] || '';
}

function openFedOfferModal(incomingID) {
    var item = (fedRequestsItems || []).find(function(i) { return i.id === incomingID; });
    var alreadyOffered = item && item.offered_title;
    var infoHtml = '';
    if (alreadyOffered) {
        var statusLabel = fedDeliveryLabel(item.delivery_status) || '';
        var dateStr = item.delivered_at ? formatDateTime(item.delivered_at) : '';
        infoHtml = '<div class="fed-offered-book">' + escapeHtml(item.offered_title || '') +
            '<span class="fed-offered-auth">' + escapeHtml(item.offered_authors || '') + '</span>' +
            '<span class="' + (item.delivery_status === 'delivered' ? 'sug-fed-ok' : 'sug-fed-fail') + '">' + escapeHtml(statusLabel) + (dateStr ? ' ' + escapeHtml(dateStr) : '') + '</span>' +
            '</div>';
        infoHtml += '<p style="font-size:12px;color:#666;margin:6px 0">Вы можете предложить другую книгу или отправить её повторно.</p>';
    }
    openAdminModal('Предложить книгу', `
        <p style="font-size:13px;color:#666">Книга будет отправлена серверу, приславшему этот запрос.</p>
        ` + infoHtml + `
        <div class="form-group"><label>Книга из библиотеки:</label>
            <div class="sug-book-search">
                <input type="hidden" class="sug-edition-id" id="fedOfferEdition">
                <input type="text" class="sug-bookname form-input" id="fedOfferBookname" autocomplete="off" placeholder="Начните вводить название...">
                <div class="search-results" style="display:none"></div>
            </div>
        </div>`);
    var group = document.querySelector('#adminModal .sug-book-search');
    if (group) attachGroupAutocomplete(group);
    document.getElementById('adminForm').onsubmit = function(e) {
        e.preventDefault();
        saveFedOffer(incomingID);
    };
}

async function saveFedOffer(incomingID) {
    var editionInput = document.getElementById('fedOfferEdition');
    var editionId = editionInput ? parseInt(editionInput.value) || 0 : 0;
    if (!editionId) { alert('Выберите книгу из библиотеки.'); return; }
    try {
        var res = await api(API + '/federation/offer', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ incoming_request_id: incomingID, edition_id: editionId })
        });
        var data = await res.json().catch(function() { return {}; });
        if (!res.ok) {
            alert(data.error || 'Ошибка отправки книги');
            return;
        }
        if (data && data.duplicate) {
            alert('Запрос выполнен: книга привязана к запросу.\n' +
                (data.title ? data.title + ' / ' + data.authors : ''));
        } else {
            alert('Запрос выполнен: книга отправлена серверу и привязана к запросу (edition ' + (data.edition_id || '') + ', work ' + (data.work_id || '') + ').');
        }
        closeAdminModal();
        loadFedRequests();
    } catch (err) {
        alert('Ошибка: ' + err.message);
    }
}
