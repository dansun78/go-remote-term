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
    let inputHandler = null; // Store reference to current input handler disposable to prevent duplicates
    let resizeHandler = null; // Store reference to current resize handler disposable to prevent duplicates
    
    // NOTE: These handler references are critical to prevent duplicate event bindings
    // which previously caused keystrokes to be sent multiple times when creating 
    // new sessions after terminating old ones
    
    // Initialize xterm.js with colors matching our dark theme
    const term = new Terminal({
        cursorBlink: true,
        theme: {
            background: '#2b2b2b',
            foreground: '#f0f0f0',
            cursor: '#4CAF50',
            cursorAccent: '#2b2b2b',
            selection: 'rgba(76, 175, 80, 0.3)',
            black: '#2b2b2b',
            red: '#ff6b6b',
            green: '#4CAF50',
            yellow: '#ffaa33',
            blue: '#2196F3',
            magenta: '#c678dd',
            cyan: '#56b6c2',
            white: '#e0e0e0',
            brightBlack: '#555555',
            brightRed: '#ff8585',
            brightGreen: '#45a049',
            brightYellow: '#ffb74d',
            brightBlue: '#0b7dda',
            brightMagenta: '#d992e9',
            brightCyan: '#6cc8d4',
            brightWhite: '#ffffff'
        },
        allowTransparency: true,
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
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
    
    // Helper function to cleanup terminal handlers and socket
    function cleanupTerminalState() {
        console.log("Cleaning up terminal state");
        // Dispose of event handlers
        if (inputHandler) {
            inputHandler.dispose();
            inputHandler = null;
        }
        if (resizeHandler) {
            resizeHandler.dispose();
            resizeHandler = null;
        }
    }
    
    // Helper function to fully reset connection state
    function resetConnectionState() {
        console.log("Resetting connection state");
        cleanupTerminalState();
        clearTimeout(reconnectTimer);
        reconnectAttempts = 0;
        
        // Explicitly null out socket reference if it exists
        if (socket) {
            if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
                socket.close(1000);
            }
            socket = null;
        }
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
        // Clean up any existing handlers
        cleanupTerminalState();
        
        console.log("Connecting to terminal, existing session:", existingSessionId, "socket state:", socket ? socket.readyState : "no socket");
        
        // Close existing connection if any
        if (socket && socket.readyState !== WebSocket.CLOSED) {
            console.log("Closing existing socket connection...");
            // Add a listener for the close event
            const onSocketClose = () => {
                console.log("Socket closed, now creating new connection");
                socket.removeEventListener('close', onSocketClose);
                setTimeout(() => createConnection(existingSessionId), 300);
            };
            
            socket.addEventListener('close', onSocketClose);
            socket.close(1000);
            return;
        }
        
        console.log("No active socket, creating connection immediately");
        // Immediately create connection if no active socket
        createConnection(existingSessionId);
        
        function createConnection(sessionId) {
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
            statusDisplay.textContent = sessionId ? 'Reconnecting...' : 'Connecting...';
            statusDisplay.style.color = 'orange';
            updateConnectionIndicator('reconnecting');
            newSessionBtn.disabled = true;
            
            // Clear the terminal if this is a new session
            if (!sessionId) {
                term.clear();
            }
            
            try {
                socket = new WebSocket(wsUrl);
                
                // Reset any socket flags used for state tracking
                socket._forceClosing = false;
                socket._forCreatingNewSession = false;
                
                socket.onopen = () => {
                    // Send authentication token as first message after connection
                    // Include session ID if we're reconnecting to an existing session
                    const authMessage = {
                        type: 'auth',
                        token: token
                    };
                    
                    if (sessionId) {  // Use the passed sessionId parameter
                        authMessage.session_id = sessionId;
                    }
                    
                    socket.send(JSON.stringify(authMessage));
                };
                
                socket.onclose = (event) => {
                    console.log("WebSocket closed with code:", event.code, "reason:", event.reason);
                    const normalClose = event.code === 1000 || event.code === 1001;
                    
                    // Update button states
                    terminateBtn.disabled = true;
                    
                    // Clean up event handlers on socket close to prevent duplicates on reconnection
                    cleanupTerminalState();
                    
                    // Check if we're in the middle of termination process or creating a new session
                    if (socket && (socket._forceClosing || socket._forCreatingNewSession)) {
                        console.log("Socket intentionally closed:", 
                                   socket._forceClosing ? "forced termination" : "for new session");
                        // Don't reconnect, we're intentionally closing 
                        // (either for termination or to create a new session)
                        newSessionBtn.disabled = false;
                        return;
                    }
                    
                    // If we have a session ID and this wasn't a normal close, try to reconnect
                    // unless sessionId has been cleared (which indicates an intentional termination)
                    if (sessionId && !normalClose) {
                        console.log("Abnormal close with active session - scheduling reconnect");
                        scheduleReconnect();
                    } else {
                        console.log("Normal close or no session - enabling new session button");
                        // Enable new session button for normal closes or when session is already null
                        newSessionBtn.disabled = false;
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
                            
                            // Clean up event handlers when shell process exits
                            if (inputHandler) {
                                inputHandler.dispose();
                                inputHandler = null;
                            }
                            if (resizeHandler) {
                                resizeHandler.dispose();
                                resizeHandler = null;
                            }
                            
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
                        
                        statusDisplay.textContent = sessionId ? 'Reconnected' : 'Connected';
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
                            console.log("Session successfully terminated by server");
                            
                            // Clean up event handlers when session is successfully terminated
                            cleanupTerminalState();
                            
                            // Clear session since it's now terminated
                            clearSavedSession();
                            sessionId = null;
                            
                            // Update UI state now that termination is confirmed
                            statusDisplay.textContent = 'Session terminated';
                            statusDisplay.style.color = 'red';
                            updateConnectionIndicator('disconnected');
                            newSessionBtn.disabled = false;
                            terminateBtn.disabled = true;
                            
                            // Show message in terminal
                            term.write('\r\n\x1b[33mSession terminated by user\x1b[0m\r\n');
                            
                            // Close the WebSocket connection with a delay
                            setTimeout(() => {
                                if (socket && socket.readyState === WebSocket.OPEN) {
                                    console.log("Closing connection after successful termination");
                                    socket._forceClosing = true;
                                    socket.close(1000);
                                }
                            }, 200);
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
                        
                        // Clean up event handlers when shell process exits
                        if (inputHandler) {
                            inputHandler.dispose();
                            inputHandler = null;
                        }
                        if (resizeHandler) {
                            resizeHandler.dispose();
                            resizeHandler = null;
                        }
                        
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
            
            // Clean up previous event handlers if they exist
            if (inputHandler) {
                inputHandler.dispose();
                inputHandler = null;
            }
            if (resizeHandler) {
                resizeHandler.dispose();
                resizeHandler = null;
            }
            
            // Create new handlers and store their disposables
            // Note: term.onData and term.onResize return disposable objects
            inputHandler = term.onData(data => {
                if (socket && socket.readyState === WebSocket.OPEN) {
                    socket.send(data);
                }
            });
            
            // Create new resize handler
            resizeHandler = term.onResize(size => {
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
        console.log("Terminate button clicked");
        
        // Clear reconnection logic
        clearTimeout(reconnectTimer);
        reconnectAttempts = 0;
        
        // If we have an active session, send a terminate message
        if (socket && socket.readyState === WebSocket.OPEN && sessionId) {
            statusDisplay.textContent = 'Terminating session...';
            statusDisplay.style.color = 'orange';
            
            // Temporarily disable the new session button until termination completes
            newSessionBtn.disabled = true;
            
            // Add a listener to ensure we can create a new session once current one is fully closed
            socket.addEventListener('close', () => {
                console.log("Socket closed after termination");
                newSessionBtn.disabled = false;
            }, { once: true });
            
            // Send termination request
            console.log("Sending termination request for session:", sessionId);
            socket.send(JSON.stringify({
                type: 'terminate',
                sessionID: sessionId
            }));
            
            // Set a timeout to close the socket if the server doesn't respond
            setTimeout(() => {
                if (socket && socket.readyState === WebSocket.OPEN) {
                    console.log("Termination response timeout - force closing");
                    // Store a flag to indicate we're force-closing
                    socket._forceClosing = true;
                    socket.close(1000, 'Session terminated by user');
                    
                    // Only if we had to force-close, update UI here
                    statusDisplay.textContent = 'Session terminated (forced)';
                    statusDisplay.style.color = 'red';
                    updateConnectionIndicator('disconnected');
                    newSessionBtn.disabled = false;
                    terminateBtn.disabled = true;
                    
                    // Clear session info since it's now terminated
                    clearSavedSession();
                    sessionId = null;
                }
            }, 1000);
            
            // Note: We don't update UI state or clear session here
            // This will be done when we receive the terminate_response
        } else if (socket) {
            // Just close the socket if no session
            socket.close(1000, 'User disconnected');
            
            // Reset UI immediately in this case
            statusDisplay.textContent = 'Disconnected';
            statusDisplay.style.color = 'red';
            updateConnectionIndicator('disconnected');
            newSessionBtn.disabled = false;
            terminateBtn.disabled = true;
            
            // Clear session info
            clearSavedSession();
            sessionId = null;
        }
    });
    
    // Handle new session creation
    newSessionBtn.addEventListener('click', () => {
        console.log("New session button clicked");
        
        // Clear saved session
        clearSavedSession();
        sessionId = null;
        
        // Reset connection state completely
        resetConnectionState();
        
        // Set status immediately so user knows something is happening
        statusDisplay.textContent = 'Creating new session...';
        statusDisplay.style.color = 'orange';
        
        // If there's an existing socket, close it properly first
        if (socket && socket.readyState !== WebSocket.CLOSED) {
            console.log("Closing existing socket before creating new session");
            socket._forCreatingNewSession = true;
            
            // Set flag to indicate we're explicitly closing this socket for a new session
            socket._forCreatingNewSession = true;
            
            // Register one-time close listener
            const createNewSession = () => {
                console.log("Socket closed, now creating new session");
                
                // Remove the event listener to prevent memory leaks
                if (socket) {
                    socket.removeEventListener('close', createNewSession);
                }
                
                // Completely reset the socket
                socket = null;
                
                // Add delay to ensure WebSocket state is fully cleaned up
                setTimeout(() => connectToTerminal(null), 500);
            };
            
            // Add the listener and close the socket
            socket.addEventListener('close', createNewSession, { once: true });
            socket.close(1000, 'Starting new session');
        } else {
            console.log("No active socket, creating new session immediately");
            // If no socket or already closed, connect immediately
            connectToTerminal(null);
        }
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