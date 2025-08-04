const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const rateLimit = require('express-rate-limit');
const fs = require('fs');
const path = require('path');

const app = express();
const PORT = process.env.PORT || 3001;
const NODE_ENV = process.env.NODE_ENV || 'development';

// Middleware
app.use(helmet());
app.use(cors());
app.use(express.json({ limit: '10mb' }));

// Rate limiting
const limiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 100 // limit each IP to 100 requests per windowMs
});
app.use(limiter);

// Health check
app.get('/health', (req, res) => {
  res.json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    uptime: process.uptime(),
    environment: NODE_ENV
  });
});

// Mock authentication endpoint
app.post('/auth/login', (req, res) => {
  const { username, password } = req.body;
  
  if (username === 'testuser' && password === 'testpass') {
    res.json({
      token: 'mock-jwt-token-' + Date.now(),
      user: { username, role: 'user' }
    });
  } else if (username === 'admin' && password === 'admin123') {
    res.json({
      token: 'mock-admin-token-' + Date.now(),
      user: { username, role: 'admin' }
    });
  } else {
    res.status(401).json({ error: 'Invalid credentials' });
  }
});

// Mock users endpoint
app.get('/users', (req, res) => {
  const mockUsers = [
    { id: 1, username: 'john', email: 'john@example.com', role: 'user' },
    { id: 2, username: 'jane', email: 'jane@example.com', role: 'admin' },
    { id: 3, username: 'bob', email: 'bob@example.com', role: 'user' }
  ];
  
  res.json(mockUsers);
});

// Mock products endpoint
app.get('/products', (req, res) => {
  const mockProducts = [
    { id: 1, name: 'Laptop', price: 999.99, category: 'Electronics' },
    { id: 2, name: 'Mouse', price: 29.99, category: 'Electronics' },
    { id: 3, name: 'Keyboard', price: 79.99, category: 'Electronics' }
  ];
  
  res.json(mockProducts);
});

// Create user endpoint
app.post('/users', (req, res) => {
  const { username, email, role = 'user' } = req.body;
  
  // Basic validation
  if (!username || !email) {
    return res.status(400).json({ error: 'Username and email required' });
  }
  
  const newUser = {
    id: Date.now(),
    username,
    email,
    role,
    createdAt: new Date().toISOString()
  };
  
  res.status(201).json(newUser);
});

// Error handling middleware
app.use((error, req, res, next) => {
  console.error('Error:', error);
  
  // Log error to persistent volume if available
  if (fs.existsSync('/volumes/nodejs-projects')) {
    const errorLog = {
      timestamp: new Date().toISOString(),
      error: error.message,
      stack: error.stack,
      url: req.url,
      method: req.method,
      ip: req.ip
    };
    
    const logPath = '/volumes/nodejs-projects/logs';
    fs.mkdirSync(logPath, { recursive: true });
    fs.appendFileSync(
      path.join(logPath, 'error.log'),
      JSON.stringify(errorLog) + '\n'
    );
  }
  
  res.status(500).json({
    error: 'Internal server error',
    timestamp: new Date().toISOString()
  });
});

// Start server
const server = app.listen(PORT, () => {
  console.log(`Demo service running on port ${PORT} in ${NODE_ENV} mode`);
  console.log(`Health check: http://localhost:${PORT}/health`);
});

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('SIGTERM received, shutting down gracefully');
  server.close(() => {
    console.log('Process terminated');
  });
});

module.exports = app;