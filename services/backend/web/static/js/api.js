const API_BASE = 'http://localhost:8080';

function getToken() {
    return localStorage.getItem('sf_token');
}

function getUsername() {
    return localStorage.getItem('sf_username');
}

function saveAuth(token, username) {
    localStorage.setItem('sf_token', token);
    localStorage.setItem('sf_username', username);
}

function clearAuth() {
    localStorage.removeItem('sf_token');
    localStorage.removeItem('sf_username');
}

function isLoggedIn() {
    return !!getToken();
}

async function apiFetch(path, options = {}) {
    const token = getToken();
    const headers = { 'Content-Type': 'application/json', ...options.headers };
    if (token) {
        headers['Authorization'] = 'Bearer ' + token;
    }

    const res = await fetch(API_BASE + path, { cache: 'no-store', ...options, headers });

    if (res.status === 401) {
        clearAuth();
        window.location.href = '/login';
        throw new Error('Unauthorized');
    }

    const text = await res.text();

    if (!res.ok) {
        let msg = 'Request failed';
        if (text) {
            try { msg = JSON.parse(text).error || msg; } catch(e) {}
        }
        throw new Error(msg);
    }

    if (!text) return null;
    return JSON.parse(text);
}

const api = {
    login(username, password) {
        return apiFetch('/auth/login', {
            method: 'POST',
            body: JSON.stringify({ username, password }),
        });
    },

    register(username, password) {
        return apiFetch('/auth/register', {
            method: 'POST',
            body: JSON.stringify({ username, password }),
        });
    },

    getUserNews(topicId) {
        let path = '/users/news';
        if (topicId) path += '?topic_id=' + topicId;
        return apiFetch(path);
    },

    getUserChannels() {
        return apiFetch('/users/channels');
    },

    addUserChannel(link, name) {
        return apiFetch('/users/channels', {
            method: 'POST',
            body: JSON.stringify({ link, name }),
        });
    },

    removeUserChannel(channelId) {
        return apiFetch('/users/channels/' + channelId, {
            method: 'DELETE',
        });
    },

    getUserTopics() {
        return apiFetch('/users/topics');
    },

    addUserTopic(name) {
        return apiFetch('/users/topics', {
            method: 'POST',
            body: JSON.stringify({ name }),
        });
    },

    removeUserTopic(topicId) {
        return apiFetch('/users/topics/' + topicId, {
            method: 'DELETE',
        });
    },

    getUserStopThemes() {
        return apiFetch('/users/stop-themes');
    },

    addUserStopTheme(name) {
        return apiFetch('/users/stop-themes', {
            method: 'POST',
            body: JSON.stringify({ name }),
        });
    },

    removeUserStopTheme(stopThemeId) {
        return apiFetch('/users/stop-themes/' + stopThemeId, {
            method: 'DELETE',
        });
    },

    getUserBlockedNews(stopThemeId) {
        return apiFetch('/users/blocked-news/' + stopThemeId);
    },

    getReadNews() {
        return apiFetch('/users/news/read');
    },

    markNewsRead(newsId) {
        return apiFetch('/users/news/' + newsId + '/read', { method: 'POST' });
    },

    unmarkNewsRead(newsId) {
        return apiFetch('/users/news/' + newsId + '/read', { method: 'DELETE' });
    },

    getSavedNews() {
        return apiFetch('/users/news/saved');
    },

    saveNews(newsId) {
        return apiFetch('/users/news/' + newsId + '/save', { method: 'POST' });
    },

    unsaveNews(newsId) {
        return apiFetch('/users/news/' + newsId + '/save', { method: 'DELETE' });
    },
};

function timeAgo(dateStr) {
    const now = new Date();
    const date = new Date(dateStr);
    const diff = Math.floor((now - date) / 1000);

    if (diff < 60) return 'только что';
    if (diff < 3600) return Math.floor(diff / 60) + ' мин';
    if (diff < 86400) return Math.floor(diff / 3600) + ' ч';
    if (diff < 604800) return Math.floor(diff / 86400) + ' д';
    return date.toLocaleDateString('ru-RU');
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}
