// chat.js

let ws = null;
let username = '';
let room = '';
let currentStats = null;

const loginScreen = document.getElementById('loginScreen');
const chatScreen = document.getElementById('chatScreen');
const usernameInput = document.getElementById('usernameInput');
const roomInput = document.getElementById('roomInput');
const joinBtn = document.getElementById('joinBtn');
const messageInput = document.getElementById('messageInput');
const sendBtn = document.getElementById('sendBtn');
const messagesContainer = document.getElementById('messagesContainer');
const roomNameSpan = document.getElementById('roomName');
const currentUserSpan = document.getElementById('currentUser');

// Event Listeners
joinBtn.addEventListener('click', connectWebSocket);
sendBtn.addEventListener('click', sendMessage);
messageInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') sendMessage();
});
usernameInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') connectWebSocket();
});
roomInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') connectWebSocket();
});

function connectWebSocket() {
    username = usernameInput.value.trim();
    room = roomInput.value.trim();

    if (!username || !room) {
        alert('Please enter both username and room name');
        return;
    }

    joinBtn.innerHTML = '<span class="spinner"></span>Connecting...';
    joinBtn.disabled = true;

    const wsUrl = `ws://localhost:8080/ws?username=${encodeURIComponent(username)}&room=${encodeURIComponent(room)}`;
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        console.log('Connected to chatroom');
        loginScreen.classList.add('hidden');
        chatScreen.classList.remove('hidden');
        roomNameSpan.textContent = room;
        currentUserSpan.textContent = username;
        addSystemMessage(`Connected to room '${room}'`);
    };

    ws.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);

            if (msg.type === 'stats') {
                currentStats = JSON.parse(msg.text);
            }

            displayMessage(msg);
        } catch (err) {
            console.error('Error parsing message:', err);
        }
    };

    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        joinBtn.innerHTML = 'Join Room';
        joinBtn.disabled = false;
    };

    ws.onclose = () => {
        console.log('Disconnected from chatroom');
        addSystemMessage('Disconnected from server');
        joinBtn.innerHTML = 'Join Room';
        joinBtn.disabled = false;
    };
}

function sendMessage() {
    const text = messageInput.value.trim();
    if (!text || !ws) return;

    const msg = { text: text };
    ws.send(JSON.stringify(msg));
    messageInput.value = '';
}

function sendCommand(cmd) {
    if (!ws) return;
    const msg = { text: cmd };
    ws.send(JSON.stringify(msg));
}

function disconnect() {
    if (ws) {
        ws.close();
        ws = null;
    }
    chatScreen.classList.add('hidden');
    loginScreen.classList.remove('hidden');
    messagesContainer.innerHTML = '';
    currentStats = null;
}

function addSystemMessage(text) {
    const time = new Date().toLocaleTimeString('en-US', { hour12: false });
    displayMessage({
        type: 'system',
        text: text,
        time: time
    });
}

function displayMessage(msg) {
    const messageDiv = document.createElement('div');
    messageDiv.className = 'message';

    switch (msg.type) {
        case 'chat':
            const isOwn = msg.username === username;
            messageDiv.innerHTML = `
                <div class="message-chat ${isOwn ? 'own' : ''}">
                    <div class="message-bubble ${isOwn ? 'own' : 'other'}">
                        <div class="message-meta">${msg.username} · ${msg.time}</div>
                        <div class="message-text">${escapeHtml(msg.text)}</div>
                    </div>
                </div>
            `;
            break;

        case 'system':
            messageDiv.innerHTML = `
                <div class="message-system">
                    <span class="system-badge">${msg.time ? msg.time + ' · ' : ''}${escapeHtml(msg.text)}</span>
                </div>
            `;
            break;

        case 'user_list':
            messageDiv.innerHTML = `
                <div class="message-info info-users">
                    <div class="info-title">👥 Users in room</div>
                    <div class="info-content">${escapeHtml(msg.text)}</div>
                </div>
            `;
            break;

        case 'room':
            messageDiv.innerHTML = `
                <div class="message-info info-rooms">
                    <div class="info-title"># Available rooms</div>
                    <div class="info-content">${escapeHtml(msg.text)}</div>
                </div>
            `;
            break;

        case 'stats':
            if (currentStats) {
                let roomDetailsHtml = '';
                for (const [roomName, count] of Object.entries(currentStats.room_details)) {
                    roomDetailsHtml += `
                        <div class="stat-row">
                            <span>${escapeHtml(roomName)}:</span>
                            <span class="stat-label">${count} user${count !== 1 ? 's' : ''}</span>
                        </div>
                    `;
                }

                messageDiv.innerHTML = `
                    <div class="message-info info-stats">
                        <div class="info-title">📊 Server Statistics</div>
                        <div class="info-content">
                            <div class="stats-grid">
                                <div class="stat-row">
                                    <span>Total Users:</span>
                                    <span class="stat-label">${currentStats.total_users}</span>
                                </div>
                                <div class="stat-row">
                                    <span>Total Rooms:</span>
                                    <span class="stat-label">${currentStats.total_rooms}</span>
                                </div>
                                <div class="stats-divider">
                                    <strong>Room Details:</strong>
                                    ${roomDetailsHtml}
                                </div>
                            </div>
                        </div>
                    </div>
                `;
            }
            break;
    }

    messagesContainer.appendChild(messageDiv);
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
