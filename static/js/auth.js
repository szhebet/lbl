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
            if (resp.ok) {
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
        openLoginModal();
    });
});

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

        try {
            var response = await fetch('/api/v1/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    username: username,
                    password: password,
                    device_name: navigator.userAgent.substring(0, 100),
                    device_fingerprint: fingerprint
                })
            });

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
                                device_name: navigator.userAgent.substring(0, 100),
                                device_fingerprint: fingerprint
                            })
                        });
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
            errorEl.textContent = 'Ошибка соединения: ' + err.message;
            errorEl.style.display = 'block';
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
