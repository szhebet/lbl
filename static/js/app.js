const API_BASE = '/api/v1';
let enableDelete = false;

function triggerDownload(url) {
    window.location.href = url;
}
let authorsPage = 1;
let booksPage = 1;
let booksSortBy = 'original_title';
let booksSortOrder = 'asc';
let userBookStatuses = {};
let readlistPage = 1;
let readlistSortBy = 'priority';
let readlistSortOrder = 'desc';
var personMap = {};

async function apiFetch(path, options) {
    const token = localStorage.getItem('auth_token');
    if (!options) options = {};
    if (!options.headers) options.headers = {};
    if (token) options.headers['Authorization'] = 'Bearer ' + token;
    var response = await fetch(path, options);
    if (response.status === 401 && path !== '/api/v1/auth/login' && path !== '/api/v1/auth/refresh') {
        var refreshed = typeof tryRefreshToken === 'function' && await tryRefreshToken();
        if (refreshed) {
            var newToken = localStorage.getItem('auth_token');
            options.headers['Authorization'] = 'Bearer ' + newToken;
            return fetch(path, options);
        }
        if (typeof handleAuthFailure === 'function') {
            handleAuthFailure();
        }
    }
    return response;
}

function promptLogin() {
    if (typeof openLoginModal === 'function') {
        openLoginModal();
    } else {
        alert('Необходимо авторизоваться');
    }
}

function handleAuthFailure() {
    if (typeof authToken !== 'undefined') { authToken = ''; }
    if (typeof authUser !== 'undefined') { authUser = null; }
    localStorage.removeItem('auth_token');
    localStorage.removeItem('auth_user');
    userBookStatuses = {};
    var btn = document.getElementById('loginBtn');
    if (btn) {
        btn.textContent = 'Авторизоваться';
        btn.classList.remove('logged-in');
    }
    promptLogin();
}

// Load user's book statuses if logged in
async function loadUserBookStatuses() {
    var token = localStorage.getItem('auth_token');
    if (!token) { userBookStatuses = {}; return; }
    try {
        var res = await apiFetch(API_BASE + '/user/books', {
            headers: { 'Authorization': 'Bearer ' + token }
        });
        if (res.ok) {
            var list = await res.json();
            userBookStatuses = {};
            list.forEach(function(ub) { userBookStatuses[ub.edition_id] = ub; });
        } else if (res.status === 401) {
            handleAuthFailure();
        }
    } catch(e) {}
}

function refreshCurrentView() {
    var authorsTab = document.getElementById('tab-authors') || document.getElementById('authors');
    var booksTab = document.getElementById('tab-books') || document.getElementById('books');
    var activeAuthors = authorsTab && (authorsTab.classList.contains('active') || document.getElementById('tab-authors')?.classList.contains('active'));
    var activeBooks = booksTab && (booksTab.classList.contains('active') || document.getElementById('tab-books')?.classList.contains('active'));
    var mainAuthors = document.getElementById('authors')?.classList.contains('active');
    var mainBooks = document.getElementById('books')?.classList.contains('active');
    var mainReadlist = document.getElementById('readlist')?.classList.contains('active');
    if (activeAuthors || mainAuthors) loadAuthors();
    if (activeBooks || mainBooks) loadBooks();
    if (mainReadlist) loadReadlist();
}

function getUserBookStatus(editionId) {
    return userBookStatuses[editionId] || { status: 'Не заполнено' };
}

function getStatusClass(status) {
    var m = { 'Прочитано': 'status-done', 'Читаю': 'status-reading', 'Отложил': 'status-paused', 'Бросил': 'status-abandoned' };
    return m[status] || '';
}

async function setUserBookStatus(editionId, status) {
    var token = localStorage.getItem('auth_token');
    if (!token) { promptLogin(); return; }
    try {
        var res = await apiFetch(API_BASE + '/user/books/' + editionId, {
            method: 'PUT',
            headers: { 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
            body: JSON.stringify({ status: status })
        });
        if (res.ok) {
            var ub = await res.json();
            userBookStatuses[editionId] = ub;
            refreshCurrentView();
        } else if (res.status === 401) {
            handleAuthFailure();
        }
    } catch(e) {}
}

// Stubs - replaced by real functions from import.js when it loads
var showImportProgress = function(dirPath, total) {};
var startImportPolling = function() {};
var switchToImportTab = function() {};

document.getElementById('loadBooksBtn')?.addEventListener('click', () => {
    document.getElementById('folderInput').click();
});

document.getElementById('showImportProgressBtn')?.addEventListener('click', () => {
    switchToImportTab();
    checkImportStatus();
});

document.getElementById('folderInput')?.addEventListener('change', async (e) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;

    const bookFiles = Array.from(files).filter(f =>
        f.name.toLowerCase().endsWith('.fb2') ||
        f.name.toLowerCase().endsWith('.fb2.zip') ||
        f.name.toLowerCase().endsWith('.epub') ||
        f.name.toLowerCase().endsWith('.pdf') ||
        f.name.toLowerCase().endsWith('.pdf.zip') ||
        f.name.toLowerCase().endsWith('.doc') ||
        f.name.toLowerCase().endsWith('.doc.zip') ||
        f.name.toLowerCase().endsWith('.docx') ||
        f.name.toLowerCase().endsWith('.docx.zip') ||
        f.name.toLowerCase().endsWith('.zip')
    );

    if (bookFiles.length === 0) {
        alert('В выбранной папке не найдены файлы поддерживаемых форматов (FB2, EPUB, PDF, DOC, DOCX)');
        return;
    }

    e.target.value = '';

    showSimpleProgress('Загрузка файлов на сервер... 0 / ' + bookFiles.length);

    try {
        const formData = new FormData();
        for (const file of bookFiles) {
            formData.append('files', file);
        }

        const response = await apiFetch(API_BASE + '/import/upload', {
            method: 'POST',
            body: formData
        });

        const data = await response.json();

        if (response.ok && data.started) {
            removeSimpleProgress();
            showImportProgress('', data.total);
            startImportPolling();
        } else {
            removeSimpleProgress();
            alert('Ошибка: ' + (data.error || 'Не удалось запустить импорт'));
        }
    } catch (err) {
        removeSimpleProgress();
        alert('Ошибка загрузки: ' + err.message);
    }
});

function showSimpleProgress(text) {
    var el = document.createElement('div');
    el.id = 'simpleProgress';
    el.className = 'import-status';
    el.innerHTML = '<div class="loading">' + escapeHtml(text) + '</div>';
    document.body.appendChild(el);
}

function removeSimpleProgress() {
    var el = document.getElementById('simpleProgress');
    if (el) el.remove();
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function getFieldValue(value) {
    if (!value) return '';
    if (typeof value === 'object') {
        if (value.Valid !== undefined) {
            return value.Valid ? String(value.String || '').trim() : '';
        }
        return '';
    }
    return String(value).trim();
}

function getNullableValue(obj, field) {
    const value = obj && obj[field];
    return getFieldValue(value);
}

function isEmptyOrWhitespace(str) {
    if (!str || typeof str !== 'string') return true;
    return str.trim() === '';
}

document.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
        document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
        tab.classList.add('active');
        document.getElementById(tab.dataset.tab).classList.add('active');
        if (tab.dataset.tab === 'authors') {
            loadAuthors();
        } else if (tab.dataset.tab === 'books') {
            loadBooks();
        } else if (tab.dataset.tab === 'genres') {
            loadGenres();
        } else if (tab.dataset.tab === 'readlist') {
            loadReadlist();
            loadReadlistNames();
        }
    });
});

document.getElementById('clearFilters')?.addEventListener('click', () => {
    document.getElementById('authorFilter').value = '';
    document.getElementById('bookFilter').value = '';
    document.getElementById('genreFilter').value = '';
    authorsPage = 1;
    loadAuthors();
});

document.getElementById('addToShelfBtn')?.addEventListener('click', async () => {
    const bookIds = [];
    
    document.querySelectorAll('.level-1:not(.collapsed) + .level-2-container .level-2 .edit-btn').forEach(btn => {
        bookIds.push(btn.dataset.id);
    });
    
    if (bookIds.length === 0) {
        alert('Разверните авторов для добавления книг на полку');
        return;
    }
    
    let added = 0;
    let errors = 0;
    
    for (const id of bookIds) {
        try {
            const response = await apiFetch(`${API_BASE}/books/${id}/shelf`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ on_shelf: true })
            });
            if (response.ok) {
                added++;
            } else {
                errors++;
            }
        } catch (err) {
            errors++;
        }
    }
    
    alert(`Добавлено на полку: ${added}, ошибок: ${errors}`);
    updateShelfCount();
});

document.getElementById('clearShelfBtn')?.addEventListener('click', async () => {
    if (!confirm('Удалить все книги с полки?')) return;
    
    try {
        const response = await apiFetch(`${API_BASE}/shelf/clear`, { method: 'PUT' });
        if (response.ok) {
            alert('Полка очищена');
            updateShelfCount();
        } else {
            alert('Ошибка при очистке полки');
        }
    } catch (err) {
        alert('Ошибка: ' + err.message);
    }
});

document.getElementById('clearShelfBooksBtn')?.addEventListener('click', async () => {
    if (!confirm('Удалить все книги с полки?')) return;
    try {
        const response = await apiFetch(`${API_BASE}/shelf/clear`, { method: 'PUT' });
        if (response.ok) {
            alert('Полка очищена');
            updateShelfCount();
        } else {
            alert('Ошибка при очистке полки');
        }
    } catch (err) {
        alert('Ошибка: ' + err.message);
    }
});

['authorFilter', 'bookFilter', 'genreFilter'].forEach(id => {
    const input = document.getElementById(id);
    if (!input) return;
    input.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            loadAuthors();
        }
    });
    input.addEventListener('blur', () => { authorsPage = 1; loadAuthors(); });
});

document.getElementById('clearBookFilters')?.addEventListener('click', () => {
    document.getElementById('bookTitleFilter').value = '';
    document.getElementById('bookAuthorFilter').value = '';
    document.getElementById('bookGenreFilter').value = '';
    const df = document.getElementById('bookDateFrom');
    const dt = document.getElementById('bookDateTo');
    if (df) df.value = '';
    if (dt) dt.value = '';
    booksPage = 1;
    loadBooks();
});

['bookTitleFilter', 'bookAuthorFilter', 'bookGenreFilter', 'bookDateFrom', 'bookDateTo'].forEach(id => {
    const input = document.getElementById(id);
    if (input) {
        input.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                booksPage = 1;
                loadBooks();
            }
        });
        input.addEventListener('blur', () => { booksPage = 1; loadBooks(); });
    }
});

document.getElementById('clearGenreFilters')?.addEventListener('click', () => {
    document.getElementById('genreNameFilter').value = '';
    document.getElementById('genreAuthorFilter').value = '';
    document.getElementById('genreBookFilter').value = '';
    loadGenres();
});

['genreNameFilter', 'genreAuthorFilter', 'genreBookFilter'].forEach(id => {
    const input = document.getElementById(id);
    if (input) {
        input.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                loadGenres();
            }
        });
        input.addEventListener('blur', () => { loadGenres(); });
    }
});

document.getElementById('editForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();

    const editType = document.getElementById('modalTitle').dataset.type;
    const id = document.getElementById('modalTitle').dataset.id;

    try {
        if (editType === 'author') {
            const formData = new FormData(e.target);
            const data = Object.fromEntries(formData.entries());
            data.first_name = data.first_name ? data.first_name.trim() : '';
            data.last_name = data.last_name ? data.last_name.trim() : '';

            if (!data.last_name) {
                alert('Фамилия обязательна');
                return;
            }

            const response = await apiFetch(`${API_BASE}/persons/${id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Ошибка сохранения');
            }

            closeModal();
            const state = saveExpandedState();
            state.keepFocus = { type: 'author', id: id };
            loadAuthorsWithState(state);
        } else if (editType === 'book') {
            const data = collectExtendedBookData();
            const response = await apiFetch(`${API_BASE}/books/${id}/extended`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Ошибка сохранения');
            }

            closeModal();
            if (document.getElementById('authorsTree')) {
                const state = saveExpandedState();
                state.keepFocus = { type: 'book', id: id };
                loadAuthorsWithState(state);
            } else {
                loadBooks();
            }
        } else if (editType === 'genre') {
            const name = document.getElementById('genre_name').value.trim();
            if (!name) {
                alert('Название жанра обязательно');
                return;
            }
            const parentSelect = document.getElementById('genre_parent');
            const parentId = parentSelect ? parentSelect.value : '';
            const body = { name: name };
            if (parentId) body.parent_id = parseInt(parentId);
            const response = await apiFetch(`${API_BASE}/genres/${id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });
            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Ошибка сохранения');
            }
            closeModal();
            loadGenres();
        }
    } catch (error) {
        alert(error.message);
    }
});

function collectExtendedBookData() {
    const work = {};

    const originalTitle = document.getElementById('original_title')?.value?.trim();
    if (originalTitle) work.original_title = originalTitle;

    const workLanguage = document.getElementById('work_language')?.value?.trim();
    if (workLanguage) work.original_language = workLanguage;

    const firstPublished = document.getElementById('first_published')?.value;
    if (firstPublished && !isEmptyOrWhitespace(firstPublished)) {
        const parsed = parseInt(firstPublished);
        if (!isNaN(parsed) && parsed !== 0) work.first_published = parsed;
    }

    const workType = document.getElementById('work_type')?.value?.trim();
    if (workType) work.work_type = workType;

    const workAnnotation = document.getElementById('work_annotation')?.value?.trim();
    if (!isEmptyOrWhitespace(workAnnotation)) work.annotation = workAnnotation;

    const edition = {};

    const editionTitle = document.getElementById('edition_title')?.value?.trim();
    if (editionTitle) edition.title = editionTitle;

    const editionLanguage = document.getElementById('edition_language')?.value?.trim();
    if (editionLanguage) edition.language = editionLanguage;

    const isbn = document.getElementById('isbn')?.value?.trim();
    if (!isEmptyOrWhitespace(isbn)) edition.isbn = isbn;

    const ean = document.getElementById('ean')?.value?.trim();
    if (!isEmptyOrWhitespace(ean)) edition.ean = ean;

    const udc = document.getElementById('udc')?.value?.trim();
    if (!isEmptyOrWhitespace(udc)) edition.udc = udc;

    const bbk = document.getElementById('bbk')?.value?.trim();
    if (!isEmptyOrWhitespace(bbk)) edition.bbk = bbk;

    const publisher = document.getElementById('publisher')?.value?.trim();
    if (!isEmptyOrWhitespace(publisher)) edition.publisher = publisher;

    const year = document.getElementById('year')?.value;
    if (year && !isEmptyOrWhitespace(year)) {
        const parsed = parseInt(year);
        if (!isNaN(parsed) && parsed !== 0) edition.year = parsed;
    }

    const city = document.getElementById('city')?.value?.trim();
    if (!isEmptyOrWhitespace(city)) edition.city = city;

    const pages = document.getElementById('pages')?.value;
    if (pages && !isEmptyOrWhitespace(pages)) {
        const parsed = parseInt(pages);
        if (!isNaN(parsed) && parsed !== 0) edition.pages = parsed;
    }

    const series = document.getElementById('series')?.value?.trim();
    if (!isEmptyOrWhitespace(series)) edition.series = series;

    const seriesNumber = document.getElementById('series_number')?.value?.trim();
    if (!isEmptyOrWhitespace(seriesNumber)) edition.series_number = seriesNumber;

    const editionAnnotation = document.getElementById('edition_annotation')?.value?.trim();
    if (!isEmptyOrWhitespace(editionAnnotation)) edition.annotation = editionAnnotation;

    const source = document.getElementById('source')?.value?.trim();
    if (!isEmptyOrWhitespace(source)) edition.source = source;

    const quality = document.getElementById('quality')?.value?.trim();
    if (quality) edition.quality = quality;

    const uploadDate = document.getElementById('upload_date')?.value;
    if (uploadDate) edition.upload_date = uploadDate;

    const isComplete = document.getElementById('is_complete')?.checked;
    edition.is_complete = isComplete;

    const authors = [];
    document.querySelectorAll('.author-row').forEach(row => {
        if (row.dataset.new === 'true') {
            const firstName = (row.dataset.firstName || '').trim();
            const lastName = (row.dataset.lastName || '').trim();
            if (lastName) {
                authors.push({
                    first_name: firstName,
                    last_name: lastName,
                    role: row.dataset.role || 'author'
                });
            }
        } else {
            const hiddenId = row.querySelector('.author-id');
            const roleSelect = row.querySelector('.author-role');
            if (hiddenId && hiddenId.value) {
                authors.push({
                    id: parseInt(hiddenId.value),
                    role: roleSelect ? roleSelect.value : 'author'
                });
            }
        }
    });

    const genres = [];
    const genresSelect = document.getElementById('genres_select');
    if (genresSelect) {
        Array.from(genresSelect.selectedOptions).forEach(opt => {
            if (opt.value) genres.push(parseInt(opt.value));
        });
    }

    const tags = [];
    const tagsSelect = document.getElementById('tags_select');
    if (tagsSelect) {
        Array.from(tagsSelect.selectedOptions).forEach(opt => {
            if (opt.value) tags.push(parseInt(opt.value));
        });
    }

    return {
        work: work,
        edition: edition,
        authors: authors,
        genres: genres,
        tags: tags
    };
}

document.addEventListener('DOMContentLoaded', () => {
    if (document.body.classList.contains('android')) {
        var readlistTab = document.querySelector('.tab[data-tab="readlist"]');
        if (readlistTab) {
            document.querySelectorAll('.tab').forEach(function(t) { t.classList.remove('active'); });
            document.querySelectorAll('.tab-content').forEach(function(c) { c.classList.remove('active'); });
            readlistTab.classList.add('active');
            document.getElementById('readlist').classList.add('active');
            loadReadlist();
            loadReadlistNames();
            updateOfflineUI();
        }
    }
    // Auto-sync if online with pending items (legacy — now handled by offline.js)
    if (isAndroid() && window.SyncService && window.ReadListStore && isOnline() && ReadListStore.countDirty() > 0) {
        SyncService.sync(true);
    }
    loadAuthors();
    updateShelfCount();
    fetchConfig();
    setupBooksTableEvents();
    setupReadlistEvents();
    loadUserBookStatuses();
});

async function fetchConfig() {
    enableDelete = false;
}

async function updateShelfCount() {
    try {
        const response = await apiFetch(`${API_BASE}/shelf/count`);
        const data = await response.json();
        document.querySelectorAll('.shelf-count').forEach(el => {
            el.textContent = `(${data.count})`;
        });
    } catch (err) {
        console.error('Error loading shelf count:', err);
    }
}

function saveExpandedState() {
    const state = { authors: new Set(), books: new Set() };
    document.querySelectorAll('.level-1:not(.collapsed)').forEach(el => {
        const btn = el.querySelector('.edit-btn');
        if (btn) state.authors.add(btn.dataset.id);
    });
    document.querySelectorAll('.level-2:not(.collapsed)').forEach(el => {
        const btn = el.querySelector('.edit-btn');
        if (btn) state.books.add(btn.dataset.id);
    });
    return state;
}

function restoreExpandedState(state) {
    state.authors.forEach(id => {
        const el = document.querySelector(`.level-1 .edit-btn[data-id="${id}"]`);
        if (el) {
            const level1 = el.closest('.level-1');
            level1.classList.remove('collapsed');
            level1.querySelector('.expand-icon').textContent = '▼';
            const container = level1.nextElementSibling;
            if (container) container.style.display = 'block';
        }
    });
    state.books.forEach(id => {
        const el = document.querySelector(`.level-2 .edit-btn[data-id="${id}"]`);
        if (el) {
            const level2 = el.closest('.level-2');
            level2.classList.remove('collapsed');
            level2.querySelector('.expand-icon').textContent = '▼';
            const container = level2.nextElementSibling;
            if (container) container.style.display = 'block';
        }
    });
}

async function loadAuthors() {
    const treeContainer = document.getElementById('authorsTree');
    const expandedState = saveExpandedState();
    treeContainer.innerHTML = '<div class="loading">Загрузка...</div>';

    let authorFilter = document.getElementById('authorFilter').value.trim().replace(/ё/g, 'е');
    let bookFilter = document.getElementById('bookFilter').value.trim().replace(/ё/g, 'е');
    let genreFilter = document.getElementById('genreFilter').value.trim();
    const page = authorsPage;

    try {
        const response = await apiFetch(`${API_BASE}/authors?author=${encodeURIComponent(authorFilter)}&book=${encodeURIComponent(bookFilter)}&genre=${encodeURIComponent(genreFilter)}&page=${page}&limit=50`);

        if (!response.ok) {
            throw new Error('Ошибка загрузки данных');
        }

        const data = await response.json();

        if (!data.authors || data.authors.length === 0) {
            treeContainer.innerHTML = '<div class="no-results">Ничего не найдено</div>';
            return;
        }

        treeContainer.innerHTML = '';
        renderAuthorsTree(data.authors, treeContainer, expandedState);
        renderPagination(data.total, data.page, data.limit);
        renderSummary(data.total, data.total_works, data.total_editions);

        if (expandedState && expandedState.keepFocus) {
            setTimeout(() => {
                const selector = expandedState.keepFocus.type === 'author' 
                    ? `.level-1 .edit-btn[data-id="${expandedState.keepFocus.id}"]`
                    : `.level-2 .edit-btn[data-id="${expandedState.keepFocus.id}"]`;
                const btn = document.querySelector(selector);
                if (btn) {
                    btn.focus();
                    btn.scrollIntoView({ behavior: 'smooth', block: 'center' });
                }
            }, 100);
        }
    } catch (error) {
        console.error('Error loading authors:', error);
        treeContainer.innerHTML = `<div class="error">Ошибка: ${error.message}</div>`;
    }
}

function loadAuthorsWithState(state) {
    const treeContainer = document.getElementById('authorsTree');
    const currentExpanded = state || saveExpandedState();
    treeContainer.innerHTML = '<div class="loading">Загрузка...</div>';

    let authorFilter = document.getElementById('authorFilter').value.trim().replace(/ё/g, 'е');
    let bookFilter = document.getElementById('bookFilter').value.trim().replace(/ё/g, 'е');
    let genreFilter = document.getElementById('genreFilter').value.trim();
    const page = authorsPage;

    apiFetch(`${API_BASE}/authors?author=${encodeURIComponent(authorFilter)}&book=${encodeURIComponent(bookFilter)}&genre=${encodeURIComponent(genreFilter)}&page=${page}&limit=50`)
        .then(res => {
            if (!res.ok) throw new Error('Ошибка загрузки данных');
            return res.json();
        })
        .then(data => {
            if (!data.authors || data.authors.length === 0) {
                treeContainer.innerHTML = '<div class="no-results">Ничего не найдено</div>';
                return;
            }
            treeContainer.innerHTML = '';
            renderAuthorsTree(data.authors, treeContainer, currentExpanded);
            renderPagination(data.total, data.page, data.limit);
            renderSummary(data.total, data.total_works, data.total_editions);
            if (currentExpanded.keepFocus) {
                setTimeout(() => {
                    const selector = currentExpanded.keepFocus.type === 'author' 
                        ? `.level-1 .edit-btn[data-id="${currentExpanded.keepFocus.id}"]`
                        : `.level-2 .edit-btn[data-id="${currentExpanded.keepFocus.id}"]`;
                    const btn = document.querySelector(selector);
                    if (btn) {
                        btn.focus();
                        btn.scrollIntoView({ behavior: 'smooth', block: 'center' });
                    }
                }, 100);
            }
        })
        .catch(error => {
            console.error('Error loading authors:', error);
            treeContainer.innerHTML = `<div class="error">Ошибка: ${error.message}</div>`;
        });
}

function renderPagination(total, page, limit) {
    const totalPages = Math.ceil(total / limit);
    if (totalPages <= 1) {
        document.getElementById('authorsPagination').innerHTML = '';
        document.getElementById('authorsPaginationBottom').innerHTML = '';
        return;
    }

    const html = buildPaginationHtml(totalPages, page, total);
    document.getElementById('authorsPagination').innerHTML = html;
    document.getElementById('authorsPaginationBottom').innerHTML = html;

    document.querySelectorAll('.pagination-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const p = parseInt(btn.dataset.page);
            if (p !== authorsPage) {
                authorsPage = p;
                loadAuthors();
                window.scrollTo({ top: document.querySelector('.filters').offsetTop, behavior: 'smooth' });
            }
        });
    });
}

function buildPaginationHtml(totalPages, currentPage, total) {
    let html = `<span class="page-info">${total} авторов, стр. ${currentPage} из ${totalPages}</span>`;

    if (currentPage > 1) {
        html += `<button class="pagination-btn" data-page="1">&laquo;</button>`;
        html += `<button class="pagination-btn" data-page="${currentPage - 1}">&lsaquo;</button>`;
    }

    const start = Math.max(1, currentPage - 2);
    const end = Math.min(totalPages, currentPage + 2);
    for (let i = start; i <= end; i++) {
        html += `<button class="pagination-btn${i === currentPage ? ' active' : ''}" data-page="${i}">${i}</button>`;
    }

    if (currentPage < totalPages) {
        html += `<button class="pagination-btn" data-page="${currentPage + 1}">&rsaquo;</button>`;
        html += `<button class="pagination-btn" data-page="${totalPages}">&raquo;</button>`;
    }

    return html;
}

function renderSummary(totalAuthors, totalWorks, totalEditions) {
    const html = `<div class="summary-row">
        <span>Авторов: <strong>${totalAuthors}</strong></span>
        <span>Произведений: <strong>${totalWorks}</strong></span>
        <span>Файлов изданий: <strong>${totalEditions}</strong></span>
    </div>`;
    document.getElementById('authorsSummary')?.remove();
    const summary = document.createElement('div');
    summary.id = 'authorsSummary';
    summary.className = 'tree-view';
    summary.innerHTML = html;
    document.getElementById('authorsTree').before(summary);
}

async function loadBooks() {
    const container = document.getElementById('booksTableContainer');
    container.innerHTML = '<div class="loading">Загрузка...</div>';

    const author = document.getElementById('bookAuthorFilter').value.trim().replace(/ё/g, 'е');
    const title = document.getElementById('bookTitleFilter').value.trim().replace(/ё/g, 'е');
    const genre = document.getElementById('bookGenreFilter').value.trim();
    const dateFrom = document.getElementById('bookDateFrom')?.value || '';
    const dateTo = document.getElementById('bookDateTo')?.value || '';
    const limit = 50;
    const offset = (booksPage - 1) * limit;

    let url = `${API_BASE}/books?limit=${limit}&offset=${offset}&sort_by=${booksSortBy}&sort_order=${booksSortOrder}`;
    if (author) url += `&author=${encodeURIComponent(author)}`;
    if (title) url += `&book=${encodeURIComponent(title)}`;
    if (genre) url += `&genre=${encodeURIComponent(genre)}`;
    if (dateFrom) url += `&date_from=${encodeURIComponent(dateFrom)}`;
    if (dateTo) url += `&date_to=${encodeURIComponent(dateTo)}`;

    try {
        const response = await apiFetch(url);
        if (!response.ok) throw new Error('HTTP ' + response.status);
        const data = await response.json();
        renderBooksTable(data);
        renderBooksPagination(data.total, booksPage, limit);
    } catch (err) {
        container.innerHTML = `<div class="error">Ошибка загрузки книг: ${escapeHtml(err.message)}</div>`;
    }
}

async function loadGenres() {
    const treeContainer = document.getElementById('genresTree');
    treeContainer.innerHTML = '<div class="loading">Загрузка...</div>';

    const genreFilter = document.getElementById('genreNameFilter').value.trim();

    try {
        let url = `${API_BASE}/genres/tree`;
        if (genreFilter) {
            url += `?genre=${encodeURIComponent(genreFilter)}`;
        }

        const response = await apiFetch(url);
        if (!response.ok) throw new Error('Ошибка загрузки жанров');

        const data = await response.json();
        const genres = data.genres || [];

        if (genres.length === 0) {
            treeContainer.innerHTML = '<div class="no-results">Ничего не найдено</div>';
            return;
        }

        treeContainer.innerHTML = '';
        renderGenreTree(genres, treeContainer);
    } catch (error) {
        console.error('Error loading genres:', error);
        treeContainer.innerHTML = `<div class="error">Ошибка: ${error.message}</div>`;
    }
}

function renderGenreTree(genres, container) {
    container.innerHTML = '';
    genres.forEach(genre => renderGenreNode(genre, container, 1));
}

function renderGenreNode(genre, container, depth) {
    const item = document.createElement('div');
    item.className = 'tree-item';

    const header = document.createElement('div');
    header.className = 'tree-node-header';
    header.style.marginLeft = (depth - 1) * 20 + 'px';
    header.style.padding = '10px';
    header.style.background = depth === 1 ? '#ecf0f1' : '#e8f4f8';
    header.style.borderRadius = '4px';
    header.style.marginBottom = '8px';
    header.style.cursor = 'pointer';
    header.style.display = 'flex';
    header.style.alignItems = 'center';
    header.style.gap = '10px';

    header.innerHTML = `
        <span class="expand-icon">▶</span>
        <span style="font-weight: ${depth === 1 ? 'bold' : 'normal'};">${escapeHtml(genre.name)}</span>
        <button class="edit-btn" data-type="genre" data-id="${genre.id}" data-name="${escapeHtml(genre.name)}" data-parent_id="${genre.parent_id || ''}">Редактировать</button>
        ${enableDelete ? `<button class="delete-btn" data-type="genre" data-id="${genre.id}" data-name="${escapeHtml(genre.name)}">Удалить</button>` : ''}
    `;

    header.querySelector('.edit-btn').addEventListener('click', (e) => {
        e.stopPropagation();
        const btn = e.target;
        openGenreModal({
            id: btn.dataset.id,
            name: btn.dataset.name,
            parent_id: btn.dataset.parent_id ? parseInt(btn.dataset.parent_id) : null
        });
    });

    const deleteGenreBtn = header.querySelector('.delete-btn[data-type="genre"]');
    if (deleteGenreBtn) {
        deleteGenreBtn.addEventListener('click', async (e) => {
            e.stopPropagation();
            const btn = e.target;
            const genreId = btn.dataset.id;
            const name = btn.dataset.name;
            if (!confirm(`Удалить жанр "${name}"? Книги не будут удалены.`)) return;
            try {
                const res = await apiFetch(`${API_BASE}/genres/${genreId}`, { method: 'DELETE' });
                if (!res.ok) {
                    const err = await res.json();
                    alert('Ошибка: ' + (err.error || 'Неизвестная ошибка'));
                    return;
                }
                loadGenres();
            } catch (err) {
                alert('Ошибка: ' + err.message);
            }
        });
    }

    const contentContainer = document.createElement('div');
    contentContainer.style.marginLeft = depth * 20 + 'px';
    contentContainer.style.display = 'none';

    const hasChildren = genre.children && genre.children.length > 0;

    header.addEventListener('click', async (e) => {
        if (e.target.closest('.edit-btn, .delete-btn')) return;
        const icon = header.querySelector('.expand-icon');
        const isCollapsed = contentContainer.style.display === 'none';

        if (isCollapsed) {
            contentContainer.style.display = 'block';
            icon.textContent = '▼';

            if (!contentContainer.dataset.loaded) {
                contentContainer.dataset.loaded = 'true';

                if (hasChildren) {
                    const childSection = document.createElement('div');
                    childSection.style.marginBottom = '8px';
                    genre.children.forEach(child => renderGenreNode(child, childSection, depth + 1));
                    contentContainer.appendChild(childSection);
                }

                await loadGenreAuthors(genre.id, contentContainer);
            }
        } else {
            contentContainer.style.display = 'none';
            icon.textContent = '▶';
        }
    });

    item.appendChild(header);
    item.appendChild(contentContainer);
    container.appendChild(item);
}

async function loadGenreAuthors(genreId, container) {
    const authorFilter = document.getElementById('genreAuthorFilter').value.trim().replace(/ё/g, 'е');
    const bookFilter = document.getElementById('genreBookFilter').value.trim().replace(/ё/g, 'е');

    const loadingEl = document.createElement('div');
    loadingEl.className = 'loading';
    loadingEl.textContent = 'Загрузка...';
    container.appendChild(loadingEl);

    try {
        let url = `${API_BASE}/genres/${genreId}/authors`;
        const params = [];
        if (authorFilter) params.push('author=' + encodeURIComponent(authorFilter));
        if (bookFilter) params.push('book=' + encodeURIComponent(bookFilter));
        if (params.length > 0) url += '?' + params.join('&');

        const response = await apiFetch(url);
        if (!response.ok) throw new Error('Ошибка загрузки');
        const data = await response.json();
        const authors = data.authors || [];

        loadingEl.remove();

        if (authors.length === 0) {
            const empty = document.createElement('div');
            empty.style.padding = '8px 20px';
            empty.style.color = '#999';
            empty.textContent = 'Нет книг в этом жанре';
            container.appendChild(empty);
            return;
        }

        authors.forEach(author => {
            const authorItem = document.createElement('div');
            authorItem.className = 'tree-item';

            const authorLevel = document.createElement('div');
            authorLevel.className = 'tree-node-header';
            authorLevel.style.marginLeft = '20px';
            authorLevel.style.padding = '8px 12px';
            authorLevel.style.background = '#f0f0f0';
            authorLevel.style.borderRadius = '4px';
            authorLevel.style.marginBottom = '6px';
            authorLevel.style.cursor = 'pointer';
            authorLevel.style.display = 'flex';
            authorLevel.style.alignItems = 'center';
            authorLevel.style.gap = '8px';

            const hasBooks = author.books && author.books.length > 0;
            authorLevel.innerHTML = `
                <span class="expand-icon">${hasBooks ? '▶' : ''}</span>
                <span>${escapeHtml(author.last_name)} ${escapeHtml(author.first_name || '')}</span>
            `;

            const booksContainer = document.createElement('div');
            booksContainer.style.marginLeft = '40px';
            booksContainer.style.display = 'none';

            if (hasBooks) {
                authorLevel.addEventListener('click', (e) => {
                    if (e.target.closest('.edit-btn, .download-btn, .shelf-checkbox, .delete-btn')) return;
                    const icon = authorLevel.querySelector('.expand-icon');
                    const isCollapsed = booksContainer.style.display === 'none';
                    booksContainer.style.display = isCollapsed ? 'block' : 'none';
                    icon.textContent = isCollapsed ? '▼' : '▶';
                });

                author.books.forEach(book => {
                    const bookDiv = document.createElement('div');
                    bookDiv.className = 'level-3';
                    bookDiv.innerHTML = `
                        <span class="book-title">${escapeHtml(book.title)}</span>
                        ${book.year ? `<span style="color: #666; font-size: 12px;">(${book.year})</span>` : ''}
                        <button class="download-btn" data-id="${book.id}" title="Скачать">⬇</button>
                        <input type="checkbox" class="shelf-checkbox" data-id="${book.id}" ${book.on_shelf ? 'checked' : ''} title="На полке">
                        <button class="edit-btn" data-type="book" data-id="${book.id}" data-title="${escapeHtml(book.title || '')}" data-year="${book.year || ''}">Редактировать</button>
                        ${enableDelete ? `<button class="delete-btn" data-id="${book.id}" data-title="${escapeHtml(book.title || '')}">Удалить</button>` : ''}
                    `;

                    bookDiv.querySelector('.shelf-checkbox').addEventListener('change', async (e) => {
                        e.stopPropagation();
                        const checkbox = e.target;
                        const editionId = checkbox.dataset.id;
                        const onShelf = checkbox.checked;
                        try {
                            const response = await apiFetch(`${API_BASE}/books/${editionId}/shelf`, {
                                method: 'PUT',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({ on_shelf: onShelf })
                            });
                            if (!response.ok) {
                                checkbox.checked = !onShelf;
                            } else {
                                updateShelfCount();
                            }
                        } catch (err) {
                            checkbox.checked = !onShelf;
                        }
                    });

                    bookDiv.querySelector('.edit-btn').addEventListener('click', (e) => {
                        e.stopPropagation();
                        const btn = e.target;
                        openBookModal({
                            id: btn.dataset.id,
                            title: btn.dataset.title,
                            year: btn.dataset.year
                        });
                    });

                    const deleteBtn = bookDiv.querySelector('.delete-btn');
                    if (deleteBtn) {
                        deleteBtn.addEventListener('click', async (e) => {
                            e.stopPropagation();
                            const btn = e.target;
                            const editionId = btn.dataset.id;
                            const title = btn.dataset.title;
                            if (!confirm(`Удалить книгу "${title}"?`)) return;
                            try {
                                const res = await apiFetch(`${API_BASE}/books/${editionId}`, { method: 'DELETE' });
                                if (!res.ok) {
                                    const err = await res.json();
                                    alert('Ошибка: ' + (err.error || 'Неизвестная ошибка'));
                                    return;
                                }
                                loadGenres();
                            } catch (err) {
                                alert('Ошибка: ' + err.message);
                            }
                        });
                    }

                    bookDiv.querySelector('.download-btn').addEventListener('click', (e) => {
                        e.stopPropagation();
                        triggerDownload(API_BASE + '/books/' + book.id + '/download');
                    });

                    booksContainer.appendChild(bookDiv);
                });
            }

            authorItem.appendChild(authorLevel);
            authorItem.appendChild(booksContainer);
            container.appendChild(authorItem);
        });
    } catch (err) {
        loadingEl.remove();
        const error = document.createElement('div');
        error.className = 'error';
        error.textContent = 'Ошибка: ' + err.message;
        container.appendChild(error);
    }
}

async function openGenreModal(genre) {
    const modal = document.getElementById('editModal');
    const modalTitle = document.getElementById('modalTitle');
    const modalBody = document.getElementById('modalBody');

    modalTitle.textContent = 'Редактирование жанра';
    modalTitle.dataset.type = 'genre';
    modalTitle.dataset.id = genre.id;

    let parentOptions = '<option value="">Нет родителя</option>';
    try {
        const res = await apiFetch(`${API_BASE}/genres`);
        const allGenres = await res.json();
        allGenres.forEach(g => {
            if (g.id != genre.id) {
                const selected = genre.parent_id && genre.parent_id == g.id ? 'selected' : '';
                parentOptions += `<option value="${g.id}" ${selected}>${escapeHtml(g.name || '(без названия)')}</option>`;
            }
        });
    } catch (e) {
        console.error('Failed to load genres for parent select:', e);
    }

    modalBody.innerHTML = `
        <div class="form-group">
            <label for="genre_name">Название:</label>
            <input type="text" id="genre_name" name="name" value="${escapeHtml(genre.name || '')}" required>
        </div>
        <div class="form-group">
            <label for="genre_parent">Родительский жанр:</label>
            <select id="genre_parent">
                ${parentOptions}
            </select>
        </div>
    `;

    modal.classList.add('active');
}

function renderBooksTable(data) {
    const container = document.getElementById('booksTableContainer');
    var books = data.books || [];

    if (books.length === 0) {
        container.innerHTML = '<div class="empty">Книги не найдены</div>';
        return;
    }

    // Client-side status filter
    var statusFilter = document.getElementById('bookStatusFilter')?.value || '';
    if (statusFilter) {
        books = books.filter(function(b) {
            return (getUserBookStatus(b.edition_id).status) === statusFilter;
        });
    }

    // Client-side status sort
    if (booksSortBy === 'status') {
        books.sort(function(a, b) {
            var sa = getUserBookStatus(a.edition_id).status;
            var sb = getUserBookStatus(b.edition_id).status;
            var order = ['Не заполнено','Читаю','Прочитано','Отложил','Бросил'];
            var ia = order.indexOf(sa);
            var ib = order.indexOf(sb);
            if (ia === -1) ia = 0;
            if (ib === -1) ib = 0;
            return booksSortOrder === 'asc' ? ia - ib : ib - ia;
        });
    }

    let html = '<table class="books-table"><thead><tr>';
    html += '<th class="col-num">#</th>';
    html += '<th class="col-date sortable" data-sort-by="upload_date">Дата загрузки' + getSortIcon('upload_date') + '</th>';
    html += '<th class="col-title sortable" data-sort-by="original_title">Название' + getSortIcon('original_title') + '</th>';
    html += '<th class="col-author sortable" data-sort-by="authors">Автор' + getSortIcon('authors') + '</th>';
    html += '<th class="col-year sortable" data-sort-by="year">Год' + getSortIcon('year') + '</th>';
    html += '<th class="col-format sortable" data-sort-by="available_formats">Формат' + getSortIcon('available_formats') + '</th>';
    html += '<th class="col-status sortable" data-sort-by="status">Статус' + getSortIcon('status') + '</th>';
    html += '<th class="col-shelf">Полка</th>';
    html += '<th class="col-actions">Действия</th></tr></thead><tbody>';

    books.forEach((book, index) => {
        const rowNum = (booksPage - 1) * 50 + index + 1;
        const dateStr = book.upload_date ? book.upload_date.substring(0, 10) : '';
        const title = book.original_title || book.edition_title || '';
        const authorName = getNullableValue(book, 'authors');
        const yearValue = book.year && book.year.Valid && book.year.Int64 ? book.year.Int64 : '';
        const formatName = getNullableValue(book, 'available_formats');
        const onShelf = book.on_shelf;
        const shelfIcon = onShelf ? '★' : '☆';
        const shelfTitle = onShelf ? 'Убрать с полки' : 'Добавить на полку';
        const editionId = book.edition_id;

        html += `<tr data-id="${editionId}">`;
        html += `<td class="col-num">${rowNum}</td>`;
        html += `<td class="col-date">${escapeHtml(dateStr)}</td>`;
        html += `<td class="col-title"><a href="#" class="book-title-link" data-id="${editionId}">${escapeHtml(title)}</a></td>`;
        html += `<td class="col-author">${escapeHtml(authorName)}</td>`;
        html += `<td class="col-year">${escapeHtml(String(yearValue))}</td>`;
        html += `<td class="col-format"><a href="#" class="book-download-link" data-id="${editionId}">${escapeHtml(formatName)}</a></td>`;
        var st = getUserBookStatus(editionId).status;
        html += `<td class="col-status"><select class="status-select ${getStatusClass(st)}" data-id="${editionId}" onchange="setUserBookStatus(${editionId}, this.value)">`;
        ['Не заполнено','Прочитано','Читаю','Отложил','Бросил'].forEach(function(s) {
            html += '<option value="' + s + '"' + (s === st ? ' selected' : '') + '>' + s + '</option>';
        });
        html += '</select></td>';
        html += `<td class="col-shelf"><a href="#" class="shelf-toggle" data-id="${editionId}" data-on-shelf="${onShelf}" title="${shelfTitle}">${shelfIcon}</a></td>`;
        html += `<td class="col-actions">`;
        html += `<button class="btn btn-small edit-book-btn" data-id="${editionId}" title="Редактировать">✎</button>`;
        if (enableDelete) {
            html += `<button class="btn btn-small delete-book-btn" data-id="${editionId}" data-title="${escapeHtml(title)}">Удалить</button>`;
        }
        html += `</td></tr>`;
    });

    html += '</tbody></table>';
    container.innerHTML = html;
    container.querySelectorAll('th.sortable').forEach(function(th) {
        th.addEventListener('click', function(e) {
            var sortBy = this.dataset.sortBy;
            if (sortBy) {
                if (booksSortBy === sortBy) {
                    booksSortOrder = booksSortOrder === 'asc' ? 'desc' : 'asc';
                } else {
                    booksSortBy = sortBy;
                    booksSortOrder = 'asc';
                }
                booksPage = 1;
                loadBooks();
            }
        });
    });
}

function getSortIcon(sortBy) {
    if (booksSortBy !== sortBy) return ' ↕';
    return booksSortOrder === 'asc' ? ' ▲' : ' ▼';
}

function renderBooksPagination(total, page, limit) {
    const totalPages = Math.ceil(total / limit);
    const topEl = document.getElementById('booksPagination');
    const bottomEl = document.getElementById('booksPaginationBottom');

    if (totalPages <= 1) {
        topEl.innerHTML = `<span class="page-info">${total} книг</span>`;
        bottomEl.innerHTML = '';
        return;
    }

    let html = `<span class="page-info">${total} книг, стр. ${page} из ${totalPages}</span>`;
    if (page > 1) {
        html += `<button class="pagination-btn" data-page="1">&laquo;</button>`;
        html += `<button class="pagination-btn" data-page="${page - 1}">&lsaquo;</button>`;
    }
    const start = Math.max(1, page - 2);
    const end = Math.min(totalPages, page + 2);
    for (let i = start; i <= end; i++) {
        html += `<button class="pagination-btn${i === page ? ' active' : ''}" data-page="${i}">${i}</button>`;
    }
    if (page < totalPages) {
        html += `<button class="pagination-btn" data-page="${page + 1}">&rsaquo;</button>`;
        html += `<button class="pagination-btn" data-page="${totalPages}">&raquo;</button>`;
    }

    topEl.innerHTML = html;
    bottomEl.innerHTML = html;
}

function setupBooksTableEvents() {
    function bindContainer(containerId) {
        const container = document.getElementById(containerId);
        if (!container) return;

        container.addEventListener('click', (e) => {
            const th = e.target.closest('th.sortable');
            if (th) {
                const sortBy = th.dataset.sortBy;
                if (sortBy) {
                    if (booksSortBy === sortBy) {
                        booksSortOrder = booksSortOrder === 'asc' ? 'desc' : 'asc';
                    } else {
                        booksSortBy = sortBy;
                        booksSortOrder = 'asc';
                    }
                    booksPage = 1;
                    loadBooks();
                }
                return;
            }

            const editBtn = e.target.closest('.edit-book-btn');
            if (editBtn) {
                e.stopPropagation();
                openBookModal({ id: editBtn.dataset.id });
                return;
            }

            const deleteBtn = e.target.closest('.delete-book-btn');
            if (deleteBtn) {
                e.preventDefault();
                const id = deleteBtn.dataset.id;
                const title = deleteBtn.dataset.title;
                if (!confirm(`Удалить книгу "${title}"?`)) return;
                (async () => {
                    try {
                        const r = await apiFetch(`${API_BASE}/books/${id}`, { method: 'DELETE' });
                        if (!r.ok) {
                            const err = await r.json();
                            alert('Ошибка: ' + (err.error || 'Неизвестная ошибка'));
                            return;
                        }
                        loadBooks();
                    } catch (err) {
                        alert('Ошибка: ' + err.message);
                    }
                })();
                return;
            }

            const shelfEl = e.target.closest('.shelf-toggle');
            if (shelfEl) {
                e.preventDefault();
                const id = shelfEl.dataset.id;
                const onShelf = shelfEl.dataset.on_shelf === 'true';
                (async () => {
                    try {
                        const r = await apiFetch(`${API_BASE}/books/${id}/shelf`, {
                            method: 'PUT',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ on_shelf: !onShelf })
                        });
                        if (r.ok) {
                            shelfEl.dataset.on_shelf = !onShelf;
                            shelfEl.textContent = !onShelf ? '★' : '☆';
                            shelfEl.title = !onShelf ? 'Убрать с полки' : 'Добавить на полку';
                            updateShelfCount();
                        }
                    } catch (err) { console.error(err); }
                })();
                return;
            }

            const titleLink = e.target.closest('.book-title-link');
            if (titleLink) {
                e.preventDefault();
                openBookModal({ id: titleLink.dataset.id });
                return;
            }

            const downloadLink = e.target.closest('.book-download-link');
            if (downloadLink) {
                e.preventDefault();
                triggerDownload(API_BASE + '/books/' + downloadLink.dataset.id + '/download');
                return;
            }

            const pageBtn = e.target.closest('.pagination-btn');
            if (pageBtn) {
                const p = parseInt(pageBtn.dataset.page);
                if (p !== booksPage) {
                    booksPage = p;
                    loadBooks();
                    window.scrollTo({ top: document.querySelector('.filters').offsetTop, behavior: 'smooth' });
                }
            }
        });
    }
    bindContainer('books');
    bindContainer('tab-books');
}


function renderAuthorsTree(authors, container, expandedState = null) {
    container.innerHTML = '';

    authors.forEach(author => {
        const authorItem = document.createElement('div');
        authorItem.className = 'tree-item';

        const isExpanded = expandedState && expandedState.authors.has(String(author.id));
        const authorLevel = document.createElement('div');
        authorLevel.className = 'level-1' + (isExpanded ? '' : ' collapsed');
        authorLevel.innerHTML = `
            <span class="expand-icon">${isExpanded ? '▼' : '▶'}</span>
            <span class="author-name">${escapeHtml(author.last_name)} ${escapeHtml(author.first_name || '')}</span>
            <span style="color: #666; font-size: 12px;">(${author.books_count || 0} книг)</span>
            <button class="edit-btn" data-type="author" data-id="${author.id}" data-first_name="${escapeHtml(author.first_name || '')}" data-last_name="${escapeHtml(author.last_name || '')}">Редактировать</button>
        `;

        authorLevel.querySelector('.edit-btn').addEventListener('click', (e) => {
            e.stopPropagation();
            const btn = e.target;
            openAuthorModal({
                id: btn.dataset.id,
                first_name: btn.dataset.first_name,
                last_name: btn.dataset.last_name
            });
        });

        authorLevel.addEventListener('click', (e) => {
            if (e.target.classList.contains('edit-btn')) return;
            authorLevel.classList.toggle('collapsed');
            const icon = authorLevel.querySelector('.expand-icon');
            icon.textContent = authorLevel.classList.contains('collapsed') ? '▶' : '▼';
            const booksContainer = authorLevel.nextElementSibling;
            if (booksContainer) {
                booksContainer.style.display = authorLevel.classList.contains('collapsed') ? 'none' : 'block';
            }
        });

        const booksContainer = document.createElement('div');
        booksContainer.className = 'level-2-container';
        booksContainer.style.display = isExpanded ? 'block' : 'none';

        if (author.books && author.books.length > 0) {
            author.books.sort(function(a, b) {
                var yearA = a.year !== undefined && a.year !== null ? a.year : null;
                var yearB = b.year !== undefined && b.year !== null ? b.year : null;
                if (yearA === yearB) {
                    var titleA = (a.title || '').toLowerCase();
                    var titleB = (b.title || '').toLowerCase();
                    return titleA < titleB ? -1 : titleA > titleB ? 1 : 0;
                }
                if (yearA === null) return 1;
                if (yearB === null) return -1;
                return yearB - yearA;
            });
            author.books.forEach(book => {
                const bookItem = document.createElement('div');
                bookItem.className = 'tree-item';

                const isBookExpanded = isExpanded && expandedState && expandedState.books.has(String(book.id));
                const bookLevel = document.createElement('div');
                bookLevel.className = 'level-2' + (isBookExpanded ? '' : ' collapsed');
                bookLevel.innerHTML = `
                    <span class="expand-icon">${isBookExpanded ? '▼' : '▶'}</span>
                    <span class="book-title">${escapeHtml(book.title)}</span>
                    ${book.year ? `<span style="color: #666; font-size: 12px;">(${book.year})</span>` : ''}
                    <button class="download-btn" data-id="${book.id}" title="Скачать">⬇</button>
                    <input type="checkbox" class="shelf-checkbox" data-id="${book.id}" ${book.on_shelf ? 'checked' : ''} title="На полке">
                    <button class="edit-btn" data-type="book" data-id="${book.id}" data-title="${escapeHtml(book.title || '')}" data-year="${book.year || ''}">Редактировать</button>
                    ${enableDelete ? `<button class="delete-btn" data-id="${book.id}" data-title="${escapeHtml(book.title || '')}">Удалить</button>` : ''}
                `;

                bookLevel.querySelector('.shelf-checkbox').addEventListener('change', async (e) => {
                    e.stopPropagation();
                    const checkbox = e.target;
                    const editionId = checkbox.dataset.id;
                    const onShelf = checkbox.checked;
                    
                    try {
                        const response = await apiFetch(`${API_BASE}/books/${editionId}/shelf`, {
                            method: 'PUT',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ on_shelf: onShelf })
                        });
                        if (!response.ok) {
                            checkbox.checked = !onShelf;
                        } else {
                            updateShelfCount();
                        }
                    } catch (err) {
                        checkbox.checked = !onShelf;
                    }
                });

                bookLevel.querySelector('.edit-btn').addEventListener('click', (e) => {
                    e.stopPropagation();
                    const btn = e.target;
                    openBookModal({
                        id: btn.dataset.id,
                        title: btn.dataset.title,
                        year: btn.dataset.year
                    });
                });

                const deleteBtn = bookLevel.querySelector('.delete-btn');
                if (deleteBtn) {
                    deleteBtn.addEventListener('click', async (e) => {
                        e.stopPropagation();
                        const btn = e.target;
                        const editionId = btn.dataset.id;
                        const title = btn.dataset.title;
                        if (!confirm(`Удалить книгу "${title}"?`)) return;
                        try {
                            const res = await apiFetch(`${API_BASE}/books/${editionId}`, {
                                method: 'DELETE'
                            });
                            if (!res.ok) {
                                const err = await res.json();
                                alert('Ошибка: ' + (err.error || 'Неизвестная ошибка'));
                                return;
                            }
                            loadAuthors();
                        } catch (err) {
                            alert('Ошибка: ' + err.message);
                        }
                    });
                }

                bookLevel.querySelector('.download-btn').addEventListener('click', (e) => {
                    e.stopPropagation();
                    const btn = e.target;
                    const editionId = btn.dataset.id;
                    triggerDownload(API_BASE + '/books/' + editionId + '/download');
                });

                bookLevel.addEventListener('click', (e) => {
                    if (e.target.classList.contains('edit-btn') || e.target.classList.contains('shelf-checkbox') || e.target.classList.contains('download-btn') || e.target.classList.contains('delete-btn')) return;
                    bookLevel.classList.toggle('collapsed');
                    const icon = bookLevel.querySelector('.expand-icon');
                    icon.textContent = bookLevel.classList.contains('collapsed') ? '▶' : '▼';
                    const formatsContainer = bookLevel.nextElementSibling;
                    if (formatsContainer) {
                        formatsContainer.style.display = bookLevel.classList.contains('collapsed') ? 'none' : 'block';
                    }
                });

                const formatsContainer = document.createElement('div');
                formatsContainer.className = 'level-3-container';
                formatsContainer.style.display = isBookExpanded ? 'block' : 'none';

                if (book.formats && book.formats.length > 0) {
                    book.formats.forEach(format => {
                        const formatItem = document.createElement('div');
                        formatItem.className = 'level-3';
                        formatItem.innerHTML = `
                            <span class="format-badge">${escapeHtml(format.format_name)}</span>
                            ${format.file_path ? `<span style="color: #999;">${escapeHtml(format.file_path)}</span>` : ''}
                        `;
                        formatsContainer.appendChild(formatItem);
                    });
                }

                bookItem.appendChild(bookLevel);
                bookItem.appendChild(formatsContainer);
                booksContainer.appendChild(bookItem);
            });
        }

        authorItem.appendChild(authorLevel);
        authorItem.appendChild(booksContainer);
        container.appendChild(authorItem);
    });
}

async function openAuthorModal(author) {
    const modal = document.getElementById('editModal');
    const modalTitle = document.getElementById('modalTitle');
    const modalBody = document.getElementById('modalBody');

    modalTitle.textContent = 'Редактирование автора';
    modalTitle.dataset.type = 'author';
    modalTitle.dataset.id = author.id;

    modalBody.innerHTML = '<div class="loading">Загрузка...</div>';
    modal.classList.add('active');

    try {
        const res = await apiFetch(`${API_BASE}/persons/${author.id}`);
        const p = await res.json();
        modalBody.innerHTML = `
            <div class="form-group">
                <label for="first_name">Имя:</label>
                <input type="text" id="first_name" name="first_name" value="${escapeHtml(p.first_name || '')}">
            </div>
            <div class="form-group">
                <label for="last_name">Фамилия:</label>
                <input type="text" id="last_name" name="last_name" value="${escapeHtml(p.last_name || '')}" required>
            </div>
            <div class="form-group">
                <label for="middle_name">Отчество:</label>
                <input type="text" id="middle_name" name="middle_name" value="${escapeHtml(p.middle_name || '')}">
            </div>
            <div class="form-group">
                <label for="pseudonym">Псевдоним:</label>
                <input type="text" id="pseudonym" name="pseudonym" value="${escapeHtml(p.pseudonym || '')}">
            </div>
            <div class="form-group">
                <label for="birth_date">Дата рождения:</label>
                <input type="date" id="birth_date" name="birth_date" value="${p.birth_date || ''}">
            </div>
            <div class="form-group">
                <label for="death_date">Дата смерти:</label>
                <input type="date" id="death_date" name="death_date" value="${p.death_date || ''}">
            </div>
            <div class="form-group">
                <label for="biography">Биография:</label>
                <textarea id="biography" name="biography" rows="3">${escapeHtml(p.biography || '')}</textarea>
            </div>
        `;
    } catch(e) {
        modalBody.innerHTML = '<p class="error">Ошибка загрузки данных</p>';
    }
}

async function openBookModal(book) {
    const modal = document.getElementById('editModal');
    const modalTitle = document.getElementById('modalTitle');
    const modalBody = document.getElementById('modalBody');

    modalTitle.textContent = 'Редактирование книги';
    modalTitle.dataset.type = 'book';
    modalTitle.dataset.id = book.id;

    modalBody.innerHTML = '<div class="loading">Загрузка данных...</div>';
    modal.classList.add('active');

    try {
        const response = await apiFetch(`${API_BASE}/books/${book.id}/extended`);
        if (!response.ok) throw new Error('Ошибка загрузки данных');
        const data = await response.json();

        const [genresRes, tagsRes, personsRes, languagesRes] = await Promise.all([
            apiFetch(`${API_BASE}/genres`),
            apiFetch(`${API_BASE}/tags`),
            apiFetch(`${API_BASE}/persons`),
            apiFetch(`${API_BASE}/languages`)
        ]);

        const genres = await genresRes.json();
        const tags = await tagsRes.json();
        const persons = await personsRes.json();
        const languages = await languagesRes.json();

        renderExtendedBookForm(data, genres, tags, persons, languages);
    } catch (error) {
        modalBody.innerHTML = `<div class="error">Ошибка: ${error.message}</div>`;
    }
}

function renderExtendedBookForm(data, genres, tags, persons, languages) {
    const modalBody = document.getElementById('modalBody');
    const book = data;
    const work = book.work || {};
    const edition = book.edition || {};

    const workTitle = getNullableValue(work, 'original_title');
    const workLanguage = getNullableValue(work, 'original_language');
    const workType = getNullableValue(work, 'work_type');
    const workAnnotation = getNullableValue(work, 'annotation');
    const workFirstPublished = work.first_published && work.first_published.Valid ? work.first_published.Int64 : '';

    const editionTitle = getNullableValue(edition, 'title');
    const editionLanguage = getNullableValue(edition, 'language');
    const editionYear = edition.year && edition.year.Valid ? edition.year.Int64 : '';
    const editionPages = edition.pages && edition.pages.Valid ? edition.pages.Int64 : '';
    const editionIsComplete = edition.is_complete !== false;
    const editionQuality = getNullableValue(edition, 'quality');
    const editionAnnotation = getNullableValue(edition, 'annotation');
    const editionSource = getNullableValue(edition, 'source');
    const editionCity = getNullableValue(edition, 'city');
    const editionSeries = getNullableValue(edition, 'series');
    const editionSeriesNumber = getNullableValue(edition, 'series_number');
    const editionPublisher = getNullableValue(edition, 'publisher');
    const editionISBN = getNullableValue(edition, 'isbn');
    const editionUploadDate = edition.upload_date ? edition.upload_date.substring(0, 10) : '';

    const genresArray = Array.isArray(genres) ? genres : [];
    const tagsArray = Array.isArray(tags) ? tags : [];
    const personsArray = Array.isArray(persons) ? persons : [];
    const languagesArray = Array.isArray(languages) ? languages : [];

    let genresOptions = genresArray.map(g => {
        const selected = book.genres && book.genres.some(bg => bg.id === g.id) ? 'selected' : '';
        return `<option value="${g.id}" ${selected}>${escapeHtml(g.name)}</option>`;
    }).join('');

    let tagsOptions = tagsArray.map(t => {
        const selected = book.tags && book.tags.some(bt => bt.id === t.id) ? 'selected' : '';
        return `<option value="${t.id}" ${selected}>${escapeHtml(t.name)}</option>`;
    }).join('');

    let personsOptions = `<option value="">-- Выберите --</option>` +
        personsArray.map(p => `<option value="${p.id}">${escapeHtml(p.last_name)} ${escapeHtml(p.first_name || '')}</option>`).join('');

    let languageOptions = `<option value="">-- Выберите --</option>` +
        languagesArray.map(l => `<option value="${l.code}" ${editionLanguage === l.code ? 'selected' : ''}>${escapeHtml(l.name)} (${escapeHtml(l.native_name || '')})</option>`).join('');

    let authorsHtml = '';
    if (book.authors && book.authors.length > 0) {
        book.authors.forEach((author, idx) => {
            const authorName = `${author.last_name} ${author.first_name || ''}`.trim();
            const personsOpts = personsArray.map(p =>
                `<option value="${p.id}">${escapeHtml(p.last_name)} ${escapeHtml(p.first_name || '')}</option>`
            ).join('');
            authorsHtml += `
                <div class="author-row" data-idx="${idx}">
                    <input type="hidden" class="author-id" value="${author.id}">
                    <div class="author-autocomplete-group">
                        <input type="text" class="author-autocomplete" value="${escapeHtml(authorName)}" autocomplete="off" placeholder="Начните вводить автора...">
                        <select class="author-popup" size="5" style="display:none;margin-top:2px;width:100%">
                            <option value="">-- Выберите --</option>
                            ${personsOpts}
                        </select>
                    </div>
                    <select name="author_role_${idx}" class="author-role">
                        <option value="author" ${author.role === 'author' ? 'selected' : ''}>Автор</option>
                        <option value="translator" ${author.role === 'translator' ? 'selected' : ''}>Переводчик</option>
                        <option value="editor" ${author.role === 'editor' ? 'selected' : ''}>Редактор</option>
                        <option value="illustrator" ${author.role === 'illustrator' ? 'selected' : ''}>Иллюстратор</option>
                    </select>
                    <button type="button" class="btn-remove-author" onclick="removeAuthorRow(this)">✕</button>
                </div>`;
        });
    }

    let filesHtml = '';
    if (book.files && book.files.length > 0) {
        filesHtml = '<div class="files-list">';
        book.files.forEach(file => {
            const fileSize = file.file_size && file.file_size.Valid ? file.file_size.Int64 : null;
            filesHtml += `
                <div class="file-item">
                    <span class="format-badge">${escapeHtml(file.format_name)}</span>
                    <span class="file-path">${escapeHtml(file.file_path)}</span>
                    ${fileSize ? `<span class="file-size">${(fileSize / 1024).toFixed(1)} KB</span>` : ''}
                    ${file.is_primary ? '<span class="primary-badge">Основной</span>' : ''}
                </div>`;
        });
        filesHtml += '</div>';
    }

    let tocButtonHtml = book.toc && book.toc.length > 0
        ? `<button type="button" class="btn" onclick="openTocEditor('${edition.id}')">Редактировать оглавление (${book.toc.length} записей)</button>`
        : `<button type="button" class="btn" onclick="openTocEditor('${edition.id}')">Добавить оглавление</button>`;

    modalBody.innerHTML = `
        <input type="hidden" name="work_id" value="${work.id || ''}">
        <input type="hidden" name="edition_id" value="${edition.id || ''}">

        <fieldset>
            <legend>Произведение (Work)</legend>
            <div class="form-group">
                <label for="original_title">Оригинальное название:</label>
                <input type="text" id="original_title" name="original_title" value="${escapeHtml(workTitle)}">
            </div>
            <div class="form-group">
                <label for="work_language">Язык произведения:</label>
                <select id="work_language" name="work_language">
                    ${languageOptions}
                </select>
            </div>
            <div class="form-group">
                <label for="first_published">Год первой публикации:</label>
                <input type="number" id="first_published" name="first_published" value="${workFirstPublished}">
            </div>
            <div class="form-group">
                <label for="work_type">Тип произведения:</label>
                <select id="work_type" name="work_type">
                    <option value="novel" ${workType === 'novel' ? 'selected' : ''}>Роман</option>
                    <option value="story" ${workType === 'story' ? 'selected' : ''}>Рассказ</option>
                    <option value="poem" ${workType === 'poem' ? 'selected' : ''}>Стихотворение</option>
                    <option value="collection" ${workType === 'collection' ? 'selected' : ''}>Сборник</option>
                    <option value="article" ${workType === 'article' ? 'selected' : ''}>Статья</option>
                </select>
            </div>
            <div class="form-group">
                <label for="work_annotation">Аннотация произведения:</label>
                <textarea id="work_annotation" name="work_annotation" rows="3">${escapeHtml(workAnnotation)}</textarea>
            </div>
        </fieldset>

        <fieldset>
            <legend>Авторы</legend>
            <div id="authors-container">${authorsHtml}</div>
            <button type="button" class="btn btn-secondary" onclick="addAuthorRow()">+ Добавить автора</button>
            <div class="form-group">
                <label>Или создать нового:</label>
                <div style="display: flex; gap: 10px;">
                    <input type="text" id="new_author_first" placeholder="Имя" style="flex: 1;">
                    <input type="text" id="new_author_last" placeholder="Фамилия" style="flex: 1;">
                    <select id="new_author_role" style="width: 120px;">
                        <option value="author">Автор</option>
                        <option value="translator">Переводчик</option>
                        <option value="editor">Редактор</option>
                    </select>
                    <button type="button" class="btn" onclick="addNewAuthor()">Добавить</button>
                </div>
            </div>
        </fieldset>

        <fieldset>
            <legend>Издание (Edition)</legend>
            <div class="form-group">
                <label for="edition_title">Название издания:</label>
                <input type="text" id="edition_title" name="edition_title" value="${escapeHtml(editionTitle)}">
            </div>
            <div class="form-group">
                <label for="edition_language">Язык издания:</label>
                <select id="edition_language" name="edition_language">
                    ${languageOptions}
                </select>
            </div>
            <div class="form-group">
                <label for="edition_year">Год издания:</label>
                <input type="number" id="edition_year" name="edition_year" value="${editionYear}">
            </div>
            <div class="form-group">
                <label for="publisher">Издательство:</label>
                <input type="text" id="publisher" name="publisher" value="${escapeHtml(editionPublisher)}">
            </div>
            <div class="form-group">
                <label for="isbn">ISBN:</label>
                <input type="text" id="isbn" name="isbn" value="${escapeHtml(editionISBN)}">
            </div>
            <div class="form-group">
                <label for="edition_pages">Количество страниц:</label>
                <input type="number" id="edition_pages" name="edition_pages" value="${editionPages}">
            </div>
            <div class="form-group">
                <label for="cover_url">URL обложки (или загрузите через кнопку выше):</label>
                <input type="text" id="cover_url" name="cover_url" value="${escapeHtml(edition.cover_url || '')}">
            </div>
            <div class="form-group">
                <label for="edition_annotation">Аннотация издания:</label>
                <textarea id="edition_annotation" name="edition_annotation" rows="3">${escapeHtml(editionAnnotation)}</textarea>
            </div>
            <div class="form-row">
                <div class="form-group" style="flex: 1;">
                    <label for="edition_quality">Качество:</label>
                    <select id="edition_quality" name="edition_quality">
                        <option value="">—</option>
                        <option value="good" ${editionQuality === 'good' ? 'selected' : ''}>Хорошее</option>
                        <option value="acceptable" ${editionQuality === 'acceptable' ? 'selected' : ''}>Приемлемое</option>
                        <option value="poor" ${editionQuality === 'poor' ? 'selected' : ''}>Плохое</option>
                    </select>
                </div>
                <div class="form-group" style="flex: 1;">
                    <label for="upload_date">Дата загрузки:</label>
                    <input type="date" id="upload_date" name="upload_date" value="${editionUploadDate}">
                </div>
                <div class="form-group" style="flex: 1;">
                    <label>
                        <input type="checkbox" id="is_complete" name="is_complete" ${editionIsComplete ? 'checked' : ''}>
                        Полное издание
                    </label>
                </div>
            </div>
        </fieldset>

        <fieldset>
            <legend>Файлы издания</legend>
            ${filesHtml || '<div class="no-files">Нет файлов</div>'}
        </fieldset>

        <fieldset>
            <legend>Жанры</legend>
            <div class="form-group">
                <select id="genres_select" multiple size="5" style="height: 100px;">
                    ${genresOptions}
                </select>
                <div style="margin-top: 5px;">
                    <span>Выбрано: <span id="selected_genres_count">${book.genres ? book.genres.length : 0}</span></span>
                </div>
            </div>
            <div class="form-group">
                <label>Создать новый жанр:</label>
                <div style="display: flex; gap: 10px;">
                    <input type="text" id="new_genre_name" placeholder="Название жанра" style="flex: 1;">
                    <button type="button" class="btn" onclick="addNewGenre()">Создать</button>
                </div>
            </div>
        </fieldset>

        <fieldset>
            <legend>Теги</legend>
            <div class="form-group">
                <select id="tags_select" multiple size="5" style="height: 100px;">
                    ${tagsOptions}
                </select>
                <div style="margin-top: 5px;">
                    <span>Выбрано: <span id="selected_tags_count">${book.tags ? book.tags.length : 0}</span></span>
                </div>
            </div>
            <div class="form-group">
                <label>Создать новый тег:</label>
                <div style="display: flex; gap: 10px;">
                    <input type="text" id="new_tag_name" placeholder="Название тега" style="flex: 1;">
                    <button type="button" class="btn" onclick="addNewTag()">Создать</button>
                </div>
            </div>
        </fieldset>

        <fieldset>
            <legend>Оглавление</legend>
            ${tocButtonHtml}
        </fieldset>
    `;

    document.getElementById('genres_select')?.addEventListener('change', function() {
        const selected = this.selectedOptions.length;
        document.getElementById('selected_genres_count').textContent = selected;
    });

    document.getElementById('tags_select')?.addEventListener('change', function() {
        const selected = this.selectedOptions.length;
        document.getElementById('selected_tags_count').textContent = selected;
    });

    document.querySelectorAll('#authors-container .author-row').forEach(function(row) {
        var input = row.querySelector('.author-autocomplete');
        var popup = row.querySelector('.author-popup');
        if (input && popup) setupAuthorAutocomplete(input, popup);
    });
}

function setupAuthorAutocomplete(input, popup) {
    if (!input || !popup) return;
    input.addEventListener('input', function() {
        var val = this.value.toLowerCase();
        var opts = popup.options;
        var matched = [];
        for (var i = 0; i < opts.length; i++) {
            if (opts[i].value === '') continue;
            var matches = opts[i].textContent.toLowerCase().indexOf(val) !== -1;
            opts[i].style.display = matches ? '' : 'none';
            if (matches) matched.push(opts[i]);
        }
        var row = this.closest('.author-row');
        var hiddenId = row ? row.querySelector('.author-id') : null;
        if (hiddenId) hiddenId.value = '';
        popup.style.display = (val && matched.length > 0) ? '' : 'none';
    });
    input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
            var val = this.value.toLowerCase();
            if (!val) return;
            var opts = popup.options;
            var matched = [];
            for (var i = 0; i < opts.length; i++) {
                if (opts[i].value === '') continue;
                if (opts[i].textContent.toLowerCase().indexOf(val) !== -1) {
                    matched.push(opts[i]);
                }
            }
            if (matched.length === 1) {
                e.preventDefault();
                fillAuthorSelection(this, popup, matched[0]);
            }
        }
    });
    input.addEventListener('blur', function() {
        var val = this.value.toLowerCase();
        if (!val) return;
        var opts = popup.options;
        var matched = [];
        for (var i = 0; i < opts.length; i++) {
            if (opts[i].value === '') continue;
            if (opts[i].textContent.toLowerCase().indexOf(val) !== -1) {
                matched.push(opts[i]);
            }
        }
        if (matched.length === 1) {
            fillAuthorSelection(this, popup, matched[0]);
        }
    });
    popup.addEventListener('change', function() {
        var row = input.closest('.author-row');
        var hiddenId = row ? row.querySelector('.author-id') : null;
        if (this.value) {
            fillAuthorSelection(input, popup, this.options[this.selectedIndex]);
        } else {
            if (hiddenId) hiddenId.value = '';
        }
    });
}

function fillAuthorSelection(input, popup, opt) {
    opt.selected = true;
    var row = input.closest('.author-row');
    var hiddenId = row ? row.querySelector('.author-id') : null;
    if (hiddenId) hiddenId.value = opt.value;
    input.value = opt.textContent;
    popup.style.display = 'none';
}

function addAuthorRow() {
    const container = document.getElementById('authors-container');
    apiFetch(`${API_BASE}/persons`)
        .then(res => res.json())
        .then(persons => {
            const personsArray = Array.isArray(persons) ? persons : [];
            const idx = container.querySelectorAll('.author-row').length;
            const personsOpts = personsArray.map(p =>
                `<option value="${p.id}">${escapeHtml(p.last_name)} ${escapeHtml(p.first_name || '')}</option>`
            ).join('');

            const div = document.createElement('div');
            div.className = 'author-row';
            div.dataset.idx = idx;
            div.innerHTML = `
                <input type="hidden" class="author-id" value="">
                <div class="author-autocomplete-group">
                    <input type="text" class="author-autocomplete" autocomplete="off" placeholder="Начните вводить автора...">
                    <select class="author-popup" size="5" style="display:none;margin-top:2px;width:100%">
                        <option value="">-- Выберите --</option>
                        ${personsOpts}
                    </select>
                </div>
                <select class="author-role">
                    <option value="author">Автор</option>
                    <option value="translator">Переводчик</option>
                    <option value="editor">Редактор</option>
                    <option value="illustrator">Иллюстратор</option>
                </select>
                <button type="button" class="btn-remove-author" onclick="removeAuthorRow(this)">✕</button>
            `;
            container.appendChild(div);
            var input = div.querySelector('.author-autocomplete');
            var popup = div.querySelector('.author-popup');
            if (input && popup) setupAuthorAutocomplete(input, popup);
        });
}

function removeAuthorRow(btn) {
    btn.parentElement.remove();
}

async function addNewAuthor() {
    const firstName = document.getElementById('new_author_first').value.trim();
    const lastName = document.getElementById('new_author_last').value.trim();
    const role = document.getElementById('new_author_role').value;

    if (!lastName) {
        alert('Введите фамилию');
        return;
    }

    const container = document.getElementById('authors-container');
    const idx = container.querySelectorAll('.author-row').length;

    const div = document.createElement('div');
    div.className = 'author-row new-author-row';
    div.dataset.idx = idx;
    div.dataset.new = 'true';
    div.dataset.firstName = firstName;
    div.dataset.lastName = lastName;
    div.dataset.role = role;
    div.innerHTML = `
        <span>${escapeHtml(lastName)} ${escapeHtml(firstName)} (${role})</span>
        <button type="button" class="btn-remove-author" onclick="this.parentElement.remove()">✕</button>
    `;
    container.appendChild(div);

    document.getElementById('new_author_first').value = '';
    document.getElementById('new_author_last').value = '';
}

async function addNewGenre() {
    const name = document.getElementById('new_genre_name').value.trim();
    if (!name) {
        alert('Введите название жанра');
        return;
    }

    try {
        const response = await apiFetch(`${API_BASE}/genres`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name })
        });
        if (!response.ok) throw new Error('Ошибка создания жанра');
        const genre = await response.json();

        const select = document.getElementById('genres_select');
        const option = document.createElement('option');
        option.value = genre.id;
        option.textContent = genre.name;
        option.selected = true;
        select.appendChild(option);

        document.getElementById('new_genre_name').value = '';
        document.getElementById('selected_genres_count').textContent = select.selectedOptions.length;
    } catch (error) {
        alert(error.message);
    }
}

async function addNewTag() {
    const name = document.getElementById('new_tag_name').value.trim();
    if (!name) {
        alert('Введите название тега');
        return;
    }

    try {
        const response = await apiFetch(`${API_BASE}/tags`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name })
        });
        if (!response.ok) throw new Error('Ошибка создания тега');
        const tag = await response.json();

        const select = document.getElementById('tags_select');
        const option = document.createElement('option');
        option.value = tag.id;
        option.textContent = tag.name;
        option.selected = true;
        select.appendChild(option);

        document.getElementById('new_tag_name').value = '';
        document.getElementById('selected_tags_count').textContent = select.selectedOptions.length;
    } catch (error) {
        alert(error.message);
    }
}

function openTocEditor(editionId) {
    alert('Редактор оглавления для издания ' + editionId + ' - в разработке');
}

function closeModal() {
    const modal = document.getElementById('editModal');
    modal.classList.remove('active');
}

// ============ Список чтения ============

var RL_API = API_BASE + '/user/readlist';

function isOnline() {
    return typeof navigator !== 'undefined' && navigator.onLine !== false;
}

function isAndroid() {
    return document.body.classList.contains('android');
}

function getAuthHeaders() {
    var token = localStorage.getItem('auth_token');
    var h = { 'Content-Type': 'application/json' };
    if (token) h['Authorization'] = 'Bearer ' + token;
    return h;
}

async function loadReadlist() {
    var container = document.getElementById('readlistTableContainer');
    if (!container) return;
    container.innerHTML = '<div class="loading">Загрузка...</div>';

    var listname = document.getElementById('readlistNameFilter').value;
    var bookname = document.getElementById('readlistBookFilter').value.trim();
    var author = document.getElementById('readlistAuthorFilter').value.trim();
    var comment = document.getElementById('readlistCommentFilter').value.trim();
    var statusFilter = document.getElementById('readlistStatusFilter').value.trim();
    var limit = 50;
    var offset = (readlistPage - 1) * limit;

    // Android: always read from local SQLite
    if (isAndroid() && window.ReadListStore) {
        // Trigger background sync if online (errors are handled internally)
        if (window.SyncService && typeof navigator !== 'undefined' && navigator.onLine) {
            try { SyncService.sync(); } catch(e) {}
        }
        try {
            var allItems = ReadListStore.query(listname, bookname, author, statusFilter);
            var total = allItems.length;
            var items = allItems.slice(offset, offset + limit);
            var data = { items: items, total: total };
            renderReadlistTable(data);
            renderReadlistPagination(total, readlistPage, limit);
        } catch(e) {
            // If local read fails (bridge error), render empty list gracefully
            renderReadlistTable({ items: [], total: 0 });
            renderReadlistPagination(0, readlistPage, limit);
        }
        return;
    }

    var url = RL_API + '?limit=' + limit + '&offset=' + offset +
        '&sort_by=' + readlistSortBy + '&sort_order=' + readlistSortOrder;
    if (listname) url += '&listname=' + encodeURIComponent(listname);
    if (bookname) url += '&bookname=' + encodeURIComponent(bookname);
    if (author) url += '&author=' + encodeURIComponent(author);
    if (comment) url += '&comment=' + encodeURIComponent(comment);
    if (statusFilter) url += '&status=' + encodeURIComponent(statusFilter);

    try {
        var res = await apiFetch(url, { headers: getAuthHeaders() });
        if (res.status === 401) { handleAuthFailure(); return; }
        if (!res.ok) throw new Error('HTTP ' + res.status);
        var data = await res.json();
        renderReadlistTable(data);
        renderReadlistPagination(data.total, readlistPage, limit);
    } catch (err) {
        container.innerHTML = '<div class="error">Ошибка загрузки: ' + escapeHtml(err.message) + '</div>';
    }
}

function renderReadlistTable(data) {
    var container = document.getElementById('readlistTableContainer');
    var items = data.items || [];

    if (items.length === 0) {
        container.innerHTML = '<div class="empty">Записи не найдены</div>';
        return;
    }

    var isAndroid = document.body.classList.contains('android');
    if (isAndroid) {
        renderReadlistCards(container, items);
    } else {
        renderReadlistTableDesktop(container, items);
    }
}

function renderReadlistTableDesktop(container, items) {
    var html = '<div class="readlist-table-wrapper"><table class="books-table"><thead><tr>';
    html += '<th class="col-library">📖</th>';
    html += '<th class="col-date sortable" data-sort-by="created_at">Дата создания' + getReadlistSortIcon('created_at') + '</th>';
    html += '<th class="col-num sortable" data-sort-by="priority">Приоритет' + getReadlistSortIcon('priority') + '</th>';
    html += '<th class="col-title sortable" data-sort-by="bookname">Название книги' + getReadlistSortIcon('bookname') + '</th>';
    html += '<th class="col-author sortable" data-sort-by="author">Автор' + getReadlistSortIcon('author') + '</th>';
    html += '<th class="col-comment sortable" data-sort-by="comment">Комментарий' + getReadlistSortIcon('comment') + '</th>';
    html += '<th class="col-listname sortable" data-sort-by="listname">Список' + getReadlistSortIcon('listname') + '</th>';
    html += '<th class="col-status sortable" data-sort-by="status">Статус' + getReadlistSortIcon('status') + '</th>';
    html += '<th class="col-format">Формат</th>';
    html += '<th class="col-shelf">Полка</th>';
    html += '<th class="col-actions">Действия</th></tr></thead><tbody>';

    items.forEach(function(item) {
        var dateStr = item.created_at ? item.created_at.substring(0, 10) : '';
        var priority = item.priority;
        var bookname = escapeHtml(item.bookname || '');
        var authorname = escapeHtml(item.author || '');
        var comment = escapeHtml(item.comment || '');
        var listname = escapeHtml(item.listname || '');
        var st = item.status || 'Не заполнено';
        var hasBook = item.edition_id != null;
        var editionId = item.edition_id;
        var formatName = item.format_name || '';
        var onShelf = item.on_shelf;
        var shelfIcon = onShelf ? '★' : '☆';
        var shelfTitle = onShelf ? 'Убрать с полки' : 'Добавить на полку';

        html += '<tr data-id="' + item.id + '">';
        html += '<td class="col-library">' + (hasBook ? '✓' : '—') + '</td>';
        html += '<td class="col-date">' + escapeHtml(dateStr) + '</td>';
        html += '<td class="col-num">' + priority + '</td>';
        html += '<td class="col-title">' + bookname + '</td>';
        html += '<td class="col-author">' + authorname + '</td>';
        html += '<td class="col-comment">' + (comment || '—') + '</td>';
        html += '<td class="col-listname">' + (listname || '—') + '</td>';
        html += '<td class="col-status">' +
            '<select class="status-select ' + getStatusClass(st) + '" data-rlid="' + item.id + '">';
        ['Не заполнено','Прочитано','Читаю','Отложил','Бросил'].forEach(function(s) {
            html += '<option value="' + s + '"' + (s === st ? ' selected' : '') + '>' + s + '</option>';
        });
        html += '</select></td>';
        html += '<td class="col-format">';
        if (hasBook && formatName) {
            html += '<a href="#" class="readlist-download-link" data-edition-id="' + editionId + '">' + escapeHtml(formatName) + '</a>';
        } else {
            html += '—';
        }
        html += '</td>';
        html += '<td class="col-shelf">';
        if (hasBook) {
            html += '<a href="#" class="readlist-shelf-toggle" data-edition-id="' + editionId + '" data-on-shelf="' + onShelf + '" title="' + shelfTitle + '">' + shelfIcon + '</a>';
        } else {
            html += '—';
        }
        html += '</td>';
        html += '<td class="col-actions">';
        html += '<button class="btn btn-small edit-readlist-btn" data-id="' + item.id + '" title="Редактировать">✎</button>';
        html += '<button class="btn btn-small delete-readlist-btn" data-id="' + item.id + '" title="Удалить">✕</button>';
        html += '</td></tr>';
    });

    html += '</tbody></table></div>';
    container.innerHTML = html;

    container.querySelectorAll('th.sortable').forEach(function(th) {
        th.addEventListener('click', function(e) {
            var sortBy = this.dataset.sortBy;
            if (sortBy) {
                if (readlistSortBy === sortBy) {
                    readlistSortOrder = readlistSortOrder === 'asc' ? 'desc' : 'asc';
                } else {
                    readlistSortBy = sortBy;
                    readlistSortOrder = 'asc';
                }
                readlistPage = 1;
                loadReadlist();
            }
        });
    });
}

function renderReadlistCards(container, items) {
    var html = '<div class="readlist-cards">';
    items.forEach(function(item) {
        var dateStr = item.created_at ? item.created_at.substring(0, 10) : '';
        var priority = item.priority;
        var bookname = escapeHtml(item.bookname || '');
        var authorname = escapeHtml(item.author || '');
        var comment = escapeHtml(item.comment || '');
        var hasBook = item.edition_id != null;
        var editionId = item.edition_id;
        var formatName = item.format_name || '';
        var onShelf = item.on_shelf;
        var shelfIcon = onShelf ? '★' : '☆';
        var shelfTitle = onShelf ? 'Убрать с полки' : 'Добавить на полку';

        html += '<div class="readlist-card" data-id="' + item.id + '">';
        // Top row: Priority, Date, Library flag
        html += '<div class="rl-card-top">';
        html += '<span class="rl-card-priority">#' + priority + '</span>';
        html += '<span class="rl-card-date">' + escapeHtml(dateStr) + '</span>';
        html += '<span class="rl-card-library">' + (hasBook ? '✓' : '—') + '</span>';
        html += '</div>';
        // Title
        html += '<div class="rl-card-title">' + bookname + '</div>';
        // Author
        html += '<div class="rl-card-author">' + authorname + '</div>';
        // Comment (only if filled)
        if (comment) {
            html += '<div class="rl-card-comment">' + comment + '</div>';
        }
        // Actions - always show edit/delete, format/shelf only if has book
        html += '<div class="rl-card-actions">';
        if (hasBook) {
            if (formatName) {
                html += '<a href="#" class="readlist-download-link" data-edition-id="' + editionId + '" title="Скачать">📥 ' + escapeHtml(formatName) + '</a>';
            }
            html += '<a href="#" class="readlist-shelf-toggle" data-edition-id="' + editionId + '" data-on-shelf="' + onShelf + '" title="' + shelfTitle + '">' + shelfIcon + ' Полка</a>';
        }
        html += '<button class="edit-readlist-btn" data-id="' + item.id + '" title="Редактировать">✎</button>';
        html += '<button class="delete-readlist-btn" data-id="' + item.id + '" title="Удалить">🗑</button>';
        html += '</div>';
        html += '</div>';
    });
    html += '</div>';
    container.innerHTML = html;
}

function getReadlistSortIcon(sortBy) {
    if (readlistSortBy !== sortBy) return ' ↕';
    return readlistSortOrder === 'asc' ? ' ▲' : ' ▼';
}

function renderReadlistPagination(total, page, limit) {
    var totalPages = Math.ceil(total / limit);
    var topEl = document.getElementById('readlistPagination');
    var bottomEl = document.getElementById('readlistPaginationBottom');
    if (!topEl) return;

    if (totalPages <= 1) {
        topEl.innerHTML = '<span class="page-info">' + total + ' записей</span>';
        if (bottomEl) bottomEl.innerHTML = '';
        return;
    }

    var html = '<span class="page-info">' + total + ' записей, стр. ' + page + ' из ' + totalPages + '</span>';
    if (page > 1) {
        html += '<button class="pagination-btn" data-page="1">&laquo;</button>';
        html += '<button class="pagination-btn" data-page="' + (page - 1) + '">&lsaquo;</button>';
    }
    var start = Math.max(1, page - 2);
    var end = Math.min(totalPages, page + 2);
    for (var i = start; i <= end; i++) {
        html += '<button class="pagination-btn' + (i === page ? ' active' : '') + '" data-page="' + i + '">' + i + '</button>';
    }
    if (page < totalPages) {
        html += '<button class="pagination-btn" data-page="' + (page + 1) + '">&rsaquo;</button>';
        html += '<button class="pagination-btn" data-page="' + totalPages + '">&raquo;</button>';
    }

    topEl.innerHTML = html;
    if (bottomEl) bottomEl.innerHTML = html;
}

function setupReadlistEvents() {
    var container = document.getElementById('readlistTableContainer');
    if (!container) return;

    // Init default status filter: all except Прочитано
    var statusEl = document.getElementById('readlistStatusFilter');
    if (statusEl && !statusEl.value) {
        statusEl.value = 'Не заполнено,Читаю,Отложил,Бросил';
    }

    // Filter clear
    document.getElementById('clearReadlistFilters')?.addEventListener('click', function() {
        document.getElementById('readlistBookFilter').value = '';
        document.getElementById('readlistAuthorFilter').value = '';
        document.getElementById('readlistCommentFilter').value = '';
        document.getElementById('readlistNameFilter').value = '';
        document.getElementById('readlistStatusFilter').value = 'Не заполнено,Читаю,Отложил,Бросил';
        updateReadlistFilterBtnText();
        readlistPage = 1;
        loadReadlist();
    });

    // Enter key and blur in filters
    ['readlistBookFilter', 'readlistAuthorFilter', 'readlistCommentFilter'].forEach(function(id) {
        var input = document.getElementById(id);
        if (input) {
            input.addEventListener('keypress', function(e) {
                if (e.key === 'Enter') {
                    e.preventDefault();
                    readlistPage = 1;
                    loadReadlist();
                }
            });
            input.addEventListener('blur', function() {
                readlistPage = 1;
                loadReadlist();
            });
        }
    });

    // Listname filter change
    document.getElementById('readlistNameFilter')?.addEventListener('change', function() {
        readlistPage = 1;
        loadReadlist();
    });

    // Create button
    document.getElementById('createReadlistBtn')?.addEventListener('click', function() {
        openCreateReadlistModal();
    });

    // Filter button
    document.getElementById('readlistFilterBtn')?.addEventListener('click', function() {
        openReadlistFilterModal();
    });

    // Filter modal Apply
    document.getElementById('rlFilterApply')?.addEventListener('click', function() {
        applyReadlistFilter();
    });

    // Filter modal Clear
    document.getElementById('rlFilterClear')?.addEventListener('click', function() {
        clearReadlistFilter();
    });

    // Filter modal Cancel — close via close-rlfilter-btn class
    document.getElementById('readlistFilterModal')?.addEventListener('click', function(e) {
        if (e.target.classList.contains('close-rlfilter-btn') || e.target === this) {
            closeReadlistFilterModal();
        }
    });

    // Author text input → filter select
    var rlAuthorInput = document.getElementById('rlAuthor');
    var rlAuthorSelect = document.getElementById('rlAuthorSelect');
    if (rlAuthorInput) {
        rlAuthorInput.addEventListener('input', function() {
            var val = this.value.toLowerCase();
            var opts = rlAuthorSelect.options;
            var matched = [];
            for (var i = 0; i < opts.length; i++) {
                if (opts[i].value === '') continue;
                var matches = opts[i].textContent.toLowerCase().indexOf(val) !== -1;
                opts[i].style.display = matches ? '' : 'none';
                if (matches) matched.push(opts[i]);
            }
            document.getElementById('rlAuthorId').value = '';
            rlAuthorSelect.style.display = (val && matched.length > 0) ? '' : 'none';
        });
        rlAuthorInput.addEventListener('blur', function() {
            var val = this.value.toLowerCase();
            var opts = rlAuthorSelect.options;
            var matched = [];
            for (var i = 0; i < opts.length; i++) {
                if (opts[i].value === '') continue;
                if (opts[i].textContent.toLowerCase().indexOf(val) !== -1) {
                    matched.push(opts[i]);
                }
            }
            if (matched.length === 1) {
                matched[0].selected = true;
                document.getElementById('rlAuthorId').value = matched[0].value;
                this.value = matched[0].textContent;
                rlAuthorSelect.style.display = 'none';
            }
        });
    }
    // Author select change → fill text
    if (rlAuthorSelect) {
        rlAuthorSelect.addEventListener('change', function() {
            if (this.value) {
                document.getElementById('rlAuthor').value = this.options[this.selectedIndex].textContent;
                document.getElementById('rlAuthorId').value = this.value;
            } else {
                document.getElementById('rlAuthorId').value = '';
            }
        });
    }

    // Book text input → filter select
    var rlBookInput = document.getElementById('rlBookname');
    var rlBookSelect = document.getElementById('rlBookSelect');
    var rlBookSelectedValue = ''; // tracks actual selected book id
    if (rlBookInput) {
        rlBookInput.addEventListener('input', function() {
            var val = this.value.toLowerCase();
            var opts = rlBookSelect.options;
            var matched = [];
            for (var i = 0; i < opts.length; i++) {
                if (opts[i].value === '') continue;
                var matches = opts[i].textContent.toLowerCase().indexOf(val) !== -1;
                opts[i].style.display = matches ? '' : 'none';
                if (matches) matched.push(opts[i]);
            }
            rlBookSelectedValue = '';
            document.getElementById('rlBookId').value = '';
            rlBookSelect.style.display = (val && matched.length > 0) ? '' : 'none';
        });
        rlBookInput.addEventListener('keydown', function(e) {
            if (e.key === 'Enter') {
                var val = this.value.toLowerCase();
                if (!val) return;
                var opts = rlBookSelect.options;
                var matched = [];
                for (var i = 0; i < opts.length; i++) {
                    if (opts[i].value === '') continue;
                    if (opts[i].textContent.toLowerCase().indexOf(val) !== -1) {
                        matched.push(opts[i]);
                    }
                }
                if (matched.length === 1) {
                    e.preventDefault();
                    fillBookSelection(matched[0]);
                }
            }
        });
        rlBookInput.addEventListener('blur', function() {
            var val = this.value.toLowerCase();
            if (!val) return;
            var opts = rlBookSelect.options;
            var matched = [];
            for (var i = 0; i < opts.length; i++) {
                if (opts[i].value === '') continue;
                if (opts[i].textContent.toLowerCase().indexOf(val) !== -1) {
                    matched.push(opts[i]);
                }
            }
            if (matched.length === 1) {
                fillBookSelection(matched[0]);
            }
        });
    }
    if (rlBookSelect) {
        rlBookSelect.addEventListener('change', function() {
            if (this.value) {
                fillBookSelection(this.options[this.selectedIndex]);
            } else {
                document.getElementById('rlBookId').value = '';
            }
        });
    }
    function fillBookSelection(opt) {
        opt.selected = true;
        rlBookSelectedValue = opt.value;
        document.getElementById('rlBookname').value = opt.dataset.title || opt.textContent;
        document.getElementById('rlBookId').value = opt.value;
        var firstAuthor = opt.dataset.firstAuthor || '';
        if (firstAuthor) {
            document.getElementById('rlAuthor').value = firstAuthor;
            var aid = personMap[firstAuthor.toLowerCase()];
            document.getElementById('rlAuthorId').value = aid || '';
        }
        rlBookSelect.style.display = 'none';
    }

    // Create/Edit form submit
    document.getElementById('readlistForm')?.addEventListener('submit', async function(e) {
        e.preventDefault();
        var editId = document.getElementById('rlEditId').value;
        var authorIdVal = document.getElementById('rlAuthorId').value;
        var authorId = authorIdVal ? parseInt(authorIdVal) : null;
        var bookIdVal = document.getElementById('rlBookId').value;
        var bookId = bookIdVal ? parseInt(bookIdVal) : null;
        var data = {
            listname: document.getElementById('rlListname').value.trim(),
            bookname: document.getElementById('rlBookname').value.trim(),
            author: document.getElementById('rlAuthor').value.trim(),
            priority: parseInt(document.getElementById('rlPriority').value) || 0,
            author_id: authorId,
            book_id: bookId,
            comment: document.getElementById('rlComment').value.trim(),
            status: document.getElementById('rlStatus').value
        };

        if (!data.listname) { alert('Укажите название списка'); return; }

        // Android: always write to local store, sync in background
        if (isAndroid() && window.ReadListStore) {
            var id = editId || (crypto.randomUUID ? crypto.randomUUID() : 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) { var r = Math.random()*16|0, v = c=='x'?r:(r&0x3|0x8); return v.toString(16); }));
            var now = new Date().toISOString();
            var existing = editId ? ReadListStore.getById(editId) : null;

            // Auto-assign priority: max existing + 1 (same as server-side logic)
            if (!editId && (!data.priority || data.priority <= 0)) {
                var allLocal = ReadListStore.getAll();
                var maxP = 0;
                for (var pi = 0; pi < allLocal.length; pi++) {
                    if (allLocal[pi].priority > maxP) maxP = allLocal[pi].priority;
                }
                data.priority = maxP + 1;
            }

            var item = {
                id: editId || id,
                listname: data.listname,
                bookname: data.bookname,
                author: data.author,
                priority: data.priority,
                author_id: data.author_id,
                book_id: data.book_id,
                comment: data.comment,
                status: data.status,
                created_at: existing ? existing.created_at : now,
                updated_at: now,
                synced_at: existing ? existing.synced_at : null,
                format_name: existing ? (existing.format_name||'') : '',
                on_shelf: existing ? (existing.on_shelf||false) : false,
                user_id: existing ? (existing.user_id||0) : 0,
                edition_id: existing ? (existing.edition_id||null) : null,
                deleted: existing ? (existing.deleted||false) : false
            };
            ReadListStore.upsert(item);
            closeReadlistModal();
            loadReadlist();
            loadReadlistNames();
            if (window.SyncService) SyncService.sync(true);
            return;
        }

        try {
            var url = RL_API;
            var method = 'POST';
            if (editId) {
                url += '/' + editId;
                method = 'PUT';
            }
            var res = await apiFetch(url, {
                method: method,
                headers: getAuthHeaders(),
                body: JSON.stringify(data)
            });
            if (res.status === 401) { handleAuthFailure(); return; }
            if (!res.ok) {
                var err = await res.json();
                alert('Ошибка: ' + (err.error || 'Неизвестная ошибка'));
                return;
            }
            closeReadlistModal();
            loadReadlist();
            loadReadlistNames();
        } catch (err) {
            alert('Ошибка: ' + err.message);
        }
    });

    // Double-click on row → edit
    container.addEventListener('dblclick', function(e) {
        var row = e.target.closest('tr[data-id]');
        if (row) {
            openEditReadlistModal(row.dataset.id);
        }
    });

    // Event delegation for table actions
    container.addEventListener('click', function(e) {
        var editBtn = e.target.closest('.edit-readlist-btn');
        if (editBtn) {
            e.stopPropagation();
            openEditReadlistModal(editBtn.dataset.id);
            return;
        }

        var deleteBtn = e.target.closest('.delete-readlist-btn');
        if (deleteBtn) {
            e.preventDefault();
            var id = deleteBtn.dataset.id;
            if (!confirm('Удалить запись из списка чтения?')) return;
            // Android: soft delete locally, sync in background
            if (isAndroid() && window.ReadListStore) {
                var item = ReadListStore.getById(id);
                if (item) {
                    item.deleted = true;
                    ReadListStore.upsert(item);
                }
                loadReadlist();
                loadReadlistNames();
                if (window.SyncService) SyncService.sync(true);
                return;
            }
            (async function() {
                try {
                    var r = await apiFetch(RL_API + '/' + id, { method: 'DELETE', headers: getAuthHeaders() });
                    if (r.status === 401) { handleAuthFailure(); return; }
                    if (r.ok) {
                        loadReadlist();
                        loadReadlistNames();
                    } else {
                        var err = await r.json();
                        alert('Ошибка: ' + (err.error || 'Неизвестная ошибка'));
                    }
                } catch (err) { alert('Ошибка: ' + err.message); }
            })();
        }

        var downloadLink = e.target.closest('.readlist-download-link');
        if (downloadLink) {
            e.preventDefault();
            var eid = downloadLink.dataset.editionId;
            triggerDownload(API_BASE + '/books/' + eid + '/download');
            return;
        }

        var shelfEl = e.target.closest('.readlist-shelf-toggle');
        if (shelfEl) {
            e.preventDefault();
            var eid = shelfEl.dataset.editionId;
            var onShelf = shelfEl.dataset.on_shelf === 'true';
            (async function() {
                try {
                    var r = await apiFetch(API_BASE + '/books/' + eid + '/shelf', {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ on_shelf: !onShelf })
                    });
                    if (r.ok) {
                        shelfEl.dataset.on_shelf = !onShelf;
                        shelfEl.textContent = !onShelf ? '★' : '☆';
                        shelfEl.title = !onShelf ? 'Убрать с полки' : 'Добавить на полку';
                        updateShelfCount();
                    }
                } catch (err) { console.error(err); }
            })();
            return;
        }

        var pageBtn = e.target.closest('.pagination-btn');
        if (pageBtn) {
            var p = parseInt(pageBtn.dataset.page);
            if (p !== readlistPage) {
                readlistPage = p;
                loadReadlist();
                window.scrollTo({ top: document.querySelector('.filters').offsetTop, behavior: 'smooth' });
            }
        }
    });

    // Sync button
    document.getElementById('syncReadlistBtn')?.addEventListener('click', function() {
        if (window.SyncService) SyncService.sync(true);
    });

    // Update offline UI periodically
    if (isAndroid()) {
        setInterval(updateOfflineUI, 5000);
        updateOfflineUI();
    }
}

function updateOfflineUI() {
    var syncBtn = document.getElementById('syncReadlistBtn');
    var pendingEl = document.getElementById('pendingCount');
    if (!window.ReadListStore) {
        if (syncBtn) syncBtn.style.display = 'none';
        if (pendingEl) pendingEl.style.display = 'none';
        return;
    }
    var count = ReadListStore.countDirty();
    if (syncBtn) syncBtn.style.display = count > 0 && isOnline() ? 'inline-block' : 'none';
    if (pendingEl) {
        if (count > 0) {
            pendingEl.style.display = 'inline';
            pendingEl.textContent = '\u23F3 ' + count + ' \u043E\u0436\u0438\u0434\u0430\u044E\u0442 \u0441\u0438\u043D\u0445\u0440\u043E\u043D\u0438\u0437\u0430\u0446\u0438\u0438';
        } else {
            pendingEl.style.display = 'none';
        }
    }
}

function populateListnameSelect(select, currentVal) {
    if (!select) return;
    select.innerHTML = '<option value="">Все списки</option>';
    var seen = {};
    var hiddenSelect = document.getElementById('readlistNameFilter');
    if (hiddenSelect) {
        for (var i = 0; i < hiddenSelect.options.length; i++) {
            var opt = hiddenSelect.options[i];
            if (opt.value === '') continue;
            if (!seen[opt.value]) {
                seen[opt.value] = true;
                var newOpt = document.createElement('option');
                newOpt.value = opt.value;
                newOpt.textContent = opt.textContent;
                select.appendChild(newOpt);
            }
        }
    }
    if (currentVal) select.value = currentVal;
}

async function loadReadlistNames() {
    // Android: get names from local store
    if (isAndroid() && window.ReadListStore) {
        ReadListStore._ensureCache();
        var names = [];
        var seen = {};
        (ReadListStore._cache || []).forEach(function(item) {
            if (item.listname && !seen[item.listname]) {
                seen[item.listname] = true;
                names.push(item.listname);
            }
        });
        populateReadlistNames(names);
        return;
    }
    try {
        var res = await apiFetch(RL_API + '/names', { headers: getAuthHeaders() });
        if (!res.ok) return;
        var names = await res.json();
        populateReadlistNames(names);
    } catch(e) {}
}

function populateReadlistNames(names) {
    var select = document.getElementById('readlistNameFilter');
    if (!select) return;
    var currentVal = select.value;
    select.innerHTML = '<option value="">Все списки</option>';
    var seen = {};
    names.forEach(function(n) {
        if (!seen[n]) {
            seen[n] = true;
            var opt = document.createElement('option');
            opt.value = n;
            opt.textContent = n;
            select.appendChild(opt);
        }
    });
    if (currentVal) select.value = currentVal;
    populateListnameSelect(document.getElementById('rlFilterListname'), currentVal);
    updateReadlistFilterBtnText();
}

async function loadAuthorSelect() {
    var select = document.getElementById('rlAuthorSelect');
    if (!select) return;
    try {
        var res = await apiFetch(API_BASE + '/persons');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        var persons = await res.json();
        personMap = {};
        select.innerHTML = '<option value="">— выберите автора —</option>';
        persons.forEach(function(p) {
            var name = p.last_name + ' ' + p.first_name;
            personMap[name.trim().toLowerCase()] = p.id;
            var opt = document.createElement('option');
            opt.value = p.id;
            opt.textContent = name;
            select.appendChild(opt);
        });
    } catch(e) {
        select.innerHTML = '<option value="">— ошибка загрузки —</option>';
    }
}

async function loadBookSelect() {
    var select = document.getElementById('rlBookSelect');
    if (!select) return;
    try {
        var res = await apiFetch(API_BASE + '/books?limit=9999');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        var data = await res.json();
        var books = data.books || [];
        select.innerHTML = '<option value="">— выберите книгу —</option>';
        var seen = {};
        books.forEach(function(b) {
            var title = (b.edition_title || b.original_title || '').trim();
            if (!title || seen[title]) return;
            seen[title] = true;
            var opt = document.createElement('option');
            opt.value = b.edition_id;
            opt.dataset.title = title;
            var author = '';
            if (typeof b.authors === 'string') {
                author = b.authors;
            } else if (b.authors && typeof b.authors.String === 'string') {
                author = b.authors.String;
            }
            var firstAuthor = author ? author.split(',')[0].trim() : '';
            opt.dataset.firstAuthor = firstAuthor;
            opt.textContent = title + (author ? ' (' + author + ')' : '');
            select.appendChild(opt);
        });
    } catch(e) {
        select.innerHTML = '<option value="">— ошибка загрузки —</option>';
    }
}

function openCreateReadlistModal() {
    document.getElementById('readlistModalTitle').textContent = 'Новая запись';
    document.getElementById('rlEditId').value = '';
    document.getElementById('rlListname').value = document.getElementById('readlistNameFilter').value || 'default';
    document.getElementById('rlBookname').value = '';
    document.getElementById('rlBookId').value = '';
    document.getElementById('rlBookSelect').style.display = 'none';
    document.getElementById('rlAuthor').value = '';
    document.getElementById('rlAuthorId').value = '';
    document.getElementById('rlAuthorSelect').style.display = 'none';
    document.getElementById('rlPriority').value = '0';
    document.getElementById('rlComment').value = '';
    document.getElementById('rlStatus').value = 'Не заполнено';
    document.getElementById('readlistModal').style.display = 'block';
    document.getElementById('readlistModal').classList.add('active');
    loadAuthorSelect();
    loadBookSelect();
}

function updateReadlistFilterBtnText() {
    var btn = document.getElementById('readlistFilterBtn');
    if (!btn) return;
    var listname = document.getElementById('readlistNameFilter').value;
    var statusFilter = document.getElementById('readlistStatusFilter').value;
    var parts = [];
    parts.push(listname || 'Все списки');
    if (statusFilter) {
        var statuses = statusFilter.split(',');
        parts.push('статус: ' + statuses.join(', '));
    } else {
        parts.push('статус: все');
    }
    btn.textContent = parts.join(' | ');
}

function openReadlistFilterModal() {
    // Populate listname dropdown from the hidden select
    var hiddenSelect = document.getElementById('readlistNameFilter');
    var modalSelect = document.getElementById('rlFilterListname');
    modalSelect.innerHTML = '<option value="">Все списки</option>';
    for (var i = 0; i < hiddenSelect.options.length; i++) {
        var opt = hiddenSelect.options[i];
        if (opt.value === '') continue;
        var newOpt = document.createElement('option');
        newOpt.value = opt.value;
        newOpt.textContent = opt.textContent;
        modalSelect.appendChild(newOpt);
    }
    modalSelect.value = hiddenSelect.value;

    // Populate status checkboxes from readlistStatusFilter
    var statusFilterVal = document.getElementById('readlistStatusFilter').value;
    var activeStatuses = {};
    if (statusFilterVal) {
        statusFilterVal.split(',').forEach(function(s) { activeStatuses[s.trim()] = true; });
    }
    var checkboxes = document.querySelectorAll('#rlFilterStatuses input[type="checkbox"]');
    checkboxes.forEach(function(cb) {
        cb.checked = !!activeStatuses[cb.value];
    });

    // Populate book/author fields from existing filters
    document.getElementById('rlFilterBook').value = document.getElementById('readlistBookFilter').value;
    document.getElementById('rlFilterAuthor').value = document.getElementById('readlistAuthorFilter').value;

    // Show modal
    document.getElementById('readlistFilterModal').style.display = 'block';
    document.getElementById('readlistFilterModal').classList.add('active');
}

function closeReadlistFilterModal() {
    document.getElementById('readlistFilterModal').style.display = 'none';
    document.getElementById('readlistFilterModal').classList.remove('active');
}

function applyReadlistFilter() {
    // Read listname
    var listname = document.getElementById('rlFilterListname').value;
    document.getElementById('readlistNameFilter').value = listname;

    // Read status checkboxes
    var selectedStatuses = [];
    document.querySelectorAll('#rlFilterStatuses input[type="checkbox"]').forEach(function(cb) {
        if (cb.checked) selectedStatuses.push(cb.value);
    });
    document.getElementById('readlistStatusFilter').value = selectedStatuses.join(',');

    // Read book/author fields
    document.getElementById('readlistBookFilter').value = document.getElementById('rlFilterBook').value;
    document.getElementById('readlistAuthorFilter').value = document.getElementById('rlFilterAuthor').value;

    // Update button text
    updateReadlistFilterBtnText();

    // Close modal and reload
    closeReadlistFilterModal();
    readlistPage = 1;
    loadReadlist();
}

function clearReadlistFilter() {
    // Reset listname to empty
    document.getElementById('readlistNameFilter').value = '';

    // Reset status to all except "Прочитано"
    document.getElementById('readlistStatusFilter').value = 'Не заполнено,Читаю,Отложил,Бросил';

    // Clear text filters
    document.getElementById('readlistBookFilter').value = '';
    document.getElementById('readlistAuthorFilter').value = '';

    // Update button text
    updateReadlistFilterBtnText();

    // Close modal and reload
    closeReadlistFilterModal();
    readlistPage = 1;
    loadReadlist();
}

async function openEditReadlistModal(id) {
    // Android: get from local store
    if (isAndroid() && window.ReadListStore) {
        var item = ReadListStore.getById(id);
        if (!item) { alert('Запись не найдена в локальном хранилище'); return; }
        fillReadlistEditForm(item);
        return;
    }
    try {
        var allRes = await apiFetch(RL_API + '?limit=9999', { headers: getAuthHeaders() });
        if (!allRes.ok) return;
        var data = await allRes.json();
        var items = data.items || [];
        var item = null;
        for (var i = 0; i < items.length; i++) {
            if (items[i].id == id) { item = items[i]; break; }
        }
        if (!item) { alert('Запись не найдена'); return; }
        fillReadlistEditForm(item);
    } catch (err) {
        alert('Ошибка: ' + err.message);
    }
}

function fillReadlistEditForm(item) {
    document.getElementById('readlistModalTitle').textContent = 'Редактирование записи';
    document.getElementById('rlEditId').value = item.id;
    document.getElementById('rlListname').value = item.listname || 'default';
    document.getElementById('rlBookname').value = item.bookname || '';
    document.getElementById('rlBookId').value = item.book_id || '';
    document.getElementById('rlBookSelect').style.display = 'none';
    document.getElementById('rlAuthor').value = item.author || '';
    document.getElementById('rlAuthorId').value = item.author_id || '';
    document.getElementById('rlAuthorSelect').style.display = 'none';
    document.getElementById('rlPriority').value = item.priority || 0;
    document.getElementById('rlComment').value = item.comment || '';
    document.getElementById('rlStatus').value = item.status || 'Не заполнено';
    document.getElementById('readlistModal').style.display = 'block';
    document.getElementById('readlistModal').classList.add('active');
    loadAuthorSelect().then(function() {
        if (item.author_id) {
            var sel = document.getElementById('rlAuthorSelect');
            for (var i = 0; i < sel.options.length; i++) {
                if (sel.options[i].value == item.author_id) {
                    sel.value = item.author_id;
                    break;
                }
            }
        }
    });
    loadBookSelect().then(function() {
        if (item.book_id) {
            var sel = document.getElementById('rlBookSelect');
            for (var i = 0; i < sel.options.length; i++) {
                if (sel.options[i].value == item.book_id) {
                    sel.value = item.book_id;
                    break;
                }
            }
        }
    });
}

function closeReadlistModal() {
    var modal = document.getElementById('readlistModal');
    modal.style.display = 'none';
    modal.classList.remove('active');
}

async function setReadlistItemStatus(id, status) {
    var token = localStorage.getItem('auth_token');
    if (!token) { promptLogin(); return; }
    // Android: update locally, sync in background
    if (isAndroid() && window.ReadListStore) {
        var item = ReadListStore.getById(id);
        if (item) {
            item.status = status;
            ReadListStore.upsert(item);
            loadReadlist();
            if (window.SyncService) SyncService.sync(true);
        }
        return;
    }
    try {
        var allRes = await apiFetch(RL_API + '?limit=9999', { headers: getAuthHeaders() });
        if (!allRes.ok) { if (allRes.status === 401) handleAuthFailure(); return; }
        var data = await allRes.json();
        var items = data.items || [];
        var item = null;
        for (var i = 0; i < items.length; i++) {
            if (items[i].id == id) { item = items[i]; break; }
        }
        if (!item) return;
        item.status = status;
        var res = await apiFetch(RL_API + '/' + id, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(item)
        });
        if (res.status === 401) { handleAuthFailure(); return; }
        if (!res.ok) {
            var err = await res.json();
            alert('Ошибка: ' + (err.error || ''));
        } else {
            loadReadlist();
            loadReadlistNames();
        }
    } catch(e) {}
}

// Event delegation for CSP compliance (no inline onclick/onchange)
document.addEventListener('change', function(e) {
    if (e.target.id === 'bookStatusFilter') loadBooks();
    var rlSelect = e.target.closest('.status-select[data-rlid]');
    if (rlSelect) {
        setReadlistItemStatus(rlSelect.dataset.rlid, rlSelect.value);
    }
});

document.addEventListener('click', function(e) {
    var target = e.target.closest('.close-readlist-btn');
    if (target) { e.preventDefault(); closeReadlistModal(); return; }
    target = e.target.closest('.close-modal-btn');
    if (target) { e.preventDefault(); closeModal(); return; }
});