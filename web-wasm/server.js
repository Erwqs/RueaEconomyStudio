const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const path = require('path');
const cors = require('cors');
const https = require('https');
const compression = require('compression');

const app = express();
const PORT = 8099;

// Enable compression for all responses
app.use(compression({
    filter: (req, res) => {
        // Always compress WASM files and other static assets
        if (req.url.endsWith('.wasm')) {
            return true;
        }
        // Use default compression filter for other files
        return compression.filter(req, res);
    },
    // Compression level (1-9, where 9 is best compression but slowest)
    level: 3,
    // Minimum size threshold for compression (bytes)
    threshold: 1024,
    // Enable brotli compression if client supports it
    brotli: {
        enabled: true,
        quality: 6 // Brotli quality (0-11, where 11 is best compression)
    }
}));

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

// Add middleware to handle conditional requests for better caching
app.use((req, res, next) => {
    // Log cache-related headers for debugging
    if (req.headers['if-modified-since'] || req.headers['if-none-match']) {
        console.log(`🔄 Cache validation request for: ${req.path}`);
        console.log(`   If-Modified-Since: ${req.headers['if-modified-since'] || 'none'}`);
        console.log(`   If-None-Match: ${req.headers['if-none-match'] || 'none'}`);
    }
    next();
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

// MIME type mapping with explicit WASM support and caching
app.use(express.static('.', {
    setHeaders: (res, path, stat) => {
        if (path.endsWith('.wasm')) {
            res.set('Content-Type', 'application/wasm');
            // Cache WASM files for 1 hour in production, allow revalidation
            res.set('Cache-Control', 'public, max-age=3600, must-revalidate');
            // Set ETag for cache validation based on file modification time and size
            const etag = `"${stat.mtime.getTime().toString(16)}-${stat.size.toString(16)}"`;
            res.set('ETag', etag);
            // Set Last-Modified header for conditional requests
            res.set('Last-Modified', stat.mtime.toUTCString());
        } else if (path.endsWith('.js') || path.endsWith('.css')) {
            // Cache JS/CSS files for 30 minutes
            res.set('Cache-Control', 'public, max-age=1800, must-revalidate');
            const etag = `"${stat.mtime.getTime().toString(16)}-${stat.size.toString(16)}"`;
            res.set('ETag', etag);
            res.set('Last-Modified', stat.mtime.toUTCString());
        } else if (path.match(/\.(png|jpg|jpeg|gif|svg|ico|woff|woff2|ttf|eot)$/)) {
            // Cache static assets for 24 hours
            res.set('Cache-Control', 'public, max-age=86400, immutable');
            const etag = `"${stat.mtime.getTime().toString(16)}-${stat.size.toString(16)}"`;
            res.set('ETag', etag);
            res.set('Last-Modified', stat.mtime.toUTCString());
        } else {
            // Default: cache HTML and other files for 5 minutes
            res.set('Cache-Control', 'public, max-age=300, must-revalidate');
            const etag = `"${stat.mtime.getTime().toString(16)}-${stat.size.toString(16)}"`;
            res.set('ETag', etag);
            res.set('Last-Modified', stat.mtime.toUTCString());
        }
        
        // Set CORS and security headers for all files
        res.set('Cross-Origin-Embedder-Policy', 'require-corp');
        res.set('Cross-Origin-Opener-Policy', 'same-origin');
    },
    // Enable conditional requests (If-Modified-Since, If-None-Match)
    lastModified: true,
    etag: true
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
    console.log('🗜️  Compression: Enabled (gzip/brotli)');
    console.log('💾 Caching strategy:');
    console.log('   - WASM files: 1 hour (3600s)');
    console.log('   - JS/CSS files: 30 minutes (1800s)');
    console.log('   - Static assets: 24 hours (86400s)');
    console.log('   - HTML/Other: 5 minutes (300s)');
    console.log('   - ETags enabled for cache validation');
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