/* ===== REDIRECT IF ALREADY LOGGED IN ===== */
(function() {
    if (localStorage.getItem('sf_token')) {
        window.location.href = '/feed';
    }
})();

/* ===== TAB SWITCHING ===== */
function switchTab(tab) {
    const loginTab = document.getElementById('tab-login');
    const registerTab = document.getElementById('tab-register');
    const loginForm = document.getElementById('form-login');
    const registerForm = document.getElementById('form-register');

    if (tab === 'login') {
        loginTab.classList.add('active');
        registerTab.classList.remove('active');
        loginForm.style.display = 'flex';
        registerForm.style.display = 'none';
    } else {
        registerTab.classList.add('active');
        loginTab.classList.remove('active');
        registerForm.style.display = 'flex';
        loginForm.style.display = 'none';
    }

    // Clear errors
    document.getElementById('login-error').textContent = '';
    document.getElementById('register-error').textContent = '';
}

/* ===== LOGIN ===== */
document.getElementById('login-btn').addEventListener('click', async function() {
    const username = document.getElementById('login-username').value.trim();
    const password = document.getElementById('login-password').value;
    const errorEl = document.getElementById('login-error');

    errorEl.textContent = '';

    if (!username || !password) {
        errorEl.textContent = 'Заполните все поля';
        return;
    }

    this.disabled = true;
    this.textContent = 'Вход...';

    try {
        const res = await fetch('/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
        });

        const data = await res.json();

        if (!res.ok) {
            errorEl.textContent = data.error || 'Ошибка входа';
            return;
        }

        localStorage.setItem('sf_token', data.token);
        localStorage.setItem('sf_username', data.username);

        window.location.href = '/feed';
    } catch (e) {
        errorEl.textContent = 'Ошибка соединения с сервером';
    } finally {
        this.disabled = false;
        this.textContent = 'Войти';
    }
});

/* ===== REGISTER ===== */
document.getElementById('register-btn').addEventListener('click', async function() {
    const username = document.getElementById('reg-username').value.trim();
    const password = document.getElementById('reg-password').value;
    const password2 = document.getElementById('reg-password2').value;
    const errorEl = document.getElementById('register-error');

    errorEl.textContent = '';

    if (!username || !password || !password2) {
        errorEl.textContent = 'Заполните все поля';
        return;
    }

    if (password !== password2) {
        errorEl.textContent = 'Пароли не совпадают';
        return;
    }

    if (password.length < 6) {
        errorEl.textContent = 'Пароль должен быть не менее 6 символов';
        return;
    }

    this.disabled = true;
    this.textContent = 'Создание...';

    try {
        const res = await fetch('/auth/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
        });

        const data = await res.json();

        if (!res.ok) {
            errorEl.textContent = data.error || 'Ошибка регистрации';
            return;
        }

        const loginRes = await fetch('/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
        });

        const loginData = await loginRes.json();

        if (loginRes.ok) {
            localStorage.setItem('sf_token', loginData.token);
            localStorage.setItem('sf_username', loginData.username);
            window.location.href = '/feed';
        } else {
            switchTab('login');
        }
    } catch (e) {
        errorEl.textContent = 'Ошибка соединения с сервером';
    } finally {
        this.disabled = false;
        this.textContent = 'Создать аккаунт';
    }
});

/* Enter key submit */
document.querySelectorAll('.auth-input').forEach(input => {
    input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
            const form = this.closest('.auth-form');
            form.querySelector('.auth-submit').click();
        }
    });
});
