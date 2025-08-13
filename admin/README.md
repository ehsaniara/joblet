# Joblet Admin

This directory contains the complete admin interface for Joblet.

## Structure

```
admin/
├── server/          # Node.js admin server
│   ├── server.js    # Express server with WebSocket support
│   ├── package.json # Server dependencies
│   └── node_modules/
└── ui/              # React admin UI
    ├── src/         # React source code
    ├── dist/        # Built UI (generated)
    ├── package.json # UI dependencies
    └── node_modules/
```

## Architecture

```
Admin UI ↔ Admin Server ↔ rnx CLI ↔ joblet server
```

- **Admin UI**: React frontend with dark mode support
- **Admin Server**: Node.js server that calls rnx CLI commands
- **rnx CLI**: Command-line tool for gRPC communication with joblet
- **joblet server**: Main job execution server

## Development

### Start Admin Server
```bash
cd admin/server
npm start
# or for development with auto-reload:
npm run dev
```

### Build Admin UI
```bash
make admin-ui
```

### Build Everything
```bash
make admin-server
```

## Access

- Admin UI: http://localhost:5173
- API endpoints: /api/*
- WebSocket endpoints: /ws/*

## Environment Variables

- `PORT`: Server port (default: 5173)
- `BIND_ADDRESS`: Bind address (default: localhost)
- `RNX_PATH`: Path to rnx binary (default: ../../bin/rnx)