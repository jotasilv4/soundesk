// SounDesk App State
let ws = null;
let currentSession = null;
let currentRole = null; // 'server' or 'client'
let soundsList = [];
let activeAudios = []; // Keep track of playing HTML5 Audio elements in browser

// DOM Elements
const connectionStatus = document.getElementById('connection-status');
const btnCreateSession = document.getElementById('btn-create-session');
const inputSessionName = document.getElementById('new-session-name');
const sessionsList = document.getElementById('sessions-list');
const uploadForm = document.getElementById('upload-form');
const btnUpload = document.getElementById('btn-upload');
const soundFile = document.getElementById('sound-file');
const fileNameDisplay = document.getElementById('file-name-display');
const deckCard = document.getElementById('deck-card');
const currentSessionTitle = document.getElementById('current-session-title');
const roleSelectorContainer = document.getElementById('role-selector-container');
const roleBadge = document.getElementById('role-badge');
const btnLeave = document.getElementById('btn-leave');
const btnStopAll = document.getElementById('btn-stop-all');
const serverStatusView = document.getElementById('server-status-view');
const soundsGrid = document.getElementById('sounds-grid');
const activityLog = document.getElementById('activity-log');

// Initialization
document.addEventListener('DOMContentLoaded', () => {
    fetchSounds();
    fetchSessions();
    setupEventListeners();
});

// Event Listeners
function setupEventListeners() {
    // Session creation
    btnCreateSession.addEventListener('click', createSession);
    
    // File upload label updater
    soundFile.addEventListener('change', (e) => {
        if (e.target.files.length > 0) {
            fileNameDisplay.textContent = e.target.files[0].name;
            fileNameDisplay.style.color = 'var(--text-primary)';
        } else {
            fileNameDisplay.textContent = 'Escolher arquivo (MP3/WAV)';
            fileNameDisplay.style.color = 'var(--text-secondary)';
        }
    });

    // Sound Upload
    uploadForm.addEventListener('submit', uploadSound);

    // Leave Session
    btnLeave.addEventListener('click', leaveSession);

    // Stop All Sounds
    btnStopAll.addEventListener('click', triggerStopAll);

    // Mobile Floating Controls
    const btnMobileStop = document.getElementById('btn-mobile-stop');
    const btnMobileLeave = document.getElementById('btn-mobile-leave');
    const btnMobileUpload = document.getElementById('btn-mobile-upload');
    const drawerBackdrop = document.getElementById('drawer-backdrop');
    
    if (btnMobileStop) btnMobileStop.addEventListener('click', triggerStopAll);
    if (btnMobileLeave) btnMobileLeave.addEventListener('click', leaveSession);
    if (btnMobileUpload) {
        btnMobileUpload.addEventListener('click', () => {
            document.body.classList.toggle('show-upload-drawer');
        });
    }
    if (drawerBackdrop) {
        drawerBackdrop.addEventListener('click', () => {
            document.body.classList.remove('show-upload-drawer');
        });
    }
}

// Fetch Sounds from Server
async function fetchSounds() {
    try {
        const response = await fetch('/api/v1/sounds');
        if (!response.ok) throw new Error('Falha ao buscar sons');
        soundsList = await response.json();
        renderSoundsGrid();
    } catch (err) {
        console.error(err);
        showLogMessage('Erro', 'Não foi possível carregar os sons.');
    }
}

// Fetch Active Sessions
async function fetchSessions() {
    try {
        const response = await fetch('/api/v1/sessions');
        if (!response.ok) throw new Error('Falha ao buscar sessões');
        const sessions = await response.json();
        renderSessionsList(sessions);
    } catch (err) {
        console.error(err);
    }
}

// Create New Session
async function createSession() {
    const name = inputSessionName.value.trim();
    if (!name) return;

    try {
        btnCreateSession.disabled = true;
        const response = await fetch(`/api/v1/sessions?name=${encodeURIComponent(name)}`, {
            method: 'POST'
        });
        if (!response.ok) throw new Error('Falha ao criar sessão');
        
        inputSessionName.value = '';
        await fetchSessions();
    } catch (err) {
        alert('Erro ao criar sessão: ' + err.message);
    } finally {
        btnCreateSession.disabled = false;
    }
}

// Upload New Sound
async function uploadSound(e) {
    e.preventDefault();
    const formData = new FormData(uploadForm);

    try {
        btnUpload.disabled = true;
        btnUpload.textContent = 'Carregando...';
        const response = await fetch('/api/v1/sounds', {
            method: 'POST',
            body: formData
        });

        if (!response.ok) {
            const data = await response.json();
            throw new Error(data.error || 'Erro no upload');
        }

        uploadForm.reset();
        fileNameDisplay.textContent = 'Escolher arquivo (MP3/WAV)';
        fileNameDisplay.style.color = 'var(--text-secondary)';
        document.body.classList.remove('show-upload-drawer');
        
        await fetchSounds();
        showLogMessage('Sistema', 'Novo som adicionado.');
    } catch (err) {
        alert('Erro no upload: ' + err.message);
    } finally {
        btnUpload.disabled = false;
        btnUpload.textContent = 'Carregar Som';
    }
}

// Render Sessions List
function renderSessionsList(sessions) {
    if (!sessions || sessions.length === 0) {
        sessionsList.innerHTML = `<p class="empty-state">Nenhuma sessão encontrada. Crie uma acima.</p>`;
        return;
    }

    sessionsList.innerHTML = sessions.map(sess => `
        <div class="session-item">
            <div class="session-info">
                <span class="session-name">${escapeHTML(sess.name)}</span>
            </div>
            <div class="session-actions">
                <button class="btn-vercel-primary" onclick="joinSession('${sess.id}', 'server')">Hospedar</button>
                <button class="btn-vercel-secondary" onclick="joinSession('${sess.id}', 'client')">Controle</button>
            </div>
        </div>
    `).join('');
}

// Render Sounds Grid
function renderSoundsGrid() {
    if (!soundsList || soundsList.length === 0) {
        soundsGrid.innerHTML = `<p class="empty-state">Biblioteca vazia. Adicione sons acima.</p>`;
        return;
    }

    soundsGrid.innerHTML = soundsList.map(snd => `
        <div class="sound-pad" id="pad-${snd.id}" onclick="triggerPlay('${snd.id}')">
            <span class="sound-pad-icon">🎵</span>
            <span class="sound-pad-name" title="${escapeHTML(snd.name)}">${escapeHTML(snd.name)}</span>
        </div>
    `).join('');
}

// Join WebSocket Session
function joinSession(sessionId, role) {
    if (ws) {
        leaveSession();
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/api/v1/sessions/${sessionId}/ws?role=${role}`;

    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        currentSession = sessionId;
        currentRole = role;
        
        // Add session active tag to body for responsive layout handling
        document.body.classList.add('session-active');
        
        // Update connection status UI
        connectionStatus.className = 'status-badge connected';
        connectionStatus.querySelector('.status-text').textContent = 'Conectado';
        
        const dockStatus = document.getElementById('dock-status');
        if (dockStatus) {
            dockStatus.className = 'dock-status connected';
            dockStatus.title = 'Conectado';
        }
        
        // Enable Soundboard Deck
        deckCard.classList.remove('disabled');
        
        // Setup session titles and badges
        const sessItem = document.querySelector(`.session-item button[onclick*="${sessionId}"]`);
        const sessionNameText = sessItem ? sessItem.closest('.session-item').querySelector('.session-name').textContent : 'Sessão Compartilhada';
        currentSessionTitle.textContent = sessionNameText;
        
        roleBadge.textContent = role === 'server' ? 'Hospedeiro' : 'Controle';
        roleBadge.className = `role-badge ${role === 'server' ? 'server-role' : 'client-role'}`;
        roleSelectorContainer.style.display = 'flex';

        // Toggle equalizers
        if (role === 'server') {
            serverStatusView.style.display = 'flex';
        } else {
            serverStatusView.style.display = 'none';
        }

        showLogMessage('Sistema', `Conectado à sessão como ${role === 'server' ? 'Hospedeiro' : 'Controle'}.`);
    };

    ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            handleWSMessage(data);
        } catch (err) {
            console.error('Erro ao processar mensagem ws:', err);
        }
    };

    ws.onclose = () => {
        showLogMessage('Sistema', 'Conexão com a sessão encerrada.');
        resetSessionUI();
    };

    ws.onerror = (err) => {
        console.error('Erro na conexão websocket:', err);
        showLogMessage('Erro', 'Erro na conexão em tempo real.');
        resetSessionUI();
    };
}

// Leave Current Session
function leaveSession() {
    if (ws) {
        ws.close();
        ws = null;
    }
    stopAllLocalAudios();
}

// Reset Session UI to Default
function resetSessionUI() {
    document.body.classList.remove('session-active');
    document.body.classList.remove('show-upload-drawer');
    connectionStatus.className = 'status-badge disconnected';
    connectionStatus.querySelector('.status-text').textContent = 'Sem Conexão';
    
    const dockStatus = document.getElementById('dock-status');
    if (dockStatus) {
        dockStatus.className = 'dock-status disconnected';
        dockStatus.title = 'Sem Conexão';
    }
    
    deckCard.classList.add('disabled');
    currentSessionTitle.textContent = 'Escolha uma sessão';
    roleSelectorContainer.style.display = 'none';
    serverStatusView.style.display = 'none';
    currentSession = null;
    currentRole = null;
}

// Trigger play action (REST or Websocket depending on context)
function triggerPlay(soundId) {
    if (!currentSession) {
        alert('Por favor, conecte-se a uma sessão primeiro.');
        return;
    }

    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
            type: 'play',
            sound_id: soundId
        }));
    } else {
        triggerPlayREST(soundId);
    }
}

// Fallback REST trigger play
async function triggerPlayREST(soundId) {
    try {
        const response = await fetch(`/api/v1/sessions/${currentSession}/play`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ sound_id: soundId })
        });
        if (!response.ok) throw new Error('Erro ao disparar áudio');
    } catch (err) {
        showLogMessage('Erro', 'Erro ao disparar o áudio.');
    }
}

// Trigger Stop All
function triggerStopAll() {
    if (!currentSession) return;

    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
            type: 'stop'
        }));
    } else {
        triggerStopAllREST();
    }
}

// Fallback REST Stop All
async function triggerStopAllREST() {
    try {
        const response = await fetch(`/api/v1/sessions/${currentSession}/stop`, {
            method: 'POST'
        });
        if (!response.ok) throw new Error('Erro ao parar reprodução');
    } catch (err) {
        showLogMessage('Erro', 'Erro ao parar reprodução.');
    }
}

// Stop all local audio files playing in browser
function stopAllLocalAudios() {
    activeAudios.forEach(audio => {
        audio.pause();
        audio.currentTime = 0;
    });
    activeAudios = [];
}

// Handle incoming websocket events
function handleWSMessage(msg) {
    if (msg.type === 'sound_played') {
        // Find pad in UI and animate
        const pad = document.getElementById(`pad-${msg.soundID}`);
        if (pad) {
            pad.classList.add('playing');
            setTimeout(() => {
                pad.classList.remove('playing');
            }, 300);
        }

        // Play the audio locally in the browser of the client acting as 'server'
        if (currentRole === 'server') {
            serverStatusView.classList.add('playing');
            setTimeout(() => {
                serverStatusView.classList.remove('playing');
            }, 600);

            if (msg.file_path) {
                // First stop any currently playing local audio to avoid overlaps
                stopAllLocalAudios();

                const audioUrl = '/' + msg.file_path;
                const audioObj = new Audio(audioUrl);
                
                // Track active audio
                activeAudios.push(audioObj);
                audioObj.onended = () => {
                    activeAudios = activeAudios.filter(a => a !== audioObj);
                };
                
                audioObj.play().catch(err => {
                    console.warn('Reprodução pelo navegador bloqueada:', err);
                    showLogMessage('Sistema', 'Clique na página para autorizar som.');
                    activeAudios = activeAudios.filter(a => a !== audioObj);
                });
            }
        }

        // Add to activity log
        showLogMessage(
            msg.is_server ? 'Hospedeiro' : 'Controle', 
            `Disparou: <strong>"${escapeHTML(msg.soundName)}"</strong>`
        );
    } else if (msg.type === 'stop_all') {
        // Stop all active audios in browser
        stopAllLocalAudios();

        // Visual feedback
        showLogMessage('Sistema', '<strong>Reprodução interrompida</strong>');
    } else if (msg.type === 'error') {
        showLogMessage('Erro', msg.message);
    }
}

// Helper: Show activity log entry
function showLogMessage(source, message) {
    const emptyMsg = activityLog.querySelector('.log-empty');
    if (emptyMsg) emptyMsg.remove();

    const entry = document.createElement('div');
    entry.className = 'log-entry';

    const now = new Date();
    const timeStr = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });

    entry.innerHTML = `
        <span class="log-time">[${timeStr}]</span>
        <span class="log-source">[${source}]</span>
        <span class="log-msg">${message}</span>
    `;

    activityLog.appendChild(entry);
    activityLog.scrollTop = activityLog.scrollHeight;
}

// Helper: Escape HTML string to prevent XSS
function escapeHTML(str) {
    return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}
