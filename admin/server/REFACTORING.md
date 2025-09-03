# Server Refactoring Plan

## Current Problem

The `server.js` file has grown to **1,389 lines** and handles multiple concerns:

- Express app setup and middleware
- All API routes (nodes, jobs, workflows, system)
- WebSocket handling (logs, workflow status, monitoring)
- Utility functions
- Configuration

## Proposed Refactored Structure

```
src/
├── server.js                 # Main entry point (~50 lines)
├── config.js                 # Configuration constants
├── middleware/
│   └── index.js              # Express middleware setup
├── routes/
│   ├── index.js              # Route registration
│   ├── nodes.js              # Node management endpoints
│   ├── jobs.js               # Job management endpoints
│   ├── workflows.js          # Workflow management endpoints
│   └── system.js             # System info & resources endpoints
├── websocket/
│   ├── index.js              # WebSocket server setup
│   └── handlers.js           # WebSocket message handlers
└── utils/
    └── rnxExecutor.js        # Utility for executing rnx commands
```

## Benefits

### 📁 **Separation of Concerns**

- Each module has a single responsibility
- Routes are organized by domain (jobs, workflows, system)
- WebSocket logic is isolated from HTTP routes

### 🔍 **Improved Maintainability**

- Easy to find specific functionality
- Changes to one domain don't affect others
- New features can be added without touching existing files

### 🧪 **Better Testability**

- Individual modules can be unit tested
- Mock dependencies more easily
- Test specific concerns in isolation

### 👥 **Team Development**

- Multiple developers can work on different modules
- Reduced merge conflicts
- Clear ownership boundaries

## Migration Plan

### Phase 1: Create New Structure (✅ Done)

- Set up modular file structure
- Extract core utilities and config
- Create route modules with basic functionality

### Phase 2: Complete Route Extraction

- Move all remaining routes from server.js
- Add comprehensive error handling
- Migrate all business logic

### Phase 3: Enhanced WebSocket Handling

- Add connection management
- Implement reconnection logic
- Add WebSocket middleware

### Phase 4: Advanced Features

- Add request logging middleware
- Implement rate limiting
- Add health check endpoints
- API versioning support

## Usage

### Current (Monolithic)

```bash
npm start                    # Uses server.js (1,389 lines)
```

### Refactored (Modular)

```bash
npm run start:refactored     # Uses src/server.js (~50 lines)
```

## File Size Comparison

| File                 | Current     | Refactored |
|----------------------|-------------|------------|
| `server.js`          | 1,389 lines | ~50 lines  |
| **Total modules**    | 1 file      | 11 files   |
| **Average per file** | 1,389 lines | ~126 lines |

## Next Steps

1. **Test the refactored structure** with existing functionality
2. **Gradually migrate** remaining routes and WebSocket handlers
3. **Add comprehensive error handling** and logging
4. **Update deployment scripts** to use new structure
5. **Add unit tests** for individual modules

This refactoring maintains all existing functionality while making the codebase much more maintainable and scalable.