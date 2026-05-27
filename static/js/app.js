const API_BASE = '/api/v1';
let enableDelete = false;
let authorsPage = 1;
let booksPage = 1;
let booksSortBy = 'original_title';
let booksSortOrder = 'asc';

// Stubs - replaced by real functions from import.js when it loads
var showImportProgress = function(dirPath, total) {};
var startImportPolling = function() {};
var switchToImportTab = function() {};

document.getElementById('loadBooksBtn')?.addEventListener('click', () => {
    document.getElementById('folderInput').click();
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

        const response = await fetch(API_BASE + '/import/upload', {
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
        if (tab.dataset.tab === 'books') {
            loadBooks();
        }
    });
});

document.getElementById('applyFilters').addEventListener('click', () => {
    authorsPage = 1;
    loadAuthors();
});

document.getElementById('clearFilters').addEventListener('click', () => {
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
            const response = await fetch(`${API_BASE}/books/${id}/shelf`, {
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
        const response = await fetch(`${API_BASE}/shelf/clear`, { method: 'PUT' });
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
        const response = await fetch(`${API_BASE}/shelf/clear`, { method: 'PUT' });
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
    input.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            loadAuthors();
        }
    });
});

document.getElementById('applyBookFilters')?.addEventListener('click', () => {
    booksPage = 1;
    loadBooks();
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
    }
});

document.getElementById('editForm').addEventListener('submit', async (e) => {
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

            const response = await fetch(`${API_BASE}/persons/${id}`, {
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
            const response = await fetch(`${API_BASE}/books/${id}/extended`, {
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
            state.keepFocus = { type: 'book', id: id };
            loadAuthorsWithState(state);
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
            const select = row.querySelector('.author-select');
            const roleSelect = row.querySelector('.author-role');
            if (select && select.value) {
                authors.push({
                    id: parseInt(select.value),
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
    loadAuthors();
    updateShelfCount();
    fetchConfig();
    setupBooksTableEvents();
});

async function fetchConfig() {
    try {
        const res = await fetch(`${API_BASE}/config`);
        const data = await res.json();
        enableDelete = data.enable_delete === true;
    } catch (e) {
        console.error('Error loading config:', e);
    }
}

async function updateShelfCount() {
    try {
        const response = await fetch(`${API_BASE}/shelf/count`);
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
        const response = await fetch(`${API_BASE}/authors?author=${encodeURIComponent(authorFilter)}&book=${encodeURIComponent(bookFilter)}&genre=${encodeURIComponent(genreFilter)}&page=${page}&limit=50`);

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

    fetch(`${API_BASE}/authors?author=${encodeURIComponent(authorFilter)}&book=${encodeURIComponent(bookFilter)}&genre=${encodeURIComponent(genreFilter)}&page=${page}&limit=50`)
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
        const response = await fetch(url);
        if (!response.ok) throw new Error('HTTP ' + response.status);
        const data = await response.json();
        renderBooksTable(data);
        renderBooksPagination(data.total, booksPage, limit);
    } catch (err) {
        container.innerHTML = `<div class="error">Ошибка загрузки книг: ${escapeHtml(err.message)}</div>`;
    }
}

function renderBooksTable(data) {
    const container = document.getElementById('booksTableContainer');
    const books = data.books || [];

    if (books.length === 0) {
        container.innerHTML = '<div class="empty">Книги не найдены</div>';
        return;
    }

    let html = '<table class="books-table"><thead><tr>';
    html += '<th class="col-num">#</th>';
    html += '<th class="col-date sortable" data-sort-by="upload_date">Дата загрузки' + getSortIcon('upload_date') + '</th>';
    html += '<th class="col-title sortable" data-sort-by="original_title">Название' + getSortIcon('original_title') + '</th>';
    html += '<th class="col-author sortable" data-sort-by="authors">Автор' + getSortIcon('authors') + '</th>';
    html += '<th class="col-format sortable" data-sort-by="available_formats">Формат' + getSortIcon('available_formats') + '</th>';
    html += '<th class="col-shelf">Полка</th>';
    html += '<th class="col-actions">Действия</th></tr></thead><tbody>';

    books.forEach((book, index) => {
        const rowNum = (booksPage - 1) * 50 + index + 1;
        const dateStr = book.upload_date ? book.upload_date.substring(0, 10) : '';
        const title = book.original_title || book.edition_title || '';
        const authorName = getNullableValue(book, 'authors');
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
        html += `<td class="col-format"><a href="#" class="book-download-link" data-id="${editionId}">${escapeHtml(formatName)}</a></td>`;
        html += `<td class="col-shelf"><a href="#" class="shelf-toggle" data-id="${editionId}" data-on-shelf="${onShelf}" title="${shelfTitle}">${shelfIcon}</a></td>`;
        html += `<td class="col-actions">`;
        html += `<button class="btn btn-small edit-book-btn" data-id="${editionId}">✎</button>`;
        html += `</td></tr>`;
    });

    html += '</tbody></table>';
    container.innerHTML = html;
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
    const container = document.getElementById('books');
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

        const shelfEl = e.target.closest('.shelf-toggle');
        if (shelfEl) {
            e.preventDefault();
            const id = shelfEl.dataset.id;
            const onShelf = shelfEl.dataset.on_shelf === 'true';
            (async () => {
                try {
                    const r = await fetch(`${API_BASE}/books/${id}/shelf`, {
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
            window.location.href = `${API_BASE}/books/${downloadLink.dataset.id}/download`;
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
            <span style="color: #666; font-size: 12px;">(${author.books_count} книг)</span>
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
                        const response = await fetch(`${API_BASE}/books/${editionId}/shelf`, {
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
                            const res = await fetch(`${API_BASE}/books/${editionId}`, {
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
                    window.location.href = `${API_BASE}/books/${editionId}/download`;
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

function openAuthorModal(author) {
    const modal = document.getElementById('editModal');
    const modalTitle = document.getElementById('modalTitle');
    const modalBody = document.getElementById('modalBody');

    modalTitle.textContent = 'Редактирование автора';
    modalTitle.dataset.type = 'author';
    modalTitle.dataset.id = author.id;

    modalBody.innerHTML = `
        <form id="editForm">
            <div class="form-group">
                <label for="first_name">Имя:</label>
                <input type="text" id="first_name" name="first_name" value="${escapeHtml(author.first_name || '')}">
            </div>
            <div class="form-group">
                <label for="last_name">Фамилия:</label>
                <input type="text" id="last_name" name="last_name" value="${escapeHtml(author.last_name || '')}" required>
            </div>
            <div class="form-actions">
                <button type="submit" class="btn">Сохранить</button>
                <button type="button" class="btn btn-secondary" onclick="closeModal()">Отмена</button>
            </div>
        </form>
    `;

    modal.classList.add('active');
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
        const response = await fetch(`${API_BASE}/books/${book.id}/extended`);
        if (!response.ok) throw new Error('Ошибка загрузки данных');
        const data = await response.json();

        const [genresRes, tagsRes, personsRes, languagesRes] = await Promise.all([
            fetch(`${API_BASE}/genres`),
            fetch(`${API_BASE}/tags`),
            fetch(`${API_BASE}/persons`),
            fetch(`${API_BASE}/languages`)
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
            const authorOptionHtml = `<option value="${author.id}" selected>${escapeHtml(author.last_name)} ${escapeHtml(author.first_name || '')}</option>`;
            const fullPersonsOptions = personsOptions.replace('<option value="">-- Выберите --</option>', `<option value="">-- Выберите --</option>${authorOptionHtml}`);
            authorsHtml += `
                <div class="author-row" data-idx="${idx}">
                    <select name="author_id_${idx}" class="author-select">${fullPersonsOptions}</select>
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
        <form id="editForm">
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
            <legend>Издание (Edition)</legend>
            <div class="form-group">
                <label for="edition_title">Название издания:</label>
                <input type="text" id="edition_title" name="edition_title" value="${escapeHtml(editionTitle)}">
            </div>
            <div class="form-row">
                <div class="form-group" style="flex: 1;">
                    <label for="isbn">ISBN:</label>
                    <input type="text" id="isbn" name="isbn" value="${escapeHtml(getNullableValue(edition, 'isbn'))}">
                </div>
                <div class="form-group" style="flex: 1;">
                    <label for="ean">EAN:</label>
                    <input type="text" id="ean" name="ean" value="${escapeHtml(getNullableValue(edition, 'ean'))}">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group" style="flex: 1;">
                    <label for="udc">УДК:</label>
                    <input type="text" id="udc" name="udc" value="${escapeHtml(getNullableValue(edition, 'udc'))}">
                </div>
                <div class="form-group" style="flex: 1;">
                    <label for="bbk">ББК:</label>
                    <input type="text" id="bbk" name="bbk" value="${escapeHtml(getNullableValue(edition, 'bbk'))}">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group" style="flex: 1;">
                    <label for="publisher">Издательство:</label>
                    <input type="text" id="publisher" name="publisher" value="${escapeHtml(editionPublisher)}">
                </div>
                <div class="form-group" style="flex: 1;">
                    <label for="year">Год издания:</label>
                    <input type="number" id="year" name="year" value="${editionYear}">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group" style="flex: 1;">
                    <label for="city">Город:</label>
                    <input type="text" id="city" name="city" value="${escapeHtml(editionCity)}">
                </div>
                <div class="form-group" style="flex: 1;">
                    <label for="pages">Страниц:</label>
                    <input type="number" id="pages" name="pages" value="${editionPages}">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group" style="flex: 1;">
                    <label for="series">Серия:</label>
                    <input type="text" id="series" name="series" value="${escapeHtml(editionSeries)}">
                </div>
                <div class="form-group" style="flex: 1;">
                    <label for="series_number">Номер в серии:</label>
                    <input type="text" id="series_number" name="series_number" value="${escapeHtml(editionSeriesNumber)}">
                </div>
            </div>
            <div class="form-group">
                <label for="edition_language">Язык издания:</label>
                <select id="edition_language" name="edition_language">
                    ${languageOptions}
                </select>
            </div>
            <div class="form-group">
                <label for="edition_annotation">Аннотация издания:</label>
                <textarea id="edition_annotation" name="edition_annotation" rows="3">${escapeHtml(editionAnnotation)}</textarea>
            </div>
            <div class="form-row">
                <div class="form-group" style="flex: 1;">
                    <label for="source">Источник:</label>
                    <input type="text" id="source" name="source" value="${escapeHtml(editionSource)}">
                </div>
                <div class="form-group" style="flex: 1;">
                    <label for="quality">Качество:</label>
                    <select id="quality" name="quality">
                        <option value="excellent" ${editionQuality === 'excellent' ? 'selected' : ''}>Отличное</option>
                        <option value="good" ${editionQuality === 'good' ? 'selected' : ''}>Хорошее</option>
                        <option value="poor" ${editionQuality === 'poor' ? 'selected' : ''}>Плохое</option>
                    </select>
                </div>
            </div>
            <div class="form-row">
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

        <div class="form-actions">
            <button type="submit" class="btn">Сохранить</button>
            <button type="button" class="btn btn-secondary" onclick="closeModal()">Отмена</button>
        </div>
        </form>
    `;

    document.getElementById('genres_select').addEventListener('change', function() {
        const selected = this.selectedOptions.length;
        document.getElementById('selected_genres_count').textContent = selected;
    });

    document.getElementById('tags_select').addEventListener('change', function() {
        const selected = this.selectedOptions.length;
        document.getElementById('selected_tags_count').textContent = selected;
    });
}

function addAuthorRow() {
    const container = document.getElementById('authors-container');
    const idx = container.querySelectorAll('.author-row').length;

    fetch(`${API_BASE}/persons`)
        .then(res => res.json())
        .then(persons => {
            const personsArray = Array.isArray(persons) ? persons : [];
            const options = `<option value="">-- Выберите --</option>` +
                personsArray.map(p => `<option value="${p.id}">${escapeHtml(p.last_name)} ${escapeHtml(p.first_name || '')}</option>`).join('');

            const div = document.createElement('div');
            div.className = 'author-row';
            div.dataset.idx = idx;
            div.innerHTML = `
                <select name="author_id_${idx}" class="author-select">${options}</select>
                <select name="author_role_${idx}" class="author-role">
                    <option value="author">Автор</option>
                    <option value="translator">Переводчик</option>
                    <option value="editor">Редактор</option>
                    <option value="illustrator">Иллюстратор</option>
                </select>
                <button type="button" class="btn-remove-author" onclick="removeAuthorRow(this)">✕</button>
            `;
            container.appendChild(div);
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
        const response = await fetch(`${API_BASE}/genres`, {
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
        const response = await fetch(`${API_BASE}/tags`, {
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