/* ===== STATE ===== */
let currentTopicId = null;
let topicsCache = [];
let newsCache = [];
let editMode = false;
let readSet = new Set();
let savedSet = new Set();

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
                <span class="cat-newsfeed__label">General</span>
            </div>
            <span class="cat-item__badge">${newsCache.length}</span>
        </div>`;
    } else {
        html += `<div class="cat-item">
            <div class="cat-newsfeed">
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

/* ===== READ / SAVED ACTIONS ===== */
async function toggleRead(newsId, btn) {
    const isRead = readSet.has(newsId);
    const card = btn.closest('.news-card');
    // Мгновенно обновляем UI
    if (isRead) {
        readSet.delete(newsId);
        btn.classList.remove('active');
        if (card) card.classList.remove('news-card--read');
    } else {
        readSet.add(newsId);
        btn.classList.add('active');
        if (card) card.classList.add('news-card--read');
    }
    try {
        if (isRead) {
            await api.unmarkNewsRead(newsId);
        } else {
            await api.markNewsRead(newsId);
        }
    } catch (e) {
        // Откатываем при ошибке
        if (isRead) {
            readSet.add(newsId);
            btn.classList.add('active');
            if (card) card.classList.add('news-card--read');
        } else {
            readSet.delete(newsId);
            btn.classList.remove('active');
            if (card) card.classList.remove('news-card--read');
        }
        console.error('Failed to toggle read:', e);
    }
}

async function toggleSaved(newsId, btn) {
    const isSaved = savedSet.has(newsId);
    const img = btn.querySelector('img');
    // Мгновенно обновляем UI
    if (isSaved) {
        savedSet.delete(newsId);
        btn.classList.remove('active');
        if (img) img.src = ICO + 'bookmark.svg';
    } else {
        savedSet.add(newsId);
        btn.classList.add('active');
        if (img) img.src = ICO + 'bookmark-filled.svg';
    }
    try {
        if (isSaved) {
            await api.unsaveNews(newsId);
        } else {
            await api.saveNews(newsId);
        }
    } catch (e) {
        // Откатываем при ошибке
        if (isSaved) {
            savedSet.add(newsId);
            btn.classList.add('active');
            if (img) img.src = ICO + 'bookmark-filled.svg';
        } else {
            savedSet.delete(newsId);
            btn.classList.remove('active');
            if (img) img.src = ICO + 'bookmark.svg';
        }
        console.error('Failed to toggle saved:', e);
    }
}

/* ===== NEWS ===== */
async function loadReadSaved() {
    try {
        const [readList, savedList] = await Promise.all([
            api.getReadNews(),
            api.getSavedNews(),
        ]);
        readSet = new Set((readList || []).map(g => g.main.id));
        savedSet = new Set((savedList || []).map(g => g.main.id));
    } catch (e) {
        // not critical — default to empty sets
    }
}

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
        const isRead = readSet.has(n.id);
        const isSaved = savedSet.has(n.id);
        const titleText = n.body.length > 70 ? n.body.substring(0, 70) + '...' : n.body;
        const bodyShort = n.body.length > 120 ? n.body.substring(0, 120) + '...' : n.body;

        const groupIdx = newsCache.indexOf(group);
        html += `<div class="news-card${isRead ? ' news-card--read' : ''}" onclick="openNewsModal(${groupIdx})" style="cursor:pointer;">
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
                    <button class="card-action-btn${isSaved ? ' active' : ''}" onclick="event.stopPropagation(); toggleSaved(${n.id}, this)" title="Сохранить">
                        <img src="${ICO}${isSaved ? 'bookmark-filled' : 'bookmark'}.svg" width="14" height="14">
                    </button>
                    <button class="card-action-btn${isRead ? ' active' : ''}" onclick="event.stopPropagation(); toggleRead(${n.id}, this)" title="Отметить прочитанным">
                        <img src="${ICO}eye.svg" width="14" height="14">
                    </button>
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

/* ===== NEWS MODAL ===== */
let modalGroup = null;   // current NewsGroup being shown
let modalDupIdx = -1;    // -1 = main, 0..N = duplicate index

function openNewsModal(groupIdx) {
    modalGroup = newsCache[groupIdx];
    if (!modalGroup) return;
    modalDupIdx = -1;
    renderModal();
    document.getElementById('news-modal').style.display = '';
    document.body.style.overflow = 'hidden';
}

function closeNewsModal() {
    document.getElementById('news-modal').style.display = 'none';
    document.body.style.overflow = '';
    modalGroup = null;
}

function renderModal() {
    if (!modalGroup) return;
    const item = modalDupIdx === -1 ? modalGroup.main : modalGroup.duplicates[modalDupIdx];
    const n = item;

    // Header
    document.getElementById('modal-source').textContent = n.channel.name || 'Канал';
    document.getElementById('modal-time').textContent = '· ' + timeAgo(n.created_at);

    // Body
    document.getElementById('modal-title').textContent = n.body.length > 100 ? n.body.substring(0, 100) + '...' : n.body;
    document.getElementById('modal-text').textContent = n.body;

    // Tag (topic)
    const tagEl = document.getElementById('modal-tag');
    if (n.topics && n.topics.length > 0) {
        tagEl.textContent = n.topics[0].name;
        tagEl.style.display = '';
    } else {
        tagEl.style.display = 'none';
    }

    // Link
    const linkEl = document.getElementById('modal-link');
    linkEl.href = n.message_link || '#';

    // Save button state
    const saveBtn = document.getElementById('modal-save-btn');
    const isSaved = savedSet.has(n.id);
    saveBtn.classList.toggle('active', isSaved);
    document.getElementById('modal-save-icon').src = ICO + (isSaved ? 'bookmark-filled' : 'bookmark') + '.svg';

    // Read button state
    const readBtn = document.getElementById('modal-read-btn');
    readBtn.classList.toggle('active', readSet.has(n.id));

    // Duplicates
    const dupGroup = document.getElementById('modal-dup-group');
    const dupCount = modalGroup.duplicate_count;
    if (dupCount > 0) {
        dupGroup.style.display = '';
        document.getElementById('modal-dup-count').textContent = '+' + dupCount + ' ' + pluralize(dupCount, 'похожая', 'похожих', 'похожих');
    } else {
        dupGroup.style.display = 'none';
    }
}

function modalDupPrev() {
    if (!modalGroup || modalGroup.duplicate_count === 0) return;
    if (modalDupIdx <= -1) {
        modalDupIdx = modalGroup.duplicates.length - 1;
    } else {
        modalDupIdx--;
    }
    renderModal();
}

function modalDupNext() {
    if (!modalGroup || modalGroup.duplicate_count === 0) return;
    if (modalDupIdx >= modalGroup.duplicates.length - 1) {
        modalDupIdx = -1;
    } else {
        modalDupIdx++;
    }
    renderModal();
}

function modalToggleSave() {
    if (!modalGroup) return;
    const n = modalDupIdx === -1 ? modalGroup.main : modalGroup.duplicates[modalDupIdx];
    const btn = document.getElementById('modal-save-btn');
    toggleSaved(n.id, btn);
    setTimeout(() => {
        document.getElementById('modal-save-icon').src = ICO + (savedSet.has(n.id) ? 'bookmark-filled' : 'bookmark') + '.svg';
    }, 50);
}

function modalToggleRead() {
    if (!modalGroup) return;
    const n = modalDupIdx === -1 ? modalGroup.main : modalGroup.duplicates[modalDupIdx];
    const btn = document.getElementById('modal-read-btn');
    toggleRead(n.id, btn);
}

// Event listeners
document.getElementById('modal-close-btn').addEventListener('click', closeNewsModal);
document.getElementById('news-modal').addEventListener('click', function(e) {
    if (e.target === this) closeNewsModal();
});
document.getElementById('modal-save-btn').addEventListener('click', modalToggleSave);
document.getElementById('modal-read-btn').addEventListener('click', modalToggleRead);
document.getElementById('modal-dup-prev').addEventListener('click', modalDupPrev);
document.getElementById('modal-dup-next').addEventListener('click', modalDupNext);
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') closeNewsModal();
});

/* ===== INIT ===== */
(function() {
    if (!isLoggedIn()) {
        window.location.href = '/login';
        return;
    }
    const username = localStorage.getItem('sf_username') || 'U';
    document.getElementById('user-avatar').textContent = username.substring(0, 2).toUpperCase();

    loadReadSaved().then(() => {
        loadTopics();
        loadNews();
    });
})();
