document.addEventListener('DOMContentLoaded', () => {
    const terminal = document.getElementById('terminal');
    const statusDisplay = document.getElementById('status');
    const connectBtn = document.getElementById('connectBtn');
    const disconnectBtn = document.getElementById('disconnectBtn');
    
    let socket = null;
    
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
    
    // Handle WebSocket connection
    connectBtn.addEventListener('click', () => {
        // Close existing connection if any
        if (socket) {
            socket.close();
        }
        
        // Get the current host and construct WebSocket URL
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        // Create new WebSocket connection
        try {
            socket = new WebSocket(wsUrl);
            
            socket.onopen = () => {
                statusDisplay.textContent = 'Connected';
                statusDisplay.style.color = 'green';
                terminal.innerHTML = '';
                connectBtn.disabled = true;
                disconnectBtn.disabled = false;
                terminal.focus();
            };
            
            socket.onclose = () => {
                statusDisplay.textContent = 'Disconnected';
                statusDisplay.style.color = 'red';
                connectBtn.disabled = false;
                disconnectBtn.disabled = true;
            };
            
            socket.onerror = (error) => {
                console.error('WebSocket error:', error);
                statusDisplay.textContent = 'Connection error';
                statusDisplay.style.color = 'red';
            };
            
            socket.onmessage = (event) => {
                const text = event.data;
                // Process terminal output with ANSI handling
                processTerminalOutput(text);
                terminal.scrollTop = terminal.scrollHeight; // Auto-scroll to bottom
            };
        } catch (error) {
            console.error('Failed to connect:', error);
            statusDisplay.textContent = 'Connection failed';
            statusDisplay.style.color = 'red';
        }
    });
    
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
    
    // Handle disconnection
    disconnectBtn.addEventListener('click', () => {
        if (socket) {
            socket.close();
        }
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
});