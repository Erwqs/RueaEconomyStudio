const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const path = require('path');
const cors = require('cors');
const https = require('https');

const app = express();
const PORT = 8099;

// Enable CORS for all routes
app.use(cors({
    origin: ['https://ruea.sequoia.ooo', 'http://localhost:8099', 'https://localhost:8099'],
    methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
    allowedHeaders: ['Content-Type', 'Authorization', 'X-Requested-With'],
    credentials: true
}));

// Additional CORS headers as middleware
app.use((req, res, next) => {
    res.header('Access-Control-Allow-Origin', '*');
    res.header('Access-Control-Allow-Methods', 'GET,PUT,POST,DELETE,OPTIONS');
    res.header('Access-Control-Allow-Headers', 'Content-Type, Authorization, Content-Length, X-Requested-With');
    
    // Handle preflight requests
    if (req.method === 'OPTIONS') {
        res.sendStatus(200);
    } else {
        next();
    }
});

// Set up proxy routes BEFORE static file serving to ensure they take precedence

// Guild list endpoint - fetches from Athena API
app.get('/guild_list', async (req, res) => {
    const athenaUrl = 'https://athena.wynntils.com/cache/get/guildList';
    console.log(`🌍 Fetching guild list from Athena API: ${athenaUrl}`);
    
    try {
        // Set CORS headers first
        res.setHeader('Access-Control-Allow-Origin', '*');
        res.setHeader('Access-Control-Allow-Methods', 'GET,PUT,POST,DELETE,OPTIONS');
        res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization, Content-Length, X-Requested-With');
        
        const response = await fetch(athenaUrl, {
            method: 'GET',
            headers: {
                'User-Agent': 'RueaEconomyStudio/1.0',
                'Accept': 'application/json',
                'Accept-Encoding': 'gzip, deflate, br'
            },
            redirect: 'follow' // Let fetch handle redirects automatically
        });
        
        console.log(`📡 Athena guild list response: ${response.status}`);
        console.log(`📡 Final URL after redirects: ${response.url}`);
        
        // Get the response data
        const data = await response.text();
        
        // Set content type based on response
        const contentType = response.headers.get('content-type') || 'application/json';
        res.setHeader('Content-Type', contentType);
        
        // Return the data with proper status
        res.status(response.status).send(data);
        
    } catch (error) {
        console.error(`❌ Error fetching guild list from Athena API:`, error.message);
        res.status(502).json({ 
            error: 'Athena API fetch error', 
            message: error.message,
            url: athenaUrl
        });
    }
});

// Territory list endpoint - fetches from Wynncraft API
app.get('/territory_list', async (req, res) => {
    const wynncraftUrl = 'https://api.wynncraft.com/v3/guild/list/territory';
    console.log(`🌍 Fetching territory list from Wynncraft API: ${wynncraftUrl}`);
    
    try {
        // Set CORS headers first
        res.setHeader('Access-Control-Allow-Origin', '*');
        res.setHeader('Access-Control-Allow-Methods', 'GET,PUT,POST,DELETE,OPTIONS');
        res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization, Content-Length, X-Requested-With');
        
        const response = await fetch(wynncraftUrl, {
            method: 'GET',
            headers: {
                'User-Agent': 'RueaEconomyStudio/1.0',
                'Accept': 'application/json',
                'Accept-Encoding': 'gzip, deflate, br'
            },
            redirect: 'follow'
        });
        
        console.log(`📡 Wynncraft territory list response: ${response.status}`);
        
        // Get the response data
        const data = await response.text();
        
        // Set content type based on response
        const contentType = response.headers.get('content-type') || 'application/json';
        res.setHeader('Content-Type', contentType);
        
        // Return the data with proper status
        res.status(response.status).send(data);
        
    } catch (error) {
        console.error(`❌ Error fetching territory list from Wynncraft API:`, error.message);
        res.status(502).json({ 
            error: 'Wynncraft API fetch error', 
            message: error.message,
            url: wynncraftUrl
        });
    }
});

// Legacy API proxies (for backward compatibility)
// Set up CORS proxy for Wynncraft API
app.use('/api/wynncraft', createProxyMiddleware({
    target: 'https://api.wynncraft.com',
    changeOrigin: true,
    pathRewrite: {
        '^/api/wynncraft': '', // remove /api/wynncraft from the beginning of the path
    },
    onProxyReq: (proxyReq, req, res) => {
        console.log(`Proxying request to: https://api.wynncraft.com${req.url.replace('/api/wynncraft', '')}`);
    },
    // Add CORS headers to proxy responses
    onProxyRes: (proxyRes, req, res) => {
        proxyRes.headers['Access-Control-Allow-Origin'] = '*';
        proxyRes.headers['Access-Control-Allow-Methods'] = 'GET,PUT,POST,DELETE,OPTIONS';
        proxyRes.headers['Access-Control-Allow-Headers'] = 'Content-Type, Authorization, Content-Length, X-Requested-With';
    },
}));

// Set up CORS proxy for Athena API
app.use('/api/athena', createProxyMiddleware({
    target: 'https://athena.wynntils.com',
    changeOrigin: true,
    pathRewrite: {
        '^/api/athena': '', // remove /api/athena from the beginning of the path
    },
    onProxyReq: (proxyReq, req, res) => {
        console.log(`Proxying request to: https://athena.wynntils.com${req.url.replace('/api/athena', '')}`);
    },
    // Add CORS headers to proxy responses
    onProxyRes: (proxyRes, req, res) => {
        proxyRes.headers['Access-Control-Allow-Origin'] = '*';
        proxyRes.headers['Access-Control-Allow-Methods'] = 'GET,PUT,POST,DELETE,OPTIONS';
        proxyRes.headers['Access-Control-Allow-Headers'] = 'Content-Type, Authorization, Content-Length, X-Requested-With';
    },
}));

// MIME type mapping with explicit WASM support
app.use(express.static('.', {
    setHeaders: (res, path, stat) => {
        if (path.endsWith('.wasm')) {
            res.set('Content-Type', 'application/wasm');
            // Stronger cache-busting for WASM files during development
            res.set('Cache-Control', 'no-cache, no-store, must-revalidate, max-age=0');
            res.set('Pragma', 'no-cache');
            res.set('Expires', '0');
            res.set('Last-Modified', new Date().toUTCString());
        }
        // Set CORS and security headers
        res.set('Cross-Origin-Embedder-Policy', 'require-corp');
        res.set('Cross-Origin-Opener-Policy', 'same-origin');
        // Default cache control for other files
        if (!path.endsWith('.wasm')) {
            res.set('Cache-Control', 'no-cache, no-store, must-revalidate');
            res.set('Pragma', 'no-cache');
            res.set('Expires', '0');
        }
    }
}));

// Generic API proxy for other API-like paths (fallback)
app.use((req, res, next) => {
    // Skip if it's a static file (has file extension) or known API routes
    const hasExtension = path.extname(req.path) !== '';
    const isKnownApiRoute = req.path.startsWith('/api/') || req.path.startsWith('/v3') || req.path.startsWith('/cache');
    
    if (hasExtension || isKnownApiRoute) {
        return next();
    }
    
    // If the path looks like an API call and starts with known patterns, proxy it
    const isApiLikeCall = req.path.match(/^\/(v[0-9]+\/|[a-z]+\/[a-z]+\/)/);
    
    if (isApiLikeCall) {
        console.log(`🌍 Generic API proxy: ${req.path} -> https://api.wynncraft.com${req.path}`);
        
        return createProxyMiddleware({
            target: 'https://api.wynncraft.com',
            changeOrigin: true,
            onError: (err, req, res) => {
                console.error(`Generic proxy error for ${req.path}:`, err.message);
                res.status(502).json({ error: 'Generic API proxy error', message: err.message });
            }
        })(req, res, next);
    }
    
    next();
});

app.listen(PORT, () => {
    console.log('='.repeat(70));
    console.log('🚀 Ruea Economy Studio - WebAssembly Development Server');
    console.log('='.repeat(70));
    console.log(`📡 Server running at: http://localhost:${PORT}`);
    console.log(`📁 Serving files from: ${__dirname}`);
    console.log(`🌐 CORS proxy available at:`);
    console.log(`   - /api/wynncraft/* -> https://api.wynncraft.com/*`);
    console.log(`   - /api/athena/* -> https://athena.wynntils.com/*`);
    console.log(`🌍 Direct domain proxies:`);
    console.log(`   - /v3/* -> https://api.wynncraft.com/v3/*`);
    console.log(`   - /cache/* -> https://athena.wynntils.com/cache/*`);
    console.log('');
    
    console.log('🎮 Open http://localhost:8099 in your browser');
    console.log('🔍 Watch this console for request logs');
    console.log('⚠️  Note: Domain proxies handle API calls when deployed');
    console.log('⏹️  Press Ctrl+C to stop the server');
    console.log('');
});

// Graceful shutdown
process.on('SIGINT', () => {
    console.log('\n\n⏹️  Server shutting down gracefully...');
    process.exit(0);
});

// Handle uncaught errors
process.on('uncaughtException', (err) => {
    console.error('❌ Uncaught Exception:', err);
    process.exit(1);
});

process.on('unhandledRejection', (reason, promise) => {
    console.error('❌ Unhandled Rejection at:', promise, 'reason:', reason);
    process.exit(1);
});