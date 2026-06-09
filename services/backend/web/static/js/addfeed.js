/* ===== CHANNELS ===== */
async function loadMyChannels() {
    const container = document.getElementById('my-channels-list');
    container.innerHTML = '<div class="loading"><div class="loading__spinner"></div> Загрузка каналов...</div>';

    try {
        const channels = await api.getUserChannels() || [];
        renderMyChannels(channels);
    } catch (e) {
        container.innerHTML = '<div class="empty-state"><span>Не удалось загрузить каналы</span></div>';
    }
}

function renderMyChannels(channels) {
    const container = document.getElementById('my-channels-list');

    if (channels.length === 0) {
        container.innerHTML = '<div class="empty-state"><span>У вас пока нет каналов</span></div>';
        return;
    }

    let html = '';
    for (const ch of channels) {
        const linkShort = (ch.Link || '').replace('https://t.me/', '@');
        html += `<div class="channel-card channel-card--my">
            <div class="channel-card__info">
                <div class="channel-card__avatar"></div>
                <div class="channel-card__text">
                    <span class="channel-card__name">${escapeHtml(ch.Name)}</span>
                    <span class="channel-card__desc">${escapeHtml(linkShort)}</span>
                </div>
            </div>
            <button class="btn-remove-channel" onclick="removeChannel(${ch.ID})">✕ Удалить</button>
        </div>`;
    }

    container.innerHTML = html;
}

async function addChannel(link, name) {
    try {
        await api.addUserChannel(link, name);
        loadMyChannels();
    } catch (e) {
        console.error('Failed to add channel:', e);
    }
}

async function removeChannel(channelId) {
    try {
        await api.removeUserChannel(channelId);
        loadMyChannels();
    } catch (e) {
        console.error('Failed to remove channel:', e);
    }
}

/* ===== SEARCH INPUT ===== */
document.getElementById('channel-search-input').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') {
        const val = this.value.trim();
        if (!val) return;

        let link = val;
        let name = val;

        if (val.startsWith('@')) {
            link = 'https://t.me/' + val.substring(1);
            name = val.substring(1);
        } else if (val.includes('t.me/')) {
            name = val.split('t.me/').pop();
        }

        addChannel(link, name);
        this.value = '';
    }
});

/* ===== INIT ===== */
(function() {
    if (!isLoggedIn()) {
        window.location.href = '/login';
        return;
    }
    const username = localStorage.getItem('sf_username') || 'U';
    document.getElementById('user-avatar').textContent = username.substring(0, 2).toUpperCase();

    loadMyChannels();
})();
