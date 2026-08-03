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
    if (authUser.role === 'viewer') {
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
        if (tab.dataset.tab === 'readlists') { loadChildren(); loadReadlists(); }
    });
});

function escapeHtml(text) {
    if (!text) return '';
    var div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function openAdminModal(title, content) {
    document.getElementById('adminModalTitle').textContent = title;
    document.getElementById('adminModalBody').innerHTML = content;
    document.getElementById('adminModal').style.display = 'flex';
    document.getElementById('adminForm').onsubmit = null;
}

function closeAdminModal() {
    document.getElementById('adminModal').style.display = 'none';
}

document.getElementById('adminModal').addEventListener('click', function(e) {
    if (e.target === this) closeAdminModal();
});

document.addEventListener('click', function(e) {
    var modal = document.getElementById('editModal');
    if (e.target === modal) closeModal();
});

var storeUsers = [];
var storeAuthors = [];
var storeGenres = [];
var storeTags = [];
var sortState = {};
const PAGE_SIZE = 50;
var usersPage = 1;
var genresPage = 1;
var tagsPage = 1;

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
    var filterText = document.getElementById('filter-users').value;
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

document.getElementById('addUserBtn').addEventListener('click', function() {
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
    var filterText = document.getElementById('filter-authors').value;
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

document.getElementById('addAuthorBtn').addEventListener('click', function() {
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
    var filterText = document.getElementById('filter-genres').value;
    var filtered = filterData(storeGenres, filterText, ['name', 'parent_name', 'description']);
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

document.getElementById('addGenreBtn').addEventListener('click', function() {
    api(API + '/genres').then(r => r.json()).then(allGenres => {
        var options = '<option value="">Нет родителя</option>' +
            allGenres.map(g => '<option value="' + g.id + '">' + escapeHtml(g.name) + '</option>').join('');
        openAdminModal('Создать жанр', `
            <div class="form-group">
                <label>Название:</label>
                <input type="text" id="f_name" required>
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
            var pid = document.getElementById('f_parent_id').value;
            var desc = document.getElementById('f_description').value;
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
                if (pid) body.parent_id = parseInt(pid); else body.parent_id = null;
                if (desc) body.description = desc; else body.description = null;
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
    var filterText = document.getElementById('filter-tags').value;
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

document.getElementById('addTagBtn').addEventListener('click', function() {
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
                sugg_hidden: true
            };
        }
        var m = merged[rid];
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

        html += '<div class="suggestion-card">' +
            '<div class="sug-field"><span class="sug-label">Книга:</span> ' + escapeHtml(item.bookname || '') + '</div>' +
            '<div class="sug-field"><span class="sug-label">Автор:</span> ' + escapeHtml(item.author || '') + '</div>' +
            '<div class="sug-field"><span class="sug-label">Пользователь:</span> ' + escapeHtml(item.username || '') + '</div>' +
            '<div class="sug-field"><span class="sug-label">Ищет:</span> ' + escapeHtml(item.looking_for || '') + '</div>' +
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

async function openSuggestModal(readListId) {
    document.getElementById('suggestModalTitle').textContent = 'Предложить книгу';

    // Load existing suggestions only (books are fetched on-demand)
    var existingSuggestions = [];
    try {
        var sugRes = await api(API + '/suggestions/readlist/' + encodeURIComponent(readListId)).catch(function() { return {ok: false}; });
        if (sugRes.ok) existingSuggestions = await sugRes.json();
    } catch(e) {}

    var html = '<div class="form-group"><label>ID записи (не редактируется):</label>' +
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
        '<select class="sug-book-select form-input" size="5" style="margin-top:5px;display:none">' +
        '<option value="">— введите 1+ символ —</option>' +
        '</select>' +
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
        if (e.target.classList.contains('sug-book-select') && e.target.value) {
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
    var select = group.querySelector('.sug-book-select');
    var editionInput = group.querySelector('.sug-edition-id');
    if (!input || !select) return;

    var fetchTimer = null;

    input.addEventListener('input', function() {
        var val = this.value;
        editionInput.value = '';

        if (val.length < 1) {
            select.innerHTML = '<option value="">— введите 1+ символ —</option>';
            select.style.display = 'none';
            return;
        }

        if (fetchTimer) clearTimeout(fetchTimer);
        fetchTimer = setTimeout(function() {
            fetch('/api/v1/books?book=' + encodeURIComponent(val) + '&limit=10', {
                headers: {'Authorization': 'Bearer ' + authToken}
            }).then(function(res) {
                if (!res.ok) return null;
                return res.json();
            }).then(function(data) {
                if (!data || !data.books) {
                    select.innerHTML = '<option value="">— ничего не найдено —</option>';
                    select.style.display = 'none';
                    return;
                }
                var opts = '<option value="">— выберите книгу —</option>';
                var seen = {};
                for (var i = 0; i < data.books.length; i++) {
                    var b = data.books[i];
                    var title = (b.edition_title || b.original_title || '').trim();
                    if (!title || seen[title]) continue;
                    seen[title] = true;
                    var author = '';
                    if (typeof b.authors === 'string') author = b.authors;
                    else if (b.authors && typeof b.authors.String === 'string') author = b.authors.String;
                    var label = title + (author ? ' (' + author + ')' : '');
                    opts += '<option value="' + b.edition_id + '" data-title="' + escapeAttr(title) + '" data-firstauthor="' + escapeAttr(author) + '">' + escapeHtml(label) + '</option>';
                }
                select.innerHTML = opts;
                select.style.display = select.options.length > 1 ? '' : 'none';
            }).catch(function() {});
        }, 300);
    });

    input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
            var val = this.value;
            if (val.length < 1) return;
            var opts = select.options;
            var matched = [];
            for (var i = 0; i < opts.length; i++) {
                if (opts[i].value === '') continue;
                if (opts[i].textContent.toLowerCase().indexOf(val.toLowerCase()) !== -1) {
                    matched.push(opts[i]);
                }
            }
            if (matched.length === 1) {
                e.preventDefault();
                fillSuggSelection(input, select, editionInput, matched[0]);
            }
        }
    });

    input.addEventListener('blur', function() {
        var val = this.value;
        if (val.length < 1) return;
        var opts = select.options;
        var matched = [];
        for (var i = 0; i < opts.length; i++) {
            if (opts[i].value === '') continue;
            if (opts[i].textContent.toLowerCase().indexOf(val.toLowerCase()) !== -1) {
                matched.push(opts[i]);
            }
        }
        if (matched.length === 1) {
            fillSuggSelection(input, select, editionInput, matched[0]);
        }
    });

    select.addEventListener('change', function() {
        if (this.value) {
            fillSuggSelection(input, select, editionInput, this.options[this.selectedIndex]);
        } else {
            editionInput.value = '';
        }
    });
}

function attachSuggestionAutocomplete() {
    document.querySelectorAll('.sug-book-search').forEach(function(group) {
        attachGroupAutocomplete(group);
    });
}

function fillSuggSelection(input, select, editionInput, opt) {
    opt.selected = true;
    editionInput.value = opt.value;
    input.value = opt.dataset.title || opt.textContent;
    select.style.display = 'none';
    // Trigger change to detect slot-fill for auto-add
    var evt = new Event('change', {bubbles: true});
    select.dispatchEvent(evt);
}

function escapeAttr(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
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

async function hideSuggestion(readListId) {
    try {
        var resp = await api(API + '/suggestions/readlist/' + encodeURIComponent(readListId));
        if (!resp.ok) { alert('Ошибка загрузки предложений'); return; }
        var existing = await resp.json();
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
        var existing = await resp.json();
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

function setupReadlistsFilters() {
    ['filter-rl-listname', 'filter-rl-bookname', 'filter-rl-author'].forEach(function(id) {
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
    var filterBtn = document.getElementById('rlChildrenFilterBtn');
    if (filterBtn) filterBtn.addEventListener('click', toggleChildrenDropdown);
    var allBtn = document.getElementById('rlChildrenAllBtn');
    if (allBtn) allBtn.addEventListener('click', function() {
        selectedChildren = storeChildren.map(function(c) { return c.id; });
        renderChildrenFilter();
        readlistsPage = 1;
        loadReadlists();
    });
    var clearBtn = document.getElementById('rlChildrenClearBtn');
    if (clearBtn) clearBtn.addEventListener('click', function() {
        selectedChildren = [];
        renderChildrenFilter();
        readlistsPage = 1;
        loadReadlists();
    });
    document.addEventListener('click', function(e) {
        var wrap = document.getElementById('rlChildrenFilterWrap');
        if (wrap && !wrap.contains(e.target)) {
            var dd = document.getElementById('rlChildrenDropdown');
            if (dd) dd.style.display = 'none';
        }
    });
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

async function loadReadlists() {
    var body = document.getElementById('readlistsTableBody');
    if (body) body.innerHTML = '<tr><td colspan="8" class="loading">Загрузка...</td></tr>';

    var listname = document.getElementById('filter-rl-listname') ? document.getElementById('filter-rl-listname').value : '';
    var bookname = document.getElementById('filter-rl-bookname') ? document.getElementById('filter-rl-bookname').value : '';
    var author = document.getElementById('filter-rl-author') ? document.getElementById('filter-rl-author').value : '';

    var params = new URLSearchParams();
    if (selectedChildren.length > 0) params.set('user_ids', selectedChildren.join(','));
    if (listname) params.set('listname', listname);
    if (bookname) params.set('bookname', bookname);
    if (author) params.set('author', author);
    params.set('limit', PAGE_SIZE);
    params.set('offset', (readlistsPage - 1) * PAGE_SIZE);

    try {
        var res = await api(API + '/readlists?' + params.toString());
        if (!res.ok) {
            if (body) body.innerHTML = '<tr><td colspan="8" class="error">Ошибка загрузки</td></tr>';
            return;
        }
        var data = await res.json();
        storeReadlists = data.items || [];
        readlistsTotal = data.total || 0;
        renderReadlists();
    } catch(e) {
        if (body) body.innerHTML = '<tr><td colspan="8" class="error">Ошибка: ' + escapeHtml(e.message) + '</td></tr>';
    }
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
            return readlistsSortDir === 'asc' ? (va ? 1 : 0) - (vb ? 1 : 0) : (vb ? 1 : 0) - (va ? 1 : 0);
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
        body.innerHTML = '<tr><td colspan="8" class="no-results">Нет данных</td></tr>';
    } else {
        body.innerHTML = sorted.map(function(item) {
            var shelf = item.on_shelf ? '✓' : '';
            var status = item.status ? escapeHtml(item.status) : '';
            return '<tr>' +
                '<td>' + escapeHtml(item.username || '') + '</td>' +
                '<td>' + escapeHtml(item.listname || '') + '</td>' +
                '<td>' + escapeHtml(item.author || '') + '</td>' +
                '<td>' + escapeHtml(item.bookname || '') + '</td>' +
                '<td>' + status + '</td>' +
                '<td>' + escapeHtml(formatDateTime(item.created_at)) + '</td>' +
                '<td class="center">' + shelf + '</td>' +
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
    openAdminModal('Создать список', `
        <div class="form-group">
            <label>Дети:</label>
            <div id="f_rl_children_picker" class="user-picker"></div>
            <button type="button" class="btn btn-secondary add-child-picker-row" data-container="f_rl_children_picker">+ Добавить ребёнка</button>
        </div>
        <div class="form-group">
            <label>Название списка:</label>
            <input type="text" id="f_rl_new_listname" required>
        </div>
        <div class="form-group">
            <label>Автор:</label>
            <input type="text" id="f_rl_new_author">
        </div>
        <div class="form-group">
            <label>Книга:</label>
            <input type="text" id="f_rl_new_bookname">
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
    document.getElementById('adminForm').onsubmit = async function(e) {
        e.preventDefault();
        var user_ids = collectChildPick('f_rl_children_picker');
        if (!user_ids.length) { alert('Выберите хотя бы одного ребёнка'); return; }
        var body = {
            user_ids: user_ids,
            listname: document.getElementById('f_rl_new_listname').value,
            author: document.getElementById('f_rl_new_author').value,
            bookname: document.getElementById('f_rl_new_bookname').value,
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
            <label>Автор:</label>
            <input type="text" id="f_rl_author" value="${escapeHtml(item.author || '')}">
        </div>
        <div class="form-group">
            <label>Книга:</label>
            <input type="text" id="f_rl_bookname" value="${escapeHtml(item.bookname || '')}">
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
    document.getElementById('adminForm').onsubmit = async function(e) {
        e.preventDefault();
        var body = {
            listname: document.getElementById('f_rl_listname').value,
            author: document.getElementById('f_rl_author').value,
            bookname: document.getElementById('f_rl_bookname').value,
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

document.addEventListener('DOMContentLoaded', async function() {
    if (!await checkAdminAccess()) return;

    if (currentRole !== 'admin') {
        var usersTab = document.querySelector('.admin-tab[data-tab="users"]');
        if (usersTab) usersTab.remove();
        var usersContent = document.getElementById('tab-users');
        if (usersContent) usersContent.remove();
        var firstTab = document.querySelector('.admin-tab');
        if (firstTab) { firstTab.classList.add('active'); document.getElementById('tab-' + firstTab.dataset.tab).classList.add('active'); }
        loadAuthors();
        loadBooks();
        loadGenres();
        loadTags();
        setupSuggestionsFilters();
        setupReadlistsFilters();
        setupReadlistsSorting();
        return;
    }

    setupSorting('table-users');
    setupSorting('table-authors');
    setupSorting('table-genres');
    setupSorting('table-tags');
    loadUsers();
    loadAuthors();
    loadGenres();
    loadTags();
    setupSuggestionsFilters();
    setupReadlistsFilters();
    setupReadlistsSorting();
    document.getElementById('adminBackLink').addEventListener('click', function(e) {
        e.preventDefault();
        if (window.history.length > 1) {
            window.history.back();
        } else if (document.referrer) {
            window.location.href = document.referrer;
        } else {
            window.location.href = '/';
        }
    });
});
