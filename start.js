#!/usr/bin/env node

/**
 * Unhinged Console - Startup Script
 * This script starts the console server for development and testing
 */

const { exec } = require('child_process');
const path = require('path');

console.log('Starting Unhinged Console...');
console.log('================================');

// Check if server.js exists
const serverPath = path.join(__dirname, 'server.js');
const fs = require('fs');

if (!fs.existsSync(serverPath)) {
    console.error('Error: server.js not found');
    process.exit(1);
}

// Start the server
const server = exec('node server.js', { cwd: __dirname });

server.stdout.on('data', (data) => {
    console.log(data.toString());
});

server.stderr.on('data', (data) => {
    console.error(data.toString());
});

server.on('close', (code) => {
    console.log(`Server process exited with code ${code}`);
});

// Handle graceful shutdown
process.on('SIGINT', () => {
    console.log('\\nShutting down...');
    server.kill('SIGTERM');
    process.exit(0);
});

process.on('SIGTERM', () => {
    console.log('\\nReceived SIGTERM, shutting down...');
    server.kill('SIGTERM');
    process.exit(0);
});
