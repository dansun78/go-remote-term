document.addEventListener('DOMContentLoaded', () => {
    const terminalElement = document.getElementById('terminal');
    const statusDisplay = document.getElementById('status');
    const sessionInfo = document.getElementById('sessionInfo');
    const connectionIndicator = document.getElementById('connectionIndicator');
    const newSessionBtn = document.getElementById('newSessionBtn');
    const terminateBtn = document.getElementById('terminateBtn');
    const fullscreenBtn = document.getElementById('fullscreenBtn');
    
    let socket = null;
    let sessionId = null; // Store the session ID for reconnection
    let reconnectAttempts = 0;
    const maxReconnectAttempts = 5;
    let reconnectTimer = null;
    let isFullscreen = false;
    
    // Initialize xterm.js
    const term = new Terminal({
        cursorBlink: true,
        theme: {
            background: '#000',
            foreground: '#0f0',
            cursor: '#0f0'
        },
        allowTransparency: true,
        fontFamily: 'Courier New, monospace',
        fontSize: 14,
        scrollback: 1000
    });
    
    // Initialize the fit addon to resize the terminal
    const fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);
    
    // Open the terminal
    term.open(terminalElement);
    fitAddon.fit();
    
    // Helper function to get a cookie value
    function getCookie(name) {
        const value = `; ${document.cookie}`;
        const parts = value.split(`; ${name}=`);
        if (parts.length === 2) return parts.pop().split(';').shift();
        return null;
    }
    
    // Get token from URL parameters or cookie
    function getAuthToken() {
        // First try URL parameter
        const urlParams = new URLSearchParams(window.location.search);
        const tokenParam = urlParams.get('token');
        
        if (tokenParam) {
            return tokenParam;
        }
        
        // Then try cookie
        return getCookie('auth_token');
    }
    
    // Store session ID in localStorage
    function saveSession(id) {
        if (id) {
            localStorage.setItem('terminal_session_id', id);
            // Update session info display
            updateSessionInfo(id);
        }
    }
    
    // Get saved session ID if available
    function getSavedSession() {
        const id = localStorage.getItem('terminal_session_id');
        if (id) {
            updateSessionInfo(id);
        }
        return id;
    }
    
    // Clear saved session
    function clearSavedSession() {
        localStorage.removeItem('terminal_session_id');
        updateSessionInfo(null);
    }
    
    // Update session info display
    function updateSessionInfo(id) {
        if (id) {
            const shortId = id.substring(0, 8); // Just show first part of UUID
            sessionInfo.textContent = `Session: ${shortId}...`;
            sessionInfo.title = `Full session ID: ${id}`;
        } else {
            sessionInfo.textContent = 'No active session';
            sessionInfo.title = '';
        }
    }
    
    // Update connection indicator
    function updateConnectionIndicator(status) {
        // Remove all classes first
        connectionIndicator.classList.remove('connected', 'reconnecting');
        
        switch (status) {
            case 'connected':
                connectionIndicator.classList.add('connected');
                break;
            case 'reconnecting':
                connectionIndicator.classList.add('reconnecting');
                break;
            default:
                // Default is disconnected (red)
                break;
        }
    }
    
    // Auto reconnect function with exponential backoff
    function scheduleReconnect() {
        if (reconnectAttempts >= maxReconnectAttempts) {
            statusDisplay.textContent = 'Reconnection failed after multiple attempts';
            updateConnectionIndicator('disconnected');
            return;
        }
        
        const delay = Math.min(30000, Math.pow(2, reconnectAttempts) * 1000); // Exponential backoff with 30s max
        reconnectAttempts++;
        
        statusDisplay.textContent = `Connection lost. Reconnecting in ${Math.round(delay/1000)}s... (${reconnectAttempts}/${maxReconnectAttempts})`;
        statusDisplay.style.color = 'orange';
        updateConnectionIndicator('reconnecting');
        
        clearTimeout(reconnectTimer);
        reconnectTimer = setTimeout(() => {
            if (sessionId) {
                connectToTerminal(sessionId);
            }
        }, delay);
    }
    
    // Create WebSocket and connect to terminal
    function connectToTerminal(existingSessionId = null) {
        // Close existing connection if any
        if (socket) {
            socket.close();
        }
        
        // Get authentication token
        const token = getAuthToken();
        if (!token) {
            statusDisplay.textContent = 'Authentication token missing';
            statusDisplay.style.color = 'red';
            updateConnectionIndicator('disconnected');
            return;
        }
        
        // Get the current host and construct WebSocket URL
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        // Update UI to show connecting state
        statusDisplay.textContent = existingSessionId ? 'Reconnecting...' : 'Connecting...';
        statusDisplay.style.color = 'orange';
        updateConnectionIndicator('reconnecting');
        newSessionBtn.disabled = true;
        
        // Clear the terminal if this is a new session
        if (!existingSessionId) {
            term.clear();
        }
        
        // Create new WebSocket connection
        try {
            socket = new WebSocket(wsUrl);
            
            socket.onopen = () => {
                // Send authentication token as first message after connection
                // Include session ID if we're reconnecting to an existing session
                const authMessage = {
                    type: 'auth',
                    token: token
                };
                
                if (existingSessionId) {
                    authMessage.session_id = existingSessionId;
                }
                
                socket.send(JSON.stringify(authMessage));
            };
            
            socket.onclose = (event) => {
                const normalClose = event.code === 1000 || event.code === 1001;
                newSessionBtn.disabled = false;
                terminateBtn.disabled = true;
                
                // If we have a session ID and this wasn't a normal close, try to reconnect
                if (sessionId && !normalClose) {
                    scheduleReconnect();
                } else {
                    statusDisplay.textContent = normalClose ? 'Disconnected' : `Connection closed: ${event.code}`;
                    statusDisplay.style.color = 'red';
                    updateConnectionIndicator('disconnected');
                }
            };
            
            socket.onerror = (error) => {
                console.error('WebSocket error:', error);
                statusDisplay.textContent = 'Connection error';
                statusDisplay.style.color = 'red';
                updateConnectionIndicator('disconnected');
            };
            
            socket.onmessage = (event) => {
                try {
                    // First check if this is a JSON message from our PTY output buffer hack
                    // which will be wrapped in <JSON>...</JSON> tags
                    const jsonMatch = event.data.match(/<JSON>(.*?)<\/JSON>/s);
                    if (jsonMatch && jsonMatch[1]) {
                        // Extract and parse the JSON
                        const jsonData = JSON.parse(jsonMatch[1]);
                        
                        // Handle the message based on its type
                        if (jsonData.type === 'session_ended') {
                            console.log("Shell process exited for session:", jsonData.sessionID);
                            
                            // Show message to user
                            term.write('\r\n\x1b[31mShell process has exited. Session terminated.\x1b[0m\r\n');
                            statusDisplay.textContent = 'Shell exited';
                            statusDisplay.style.color = 'orange';
                            
                            // Clear saved session
                            clearSavedSession();
                            sessionId = null;
                            
                            // Update UI state
                            newSessionBtn.disabled = false;
                            terminateBtn.disabled = true;
                            updateConnectionIndicator('disconnected');
                            
                            // Close socket connection
                            setTimeout(() => {
                                if (socket && socket.readyState === WebSocket.OPEN) {
                                    socket.close(1000, 'Shell process exited');
                                }
                            }, 100);
                            
                            // Remove the JSON wrapper from the terminal output
                            const cleanedData = event.data.replace(/<JSON>.*?<\/JSON>/s, '');
                            if (cleanedData.trim()) {
                                term.write(cleanedData);
                            }
                            return;
                        }
                        
                        // If we reach here, we didn't handle the wrapped JSON specifically
                        // Remove the JSON wrapper and continue processing as normal text
                        const cleanedData = event.data.replace(/<JSON>.*?<\/JSON>/s, '');
                        if (cleanedData.trim()) {
                            term.write(cleanedData);
                        }
                        return;
                    }
                    
                    // Next, check if the entire message is JSON
                    const data = JSON.parse(event.data);
                    
                    if (data.type === 'auth_response') {
                        if (!data.success) {
                            statusDisplay.textContent = 'Authentication failed: ' + data.message;
                            statusDisplay.style.color = 'red';
                            // Clear session if authentication fails
                            clearSavedSession();
                            sessionId = null;
                            socket.close();
                            updateConnectionIndicator('disconnected');
                            return;
                        }
                        
                        // Authentication succeeded
                        // Store the session ID for reconnection
                        sessionId = data.session_id;
                        saveSession(sessionId);
                        
                        // Reset reconnect attempts on successful connection
                        reconnectAttempts = 0;
                        clearTimeout(reconnectTimer);
                        
                        statusDisplay.textContent = existingSessionId ? 'Reconnected' : 'Connected';
                        statusDisplay.style.color = 'green';
                        updateConnectionIndicator('connected');
                        
                        newSessionBtn.disabled = false;
                        terminateBtn.disabled = false;
                        term.focus();
                        return; // Don't process auth responses as terminal output
                    } 
                    // Add handling for terminate_response
                    else if (data.type === 'terminate_response') {
                        if (data.success) {
                            console.log("Session successfully terminated");
                            
                            // Clear session since it's now terminated
                            clearSavedSession();
                            sessionId = null;
                        }
                        return; // Don't process as terminal output
                    }
                    // Add handling for session_ended (when shell exits naturally)
                    else if (data.type === 'session_ended') {
                        console.log("Shell process exited for session:", data.sessionID);
                        
                        // Show message to user
                        term.write('\r\n\x1b[31mShell process has exited. Session terminated.\x1b[0m\r\n');
                        statusDisplay.textContent = 'Shell exited';
                        statusDisplay.style.color = 'orange';
                        
                        // Clear saved session
                        clearSavedSession();
                        sessionId = null;
                        
                        // Update UI state
                        newSessionBtn.disabled = false;
                        terminateBtn.disabled = true;
                        updateConnectionIndicator('disconnected');
                        
                        // Close socket connection
                        setTimeout(() => {
                            if (socket && socket.readyState === WebSocket.OPEN) {
                                socket.close(1000, 'Shell process exited');
                            }
                        }, 100);
                        
                        return; // Don't process as terminal output
                    }
                } catch (e) {
                    // Not JSON, treat as normal terminal output
                    // Simply write raw data to the terminal - xterm.js handles ANSI codes
                    term.write(event.data);
                }
            };
            
            // Handle terminal input
            term.onData(data => {
                if (socket && socket.readyState === WebSocket.OPEN) {
                    socket.send(data);
                }
            });
            
            // Handle terminal resize
            term.onResize(size => {
                if (socket && socket.readyState === WebSocket.OPEN && sessionId) {
                    socket.send(JSON.stringify({
                        type: 'resize',
                        rows: size.rows,
                        cols: size.cols
                    }));
                }
            });
            
        } catch (error) {
            console.error('Failed to connect:', error);
            statusDisplay.textContent = 'Connection failed';
            statusDisplay.style.color = 'red';
            updateConnectionIndicator('disconnected');
            newSessionBtn.disabled = false;
        }
    }
    
    // Toggle fullscreen mode
    function toggleFullscreen() {
        const container = document.body;
        
        if (!isFullscreen) {
            // Enter fullscreen
            if (container.requestFullscreen) {
                container.requestFullscreen();
            } else if (container.mozRequestFullScreen) { // Firefox
                container.mozRequestFullScreen();
            } else if (container.webkitRequestFullscreen) { // Chrome, Safari & Opera
                container.webkitRequestFullscreen();
            } else if (container.msRequestFullscreen) { // IE/Edge
                container.msRequestFullscreen();
            }
        } else {
            // Exit fullscreen
            if (document.exitFullscreen) {
                document.exitFullscreen();
            } else if (document.mozCancelFullScreen) {
                document.mozCancelFullScreen();
            } else if (document.webkitExitFullscreen) {
                document.webkitExitFullscreen();
            } else if (document.msExitFullscreen) {
                document.msExitFullscreen();
            }
        }
    }
    
    // Update UI for fullscreen mode
    function updateFullscreenUI(isFullscreenActive) {
        isFullscreen = isFullscreenActive;
        
        if (isFullscreenActive) {
            document.body.classList.add('fullscreen-mode');
            fullscreenBtn.classList.add('active');
        } else {
            document.body.classList.remove('fullscreen-mode');
            fullscreenBtn.classList.remove('active');
        }
        
        // Resize terminal to fit the new container size
        setTimeout(() => {
            fitAddon.fit();
        }, 100);
    }
    
    // Listen for fullscreen change events
    document.addEventListener('fullscreenchange', () => {
        updateFullscreenUI(!!document.fullscreenElement);
    });
    document.addEventListener('webkitfullscreenchange', () => {
        updateFullscreenUI(!!document.webkitFullscreenElement);
    });
    document.addEventListener('mozfullscreenchange', () => {
        updateFullscreenUI(!!document.mozFullscreenElement);
    });
    document.addEventListener('MSFullscreenChange', () => {
        updateFullscreenUI(!!document.msFullscreenElement);
    });
    
    // Handle fullscreen button click
    fullscreenBtn.addEventListener('click', toggleFullscreen);
    
    // Handle keyboard shortcut for fullscreen (F11)
    document.addEventListener('keydown', (e) => {
        if (e.key === 'F11') {
            e.preventDefault();
            toggleFullscreen();
        }
        
        // ESC key in fullscreen mode will exit fullscreen
        if (e.key === 'Escape' && isFullscreen) {
            // Note: Most browsers automatically exit fullscreen on ESC
            // This is just a safeguard
            if (document.exitFullscreen) {
                document.exitFullscreen();
            }
        }
    });
    
    // Handle window resize to resize the terminal
    window.addEventListener('resize', () => {
        fitAddon.fit();
    });
    
    // Handle session termination
    terminateBtn.addEventListener('click', () => {
        // Clear reconnection logic
        clearTimeout(reconnectTimer);
        reconnectAttempts = 0;
        
        // If we have an active session, send a terminate message
        if (socket && socket.readyState === WebSocket.OPEN && sessionId) {
            statusDisplay.textContent = 'Terminating session...';
            statusDisplay.style.color = 'orange';
            
            // Send termination request
            socket.send(JSON.stringify({
                type: 'terminate',
                sessionID: sessionId
            }));
            
            // Set a timeout to close the socket if the server doesn't respond
            setTimeout(() => {
                if (socket && socket.readyState === WebSocket.OPEN) {
                    socket.close(1000, 'Session terminated by user');
                }
            }, 1000);
        } else if (socket) {
            // Just close the socket if no session
            socket.close(1000, 'User disconnected');
        }
        
        // Reset UI
        statusDisplay.textContent = 'Session terminated';
        statusDisplay.style.color = 'red';
        updateConnectionIndicator('disconnected');
        newSessionBtn.disabled = false;
        terminateBtn.disabled = true;
        
        // Clear session info since it's now terminated
        clearSavedSession();
        sessionId = null;
    });
    
    // Handle new session creation
    newSessionBtn.addEventListener('click', () => {
        // Clear saved session
        clearSavedSession();
        sessionId = null;
        
        // Clear reconnection logic
        clearTimeout(reconnectTimer);
        reconnectAttempts = 0;
        
        // Close current socket
        if (socket) {
            socket.close(1000, 'Starting new session');
        }
        
        // Connect with no session ID to get a fresh session
        connectToTerminal();
    });
    
    // Handle beforeunload event to warn about active sessions
    window.addEventListener('beforeunload', (e) => {
        if (socket && socket.readyState === WebSocket.OPEN) {
            const message = 'You have an active terminal session. Are you sure you want to leave?';
            e.returnValue = message;
            return message;
        }
    });
    
    // Initialize connection indicator
    updateConnectionIndicator('disconnected');
    
    // Auto-connect if we have a token
    if (getAuthToken()) {
        // Try to connect with saved session if available
        const savedSession = getSavedSession();
        connectToTerminal(savedSession);
    } else {
        // Redirect to login page if no token is found
        window.location.href = '/login.html';
    }
});