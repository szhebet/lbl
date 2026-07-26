var REFRESH_IN_PROGRESS = null;

function getDeviceFingerprint() {
    var parts = [
        navigator.userAgent,
        screen.width + 'x' + screen.height + 'x' + screen.colorDepth,
        navigator.language || '',
        Intl.DateTimeFormat().resolvedOptions().timeZone || '',
        navigator.hardwareConcurrency || '',
        navigator.platform || ''
    ];
    var raw = parts.join('|||');
    var hash = 0;
    for (var i = 0; i < raw.length; i++) {
        var chr = raw.charCodeAt(i);
        hash = ((hash << 5) - hash) + chr;
        hash |= 0;
    }
    return 'fp_' + Math.abs(hash).toString(36);
}

var authToken = localStorage.getItem('auth_token') || '';
var authUser = null;
try {
    var stored = localStorage.getItem('auth_user');
    if (stored) authUser = JSON.parse(stored);
} catch (e) {}

function isAuthenticated() {
    return !!authToken;
}

function isAndroidApp() {
    return typeof AndroidTokenBridge !== 'undefined';
}

function storeRefreshToken(token) {
    if (isAndroidApp()) {
        try {
            AndroidTokenBridge.storeRefreshToken(token);
        } catch (e) {
            console.warn('Failed to store refresh token:', e);
        }
    }
}

function getRefreshToken() {
    if (isAndroidApp()) {
        try {
            return AndroidTokenBridge.getRefreshToken();
        } catch (e) {
            console.warn('Failed to get refresh token:', e);
        }
    }
    return null;
}

function clearRefreshToken() {
    if (isAndroidApp()) {
        try {
            AndroidTokenBridge.clearRefreshToken();
        } catch (e) {
            console.warn('Failed to clear refresh token:', e);
        }
    }
}

async function tryRefreshToken() {
    var rt = getRefreshToken();
    if (!rt) return false;

    if (REFRESH_IN_PROGRESS) return REFRESH_IN_PROGRESS;

    REFRESH_IN_PROGRESS = (async function() {
        try {
            var resp = await fetch('/api/v1/auth/refresh', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ refresh_token: rt })
            });
            var ct = resp.headers.get('content-type') || '';
            if (resp.ok && ct.indexOf('application/json') !== -1) {
                var data = await resp.json();
                if (data.token) {
                    authToken = data.token;
                    localStorage.setItem('auth_token', authToken);
                    return true;
                }
            }
            authToken = '';
            localStorage.removeItem('auth_token');
            localStorage.removeItem('auth_user');
            authUser = null;
            window.dispatchEvent(new Event('auth-changed'));
            return false;
        } catch (e) {
            console.warn('Refresh failed:', e);
            return false;
        } finally {
            REFRESH_IN_PROGRESS = null;
        }
    })();

    return REFRESH_IN_PROGRESS;
}

function handleLoginResponse(data) {
    if (data.token) {
        authToken = data.token;
        authUser = data.user;
        localStorage.setItem('auth_token', authToken);
        localStorage.setItem('auth_user', JSON.stringify(authUser));
        if (data.refresh_token) {
            storeRefreshToken(data.refresh_token);
        }
        closeLoginModal();
        var btn = document.getElementById('loginBtn');
        if (btn) {
            btn.textContent = authUser.username;
            btn.classList.add('logged-in');
        }
        // Notify offline module about auth change
        window.dispatchEvent(new Event('auth-changed'));
        if (typeof loadUserBookStatuses === 'function') {
            loadUserBookStatuses().then(function() {
                if (typeof refreshCurrentView === 'function') refreshCurrentView();
            });
        }
        // Android: force refresh static files from server (bypass asset cache)
        if (isAndroidApp()) {
            try {
                AndroidTokenBridge.setForceNetworkRefresh(true);
            } catch(e) {}
            // Reload to get fresh static files from server
            setTimeout(function() { location.reload(true); }, 100);
        }
    }
}

document.addEventListener('DOMContentLoaded', function() {
    var loginBtn = document.getElementById('loginBtn');
    if (!loginBtn) return;

    if (isAuthenticated() && authUser) {
        loginBtn.textContent = authUser.username;
        loginBtn.classList.add('logged-in');
    }

    loginBtn.addEventListener('click', function() {
        if (isAuthenticated() && authUser) {
            if (confirm('Вы хотите завершить сессию пользователя?')) {
                doLogout();
                openLoginModal();
            }
        } else {
            openLoginModal();
        }
    });
});

function doLogout() {
    authToken = '';
    authUser = null;
    localStorage.removeItem('auth_token');
    localStorage.removeItem('auth_user');
    clearRefreshToken();
    var btn = document.getElementById('loginBtn');
    if (btn) {
        btn.textContent = 'Авторизоваться';
        btn.classList.remove('logged-in');
    }
    window.dispatchEvent(new Event('auth-changed'));
}

function openLoginModal() {
    var existing = document.getElementById('loginModal');
    if (existing) existing.remove();

    var overlay = document.createElement('div');
    overlay.className = 'modal';
    overlay.id = 'loginModal';
    overlay.innerHTML = '<div class="modal-content" style="max-width:400px">' +
        '<div class="modal-header">' +
        '<h2>Авторизация</h2>' +
        '<button class="modal-close" onclick="closeLoginModal()">&times;</button>' +
        '</div>' +
        '<form id="loginForm" style="padding:20px">' +
        '<div class="form-group">' +
        '<label for="loginUsername">Имя пользователя:</label>' +
        '<input type="text" id="loginUsername" name="username" required autocomplete="username">' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="loginPassword">Пароль:</label>' +
        '<input type="password" id="loginPassword" name="password" required autocomplete="current-password">' +
        '</div>' +
        '<div id="loginError" class="form-error" style="display:none;margin-bottom:10px"></div>' +
        '<div class="modal-footer" style="padding:0;border:none">' +
        '<button type="submit" class="btn" style="width:100%">Войти</button>' +
        '<button type="button" class="btn btn-secondary" style="width:100%;margin-top:8px" onclick="closeLoginModal()">Отмена</button>' +
        '</div>' +
        '</form>' +
        '</div>';
    document.body.appendChild(overlay);
    overlay.classList.add('active');

    document.getElementById('loginForm').addEventListener('submit', async function(e) {
        e.preventDefault();
        var username = document.getElementById('loginUsername').value.trim();
        var password = document.getElementById('loginPassword').value;
        var errorEl = document.getElementById('loginError');

        if (!username || !password) {
            errorEl.textContent = 'Заполните все поля';
            errorEl.style.display = 'block';
            return;
        }

        var fingerprint = getDeviceFingerprint();
        var deviceName = navigator.userAgent.substring(0, 100);

        try {
            var response = await fetch('/api/v1/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    username: username,
                    password: password,
                    device_name: deviceName,
                    device_fingerprint: fingerprint
                })
            });

            var ct = response.headers.get('content-type') || '';
            if (ct.indexOf('application/json') === -1) {
                throw new Error('not-json');
            }
            var data = await response.json();

            if (response.ok && data.token) {
                handleLoginResponse(data);
            } else if (data.user_not_found) {
                if (confirm('Пользователь "' + username + '" не найден. Создать нового пользователя?')) {
                    try {
                        var regResponse = await fetch('/api/v1/auth/register', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({
                                username: username,
                                password: password,
                                device_name: deviceName,
                                device_fingerprint: fingerprint
                            })
                        });
                        var regCt = regResponse.headers.get('content-type') || '';
                        if (regCt.indexOf('application/json') === -1) {
                            throw new Error('not-json');
                        }
                        var regData = await regResponse.json();
                        if (regResponse.ok && regData.token) {
                            handleLoginResponse(regData);
                        } else {
                            errorEl.textContent = regData.error || 'Ошибка создания пользователя';
                            errorEl.style.display = 'block';
                        }
                    } catch (regErr) {
                        errorEl.textContent = 'Ошибка соединения: ' + regErr.message;
                        errorEl.style.display = 'block';
                    }
                }
            } else {
                errorEl.textContent = data.error || 'Ошибка авторизации';
                errorEl.style.display = 'block';
            }
        } catch (err) {
            if (isAndroidApp()) {
                loginViaBridge(username, password, deviceName, fingerprint, errorEl);
            } else {
                errorEl.textContent = 'Ошибка соединения: ' + err.message;
                errorEl.style.display = 'block';
            }
        }
    });
}

function closeLoginModal() {
    var modal = document.getElementById('loginModal');
    if (modal) {
        modal.classList.remove('active');
        modal.remove();
    }
}

function loginViaBridge(username, password, deviceName, fingerprint, errorEl) {
    if (!isAndroidApp()) {
        errorEl.textContent = 'Ошибка соединения';
        errorEl.style.display = 'block';
        return;
    }
    errorEl.textContent = 'Подключение через обход SSL...';
    errorEl.style.display = 'block';
    window._authBridgeLoginErrorEl = errorEl;
    try {
        AndroidTokenBridge.login(username, password, deviceName, fingerprint);
    } catch (e) {
        errorEl.textContent = 'Ошибка моста: ' + e.message;
        errorEl.style.display = 'block';
    }
}

window._authBridgeCallback = function(code, body) {
    var errorEl = window._authBridgeLoginErrorEl;
    try {
        var data = JSON.parse(body);
        if (code >= 200 && code < 300 && data.token) {
            handleLoginResponse(data);
        } else {
            if (errorEl) {
                errorEl.textContent = data.error || 'Ошибка авторизации';
                errorEl.style.display = 'block';
            }
        }
    } catch (e) {
        if (errorEl) {
            errorEl.textContent = 'Ошибка ответа сервера';
            errorEl.style.display = 'block';
        }
    }
};
