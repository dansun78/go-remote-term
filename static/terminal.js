document.addEventListener('DOMContentLoaded', () => {
    const terminal = document.getElementById('terminal');
    const statusDisplay = document.getElementById('status');
    const sessionInfo = document.getElementById('sessionInfo');
    const connectionIndicator = document.getElementById('connectionIndicator');
    const newSessionBtn = document.getElementById('newSessionBtn');
    const terminateBtn = document.getElementById('terminateBtn');
    
    let socket = null;
    let sessionId = null; // Store the session ID for reconnection
    let reconnectAttempts = 0;
    const maxReconnectAttempts = 5;
    let reconnectTimer = null;
    
    // Make terminal div focusable
    terminal.tabIndex = 0;
    
    // Focus terminal on click
    terminal.addEventListener('click', () => {
        terminal.focus();
    });
    
    // Show focus indicator
    terminal.addEventListener('focus', () => {
        terminal.style.outline = '2px solid #4CAF50';
    });
    
    terminal.addEventListener('blur', () => {
        terminal.style.outline = 'none';
    });
    
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
                // Check if the message is JSON (control message)
                try {
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
                        
                        // Only clear terminal if it's a new session (not a reconnection)
                        if (!existingSessionId) {
                            terminal.textContent = '';
                        }
                        
                        newSessionBtn.disabled = false;
                        terminateBtn.disabled = false;
                        terminal.focus();
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
                } catch (e) {
                    // Not JSON, treat as normal terminal output
                }
                
                const text = event.data;
                // Process terminal output with ANSI handling
                processTerminalOutput(text);
                terminal.scrollTop = terminal.scrollHeight; // Auto-scroll to bottom
            };
        } catch (error) {
            console.error('Failed to connect:', error);
            statusDisplay.textContent = 'Connection failed';
            statusDisplay.style.color = 'red';
            updateConnectionIndicator('disconnected');
            newSessionBtn.disabled = false;
        }
    }
    
    // Handle various terminal control sequences
    function processTerminalOutput(text) {
        // Filter out problematic sequences
        let processedText = text;
        
        // Filter out bracketed paste mode sequences (including the caret representation)
        processedText = processedText.replace(/\^\[\[\?2004[hl]/g, '');  // ^[[?2004h or ^[[?2004l
        processedText = processedText.replace(/\[\?2004[hl]/g, '');      // [?2004h or [?2004l
        
        // Check if we're getting ANSI color/formatting codes
        if (processedText.includes('\x1b[') || processedText.includes('^[')) {
            // Convert the ANSI escape sequences to plain text
            // This simple approach removes ANSI codes rather than interpreting them
            processedText = processedText.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, ''); // Remove standard ANSI sequences
            processedText = processedText.replace(/\^\[\[[0-9;]*[a-zA-Z]/g, ''); // Remove caret representation
        }
        
        // Handle other control characters
        processedText = processedText.replace(/\x1b/g, ''); // Remove ESC character
        
        // Handle terminal title sequences (common in bash)
        processedText = processedText.replace(/\]0;.*?\x07/g, '');
        
        // Ensure we handle HTML line breaks properly for output rendering
        processedText = processedText.replace(/\r\n/g, '\n'); // Normalize line endings
        
        // Append the processed text to terminal
        if (processedText.length > 0) {
            terminal.textContent += processedText;
        }
    }
    
    // Handle window resize to update terminal size
    window.addEventListener('resize', () => {
        if (socket && socket.readyState === WebSocket.OPEN && sessionId) {
            // Calculate terminal size based on character dimensions
            const testChar = document.createElement('span');
            testChar.textContent = 'X';
            testChar.style.visibility = 'hidden';
            terminal.appendChild(testChar);
            
            const charWidth = testChar.getBoundingClientRect().width;
            const charHeight = testChar.getBoundingClientRect().height;
            
            terminal.removeChild(testChar);
            
            const rows = Math.floor(terminal.clientHeight / charHeight);
            const cols = Math.floor(terminal.clientWidth / charWidth);
            
            if (rows > 0 && cols > 0) {
                // Send resize message
                socket.send(JSON.stringify({
                    type: 'resize',
                    rows: rows,
                    cols: cols
                }));
            }
        }
    });
    
    // Handle session termination (renamed from disconnect)
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
    
    // Handle keyboard input
    terminal.addEventListener('keydown', (e) => {
        if (!socket || socket.readyState !== WebSocket.OPEN) return;
        
        // Convert key combination to terminal control sequences
        let data = '';
        
        if (e.key === 'Enter') {
            data = '\r';  // Send carriage return only - PTY will handle line feed
            e.preventDefault();
        } else if (e.key === 'Backspace') {
            data = '\x7F';  // DEL character for better bash compatibility
            e.preventDefault();
        } else if (e.key === 'Tab') {
            data = '\t';
            e.preventDefault(); // Prevent tab from changing focus
        } else if (e.key === 'Escape') {
            data = '\x1B';
            e.preventDefault();
        } else if (e.key === 'ArrowUp') {
            data = '\x1B[A';
            e.preventDefault();
        } else if (e.key === 'ArrowDown') {
            data = '\x1B[B';
            e.preventDefault();
        } else if (e.key === 'ArrowRight') {
            data = '\x1B[C';
            e.preventDefault();
        } else if (e.key === 'ArrowLeft') {
            data = '\x1B[D';
            e.preventDefault();
        } else if (e.ctrlKey) {
            // Handle Ctrl+key combinations more precisely
            if (e.key.length === 1) {
                const keyCode = e.key.toUpperCase().charCodeAt(0);
                if (keyCode >= 65 && keyCode <= 90) { // A-Z
                    data = String.fromCharCode(keyCode - 64);
                    e.preventDefault();
                }
            } else if (e.key === 'c') {
                data = '\x03'; // Ctrl+C (ETX)
                e.preventDefault();
            } else if (e.key === 'd') {
                data = '\x04'; // Ctrl+D (EOT)
                e.preventDefault();
            } else if (e.key === 'z') {
                data = '\x1A'; // Ctrl+Z (SUB)
                e.preventDefault();
            }
        } else if (e.key.length === 1) {
            // Regular character input
            data = e.key;
        }
        
        if (data && socket.readyState === WebSocket.OPEN) {
            socket.send(data);
        }
    });
    
    // Make sure we capture all keyboard events
    terminal.addEventListener('keypress', (e) => {
        e.preventDefault(); // Prevent default to handle all input manually
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