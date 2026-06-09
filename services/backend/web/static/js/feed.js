/* ===== STATE ===== */
let currentTopicId = null;
let topicsCache = [];
let newsCache = [];
let editMode = false;

/* ===== ICON PATHS ===== */
const ICO = '/static/images/';

/* ===== EDIT MODE ===== */
document.getElementById('edit-categories-btn').addEventListener('click', function() {
    editMode = !editMode;
    this.classList.toggle('active', editMode);
    renderCategories();
});

/* ===== CATEGORIES ===== */
async function loadTopics() {
    try {
        topicsCache = await api.getUserTopics() || [];
    } catch (e) {
        topicsCache = [];
    }
    renderCategories();
}

function renderCategories() {
    const list = document.getElementById('categories-list');
    let html = '';

    // General / Newsfeed — always visible, not deletable
    if (!editMode) {
        html += `<div class="cat-item ${currentTopicId === null ? 'active' : ''}" onclick="selectTopic(null)">
            <div class="cat-newsfeed">
                <span class="cat-newsfeed__emoji">📰</span>
                <span class="cat-newsfeed__label">Newsfeed</span>
            </div>
            <span class="cat-item__badge">${newsCache.length}</span>
        </div>`;
    } else {
        html += `<div class="cat-item">
            <div class="cat-newsfeed">
                <span class="cat-newsfeed__emoji">📰</span>
                <span class="cat-newsfeed__label">General</span>
            </div>
        </div>`;
    }

    html += `<div class="cat-divider"><div class="cat-divider__line"></div></div>`;

    // Topics
    for (const topic of topicsCache) {
        if (editMode) {
            html += `<div class="cat-item">
                <span class="cat-item__name">${escapeHtml(topic.Name)}</span>
                <div class="cat-item__delete" onclick="deleteTopic(${topic.ID})" title="Удалить"><img src="${ICO}trash.svg" width="16" height="16"></div>
            </div>`;
        } else {
            const isActive = currentTopicId === topic.ID;
            html += `<div class="cat-item ${isActive ? 'active' : ''}" onclick="selectTopic(${topic.ID})">
                <span class="cat-item__name">${escapeHtml(topic.Name)}</span>
            </div>`;
        }
    }

    // Add category button (only in edit mode)
    if (editMode) {
        html += `<div class="cat-add-btn-wrap" id="cat-add-area">
            <button class="btn-add-category" onclick="showAddCategory()">
                <span>+</span><span>Добавить категорию</span>
            </button>
        </div>`;
    }

    list.innerHTML = html;
}

function selectTopic(topicId) {
    currentTopicId = topicId;
    loadNews();
    renderCategories();
}

async function deleteTopic(topicId) {
    try {
        await api.removeUserTopic(topicId);
        if (currentTopicId === topicId) currentTopicId = null;
        await loadTopics();
        loadNews();
    } catch (e) {
        console.error('Failed to delete topic:', e);
    }
}

function showAddCategory() {
    const area = document.getElementById('cat-add-area');
    area.className = '';
    area.innerHTML = `<div class="cat-add-row">
        <input type="text" class="cat-add-input" id="new-category-input" placeholder="Название...">
        <div class="cat-add-confirm" onclick="confirmAddCategory()" title="Добавить">
            <img src="${ICO}checkmark.svg" width="14" height="14">
        </div>
    </div>`;

    const input = document.getElementById('new-category-input');
    input.focus();

    input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') confirmAddCategory();
        if (e.key === 'Escape') renderCategories();
    });
}

async function confirmAddCategory() {
    const input = document.getElementById('new-category-input');
    if (!input) return;
    const name = input.value.trim();
    if (!name) return;

    try {
        await api.addUserTopic(name);
        await loadTopics();
    } catch (err) {
        console.error('Failed to add topic:', err);
    }
}

/* ===== NEWS ===== */
async function loadNews() {
    const grid = document.getElementById('news-grid');
    grid.innerHTML = '<div class="loading"><div class="loading__spinner"></div> Загрузка новостей...</div>';

    try {
        newsCache = await api.getUserNews(currentTopicId) || [];
        renderNews();
        updateNewsHeader();
        renderCategories();
    } catch (e) {
        grid.innerHTML = '<div class="empty-state"><span>Не удалось загрузить новости</span></div>';
    }
}

function updateNewsHeader() {
    const titleEl = document.getElementById('news-topic-title');
    const countEl = document.getElementById('news-topic-count');

    if (currentTopicId === null) {
        titleEl.textContent = 'Все новости';
    } else {
        const topic = topicsCache.find(t => t.ID === currentTopicId);
        titleEl.textContent = topic ? topic.Name : 'Категория';
    }

    const count = newsCache.length;
    countEl.textContent = count > 0 ? `· ${count} ${pluralize(count, 'новость', 'новости', 'новостей')}` : '';
}

function renderNews() {
    const grid = document.getElementById('news-grid');

    if (newsCache.length === 0) {
        grid.innerHTML = '<div class="empty-state"><span>Нет новостей</span><span style="font-size:12px;color:#a6adbf;">Добавьте каналы, чтобы получать новости</span></div>';
        return;
    }

    let html = '';
    for (const group of newsCache) {
        const n = group.main;
        const dupCount = group.duplicate_count;
        const titleText = n.body.length > 70 ? n.body.substring(0, 70) + '...' : n.body;
        const bodyShort = n.body.length > 120 ? n.body.substring(0, 120) + '...' : n.body;

        html += `<div class="news-card">
            <div class="news-card__body news-card__body--noimg">
                <span class="news-card__title">${escapeHtml(titleText)}</span>
                <span class="news-card__desc">${escapeHtml(bodyShort)}</span>
                <div class="news-card__meta">
                    <span class="news-card__source">${escapeHtml(n.channel.name || 'Канал')}</span>
                    <span class="news-card__time">· ${timeAgo(n.created_at)}</span>
                </div>
            </div>
            <div class="news-card__footer">
                ${dupCount > 0 ? `<div class="dup-badge"><img src="${ICO}stack.svg" width="12" height="12"><span>+${dupCount}</span></div>` : '<div></div>'}
                <div class="news-card__actions">
                    <img src="${ICO}bookmark.svg" width="14" height="14">
                    <img src="${ICO}eye.svg" width="14" height="14">
                    <span class="news-card__more">⋯</span>
                </div>
            </div>
        </div>`;
    }

    grid.innerHTML = html;
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

    loadTopics();
    loadNews();
})();
