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
        if (tab.dataset.tab === 'settings') { loadSettings(); }
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

function loadUsers() {
    api(API + '/users').then(r => r.json()).then(users => {
        storeUsers = users;
        applyFilters();
    });
}

function renderUsers() {
    var tbody = document.getElementById('usersTableBody');
    var filterText = document.getElementById('filter-users').value;
    var filtered = filterData(storeUsers, filterText, ['username', 'email', 'role']);
    var sorted = sortData(filtered, getSortKey('table-users'), getSortDir('table-users'));
    renderTable(tbody, sorted, function(u) {
        return '<tr>' +
            '<td>' + u.id + '</td>' +
            '<td>' + escapeHtml(u.username) + '</td>' +
            '<td>' + escapeHtml(u.email || '') + '</td>' +
            '<td><span class="badge-role ' + (u.role || '') + '">' + escapeHtml(u.role || '') + '</span></td>' +
            '<td>' + (u.created_at || '') + '</td>' +
            '<td class="actions">' +
            '<button class="btn btn-small edit-user" data-id="' + u.id + '">✎</button> ' +
            '<button class="btn btn-small btn-secondary delete-user" data-id="' + u.id + '">Удалить</button>' +
            '</td></tr>';
    });
}

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
            <button id="clearAdminBookFilters" class="btn btn-secondary btn-clear" title="Очистить фильтры">🗑</button>
        </div>
        `);
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
    `);
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
    api(API + '/persons').then(r => r.json()).then(persons => {
        storeAuthors = persons;
        applyFilters();
    });
}

function renderAuthors() {
    var tbody = document.getElementById('authorsTableBody');
    var filterText = document.getElementById('filter-authors').value;
    var filtered = filterData(storeAuthors, filterText, ['first_name', 'last_name', 'middle_name', 'pseudonym']);
    var sorted = sortData(filtered, getSortKey('table-authors'), getSortDir('table-authors'));
    renderTable(tbody, sorted, function(p) {
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
    api(API + '/genres').then(r => r.json()).then(genres => {
        storeGenres = genres;
        applyFilters();
    });
}

function renderGenres() {
    var tbody = document.getElementById('genresTableBody');
    var filterText = document.getElementById('filter-genres').value;
    var filtered = filterData(storeGenres, filterText, ['name', 'parent_name', 'description']);
    var sorted = sortData(filtered, getSortKey('table-genres'), getSortDir('table-genres'));
    renderTable(tbody, sorted, function(g) {
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
    var filterText = document.getElementById('filter-tags').value;
    var filtered = filterData(storeTags, filterText, ['name', 'description', 'color']);
    var sorted = sortData(filtered, getSortKey('table-tags'), getSortDir('table-tags'));
    renderTable(tbody, sorted, function(t) {
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

setupFilterInput('filter-users', applyFilters);
setupFilterInput('filter-authors', applyFilters);
setupFilterInput('filter-genres', applyFilters);
setupFilterInput('filter-tags', applyFilters);

setupFilterInput('bookAuthorFilter', function() { booksPage = 1; loadBooks(); });
setupFilterInput('bookTitleFilter', function() { booksPage = 1; loadBooks(); });
setupFilterInput('bookGenreFilter', function() { booksPage = 1; loadBooks(); });
setupFilterInput('bookDateFrom', function() { booksPage = 1; loadBooks(); });
setupFilterInput('bookDateTo', function() { booksPage = 1; loadBooks(); });
setupFilterInput('bookStatusFilter', function() { booksPage = 1; loadBooks(); });

document.getElementById('clearAdminBookFilters')?.addEventListener('click', function() {
    document.getElementById('bookAuthorFilter').value = '';
    document.getElementById('bookTitleFilter').value = '';
    document.getElementById('bookGenreFilter').value = '';
    var df = document.getElementById('bookDateFrom');
    var dt = document.getElementById('bookDateTo');
    if (df) df.value = '';
    if (dt) dt.value = '';
    booksPage = 1;
    loadBooks();
});

// Event delegation for admin table action buttons
document.addEventListener('click', function(e) {
    var target = e.target.closest('button');
    if (!target) return;

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
    }
});

document.getElementById('settingsForm').addEventListener('submit', async function(e) {
    e.preventDefault();
    var statusEl = document.getElementById('settingsSaveStatus');
    statusEl.textContent = 'Сохранение...';
    try {
        var res = await api(API + '/settings', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ backup_dir: document.getElementById('settingsBackupDir').value.trim() })
        });
        if (res.ok) {
            statusEl.textContent = '✓ Сохранено';
            setTimeout(function() { statusEl.textContent = ''; }, 3000);
        } else {
            var err = await res.json();
            statusEl.textContent = 'Ошибка: ' + (err.error || 'Неизвестная ошибка');
        }
    } catch(e) {
        statusEl.textContent = 'Ошибка сети: ' + e.message;
    }
});

async function loadSettings() {
    var statusEl = document.getElementById('settingsSaveStatus');
    statusEl.textContent = 'Загрузка...';
    try {
        var res = await api(API + '/settings');
        if (!res.ok) { statusEl.textContent = 'Ошибка загрузки'; return; }
        var data = await res.json();
        document.getElementById('settingsBackupDir').value = data.backup_dir || '';
        statusEl.textContent = '';
    } catch(e) {
        statusEl.textContent = 'Ошибка сети: ' + e.message;
    }
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
