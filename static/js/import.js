if (typeof API_BASE === 'undefined') {
    const API_BASE = '/api/v1';
}

document.addEventListener('DOMContentLoaded', function() {
    const uploadForm = document.getElementById('uploadForm');
    const directoryForm = document.getElementById('directoryForm');
    const bookFile = document.getElementById('bookFile');
    const uploadPreview = document.getElementById('uploadPreview');
    const uploadResult = document.getElementById('uploadResult');
    const directoryResult = document.getElementById('directoryResult');

    if (bookFile) {
        bookFile.addEventListener('change', function(e) {
            const file = e.target.files[0];
            if (file) {
                const sizeMB = (file.size / (1024 * 1024)).toFixed(2);
                uploadPreview.innerHTML = `
                    <div class="preview-item">
                        <strong>${escapeHtml(file.name)}</strong>
                        <span>${sizeMB} МБ</span>
                    </div>
                `;
                uploadPreview.style.display = 'block';
            }
        });
    }

    if (uploadForm) {
        uploadForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const fileInput = document.getElementById('bookFile');
            const file = fileInput.files[0];
            
            if (!file) {
                uploadResult.innerHTML = '<div class="error">Выберите файл</div>';
                return;
            }

            uploadResult.innerHTML = '<div class="loading">Загрузка...</div>';

            const formData = new FormData();
            formData.append('file', file);

            try {
                const response = await fetch(`${API_BASE}/import/file`, {
                    method: 'POST',
                    body: formData
                });

                const data = await response.json();

                if (response.ok) {
                    uploadResult.innerHTML = `
                        <div class="success">
                            <h4>Книга успешно импортирована!</h4>
                            <ul>
                                <li><strong>Название:</strong> ${escapeHtml(data.title || 'N/A')}</li>
                                <li><strong>Авторы:</strong> ${escapeHtml((data.authors || []).join(', ') || 'N/A')}</li>
                                <li><strong>Год:</strong> ${data.year || 'N/A'}</li>
                                <li><strong>ISBN:</strong> ${data.isbn || 'N/A'}</li>
                                <li><strong>Язык:</strong> ${data.language || 'N/A'}</li>
                                <li><strong>Файл:</strong> ${escapeHtml(data.file_path || 'N/A')}</li>
                            </ul>
                            <p>Parsed: ${data.parsed ? '✓ Метаданные извлечены' : '⚠ Метаданные не найдены'}</p>
                        </div>
                    `;
                    fileInput.value = '';
                    uploadPreview.style.display = 'none';
                    if (typeof loadAuthorsWithState === 'function') {
                        loadAuthorsWithState(saveExpandedState());
                    }
                } else {
                    uploadResult.innerHTML = `<div class="error">Ошибка: ${escapeHtml(data.error || 'Неизвестная ошибка')}</div>`;
                }
            } catch (error) {
                uploadResult.innerHTML = `<div class="error">Ошибка загрузки: ${escapeHtml(error.message)}</div>`;
            }
        });
    }

    if (directoryForm) {
        directoryForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const dirPath = document.getElementById('directoryPath').value.trim();
            
            if (!dirPath) {
                directoryResult.innerHTML = '<div class="error">Введите путь к папке</div>';
                return;
            }

            directoryResult.innerHTML = '<div class="loading">Импорт книг из папки...</div>';

            try {
                const formData = new FormData();
                formData.append('directory', dirPath);

                const response = await fetch(`${API_BASE}/import/directory`, {
                    method: 'POST',
                    body: formData
                });

                const data = await response.json();

                if (response.ok) {
                    let html = `
                        <div class="success">
                            <h4>Импорт завершен</h4>
                            <p><strong>Всего файлов:</strong> ${data.total || 0}</p>
                            <p><strong>Успешно:</strong> ${data.success || 0}</p>
                            <p><strong>Ошибок:</strong> ${data.errors || 0}</p>
                        </div>
                    `;

                    if (data.results && data.results.length > 0) {
                        html += '<div class="results-list"><h4>Результаты:</h4><ul>';
                        data.results.forEach(function(item) {
                            if (item.success) {
                                html += `<li class="success-item">✓ ${escapeHtml(item.title || item.file)} - импортирована</li>`;
                            } else {
                                html += `<li class="error-item">✗ ${escapeHtml(item.file)} - ${escapeHtml(item.error || 'ошибка')}</li>`;
                            }
                        });
                        html += '</ul></div>';
                    }

                    directoryResult.innerHTML = html;

                    if (typeof loadAuthorsWithState === 'function') {
                        loadAuthorsWithState(saveExpandedState());
                    }
                } else {
                    directoryResult.innerHTML = `<div class="error">Ошибка: ${escapeHtml(data.error || 'Неизвестная ошибка')}</div>`;
                }
            } catch (error) {
                directoryResult.innerHTML = `<div class="error">Ошибка загрузки: ${escapeHtml(error.message)}</div>`;
            }
        });
    }
});

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}