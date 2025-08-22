# Joblet Admin UI

React-based administrative interface for the Joblet job orchestration system.

## 🚀 Quick Start

```bash
# Install dependencies
npm install

# Start development server
npm run dev
```

## 📋 Available Scripts

```bash
# Development
npm run dev              # Start dev server
npm run build           # Production build
npm run preview         # Preview production build

# Code Quality
npm run lint            # Run ESLint (allows warnings)
npm run lint:strict     # Run ESLint (no warnings)
npm run lint:fix        # Auto-fix ESLint issues
npm run type-check      # TypeScript type checking

# Validation (using Make)
make check              # Quick validation (TypeScript + ESLint)
make build              # Production build
make clean              # Clean build artifacts
```

## 🚨 Error Detection Pipeline

The project catches errors at multiple stages:

| Stage     | When         | Time   | What It Checks        |
|-----------|--------------|--------|-----------------------|
| **Local** | `make check` | 5-10s  | TypeScript + ESLint   |
| **Build** | `make build` | 30s    | Production build      |
| **CI**    | PR/Push      | 2-5min | Full validation suite |

## 🏗️ Project Structure

```
src/
├── components/          # React components
├── hooks/              # Custom React hooks  
├── pages/              # Route components
├── services/           # API and utility services
└── types/              # TypeScript type definitions
```

## 🔧 Development Workflow

1. **Before coding**: Run `make check` to ensure clean start
2. **During development**: Fix TypeScript/ESLint issues as you code
3. **Before committing**: Run `make build` to ensure it builds
4. **CI handles the rest**: Automated validation on PR/push

## 🤝 Contributing

1. Run `make check` before committing
2. Ensure `make build` succeeds
3. Create PR - CI will run full validation

## 📦 Build & Deployment

```bash
npm run build    # Production build → dist/
make build       # Build with validation
```