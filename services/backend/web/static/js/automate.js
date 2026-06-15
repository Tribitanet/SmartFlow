/* ===== STOP THEMES ===== */
let themesCache = [];

async function loadStopThemes() {
    const container = document.getElementById('stop-themes-list');
    container.innerHTML = '<div class="loading"><div class="loading__spinner"></div> Загрузка стоп-тем...</div>';

    try {
        themesCache = await api.getUserStopThemes() || [];
        renderStopThemes(themesCache);
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
                <button class="btn-view-blocked" onclick="openBlockedNews(${theme.ID}, '${escapeHtml(theme.Name).replace(/'/g, "\\'")}')">Заблокированные</button>
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
    // Оптимистично убираем из UI
    const prev = themesCache;
    themesCache = themesCache.filter(t => t.ID !== id);
    renderStopThemes(themesCache);

    try {
        await api.removeUserStopTheme(id);
    } catch (e) {
        // Откат при ошибке
        themesCache = prev;
        renderStopThemes(themesCache);
        console.error('Failed to remove stop theme:', e);
    }
}

/* ===== BLOCKED NEWS MODAL ===== */
async function openBlockedNews(stopThemeId, themeName) {
    const overlay = document.getElementById('blocked-modal');
    const titleEl = document.getElementById('blocked-modal-title');
    const bodyEl = document.getElementById('blocked-modal-body');

    titleEl.textContent = 'Заблокировано: ' + themeName;
    bodyEl.innerHTML = '<div class="loading"><div class="loading__spinner"></div> Загрузка...</div>';
    overlay.style.display = '';
    document.body.style.overflow = 'hidden';

    try {
        const groups = await api.getUserBlockedNews(stopThemeId) || [];
        renderBlockedNews(groups);
    } catch (e) {
        bodyEl.innerHTML = '<div class="empty-state"><span>Не удалось загрузить новости</span></div>';
    }
}

function renderBlockedNews(groups) {
    const bodyEl = document.getElementById('blocked-modal-body');

    if (groups.length === 0) {
        bodyEl.innerHTML = '<div class="empty-state"><span>Нет заблокированных новостей</span></div>';
        return;
    }

    let html = '';
    for (const group of groups) {
        const n = group.main;
        const dupCount = group.duplicate_count;
        html += `<div class="blocked-item">
            <div class="blocked-item__text">${escapeHtml(n.body)}</div>
            <div class="blocked-item__meta">
                <span class="blocked-item__source">${escapeHtml(n.channel.name || 'Канал')}</span>
                <span class="blocked-item__time">· ${timeAgo(n.created_at)}</span>
                ${dupCount > 0 ? `<span class="blocked-item__dup">+${dupCount}</span>` : ''}
                ${n.message_link ? `<a class="blocked-item__link" href="${n.message_link}" target="_blank">Открыть →</a>` : ''}
            </div>
        </div>`;
    }
    bodyEl.innerHTML = html;
}

function closeBlockedNews() {
    document.getElementById('blocked-modal').style.display = 'none';
    document.body.style.overflow = '';
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

    document.getElementById('blocked-modal-close').addEventListener('click', closeBlockedNews);
    document.getElementById('blocked-modal').addEventListener('click', function(e) {
        if (e.target === this) closeBlockedNews();
    });
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') closeBlockedNews();
    });

    loadStopThemes();
})();
