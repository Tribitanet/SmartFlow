/* ===== STOP THEMES ===== */
async function loadStopThemes() {
    const container = document.getElementById('stop-themes-list');
    container.innerHTML = '<div class="loading"><div class="loading__spinner"></div> Загрузка стоп-тем...</div>';

    try {
        const themes = await api.getUserStopThemes() || [];
        renderStopThemes(themes);
    } catch (e) {
        container.innerHTML = '<div class="empty-state"><span>Не удалось загрузить стоп-темы</span></div>';
    }
}

function renderStopThemes(themes) {
    const container = document.getElementById('stop-themes-list');
    const countEl = document.getElementById('stop-themes-count');

    countEl.textContent = `${themes.length} ${pluralize(themes.length, 'тема', 'темы', 'тем')}`;

    if (themes.length === 0) {
        container.innerHTML = '<div class="empty-state"><span>Нет активных стоп-тем</span></div>';
        return;
    }

    let html = '';
    for (const theme of themes) {
        html += `<div class="stop-item">
            <div class="stop-item__left">
                <div class="stop-item__dot"></div>
                <span class="stop-item__name">${escapeHtml(theme.Name)}</span>
            </div>
            <div class="stop-item__right">
                <button class="btn-remove-stop" onclick="removeStopTheme(${theme.ID})">✕ Удалить</button>
            </div>
        </div>`;
    }

    container.innerHTML = html;
}

document.getElementById('add-stop-theme-btn').addEventListener('click', addStopTheme);
document.getElementById('stop-theme-input').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') addStopTheme();
});

async function addStopTheme() {
    const input = document.getElementById('stop-theme-input');
    const name = input.value.trim();
    if (!name) return;

    try {
        await api.addUserStopTheme(name);
        input.value = '';
        loadStopThemes();
    } catch (e) {
        console.error('Failed to add stop theme:', e);
    }
}

async function removeStopTheme(id) {
    try {
        await api.removeUserStopTheme(id);
        loadStopThemes();
    } catch (e) {
        console.error('Failed to remove stop theme:', e);
    }
}

function pluralize(n, one, few, many) {
    const mod10 = n % 10;
    const mod100 = n % 100;
    if (mod100 >= 11 && mod100 <= 19) return many;
    if (mod10 === 1) return one;
    if (mod10 >= 2 && mod10 <= 4) return few;
    return many;
}

/* ===== INIT ===== */
(function() {
    if (!isLoggedIn()) {
        window.location.href = '/login';
        return;
    }
    const username = localStorage.getItem('sf_username') || 'U';
    document.getElementById('user-avatar').textContent = username.substring(0, 2).toUpperCase();

    loadStopThemes();
})();
