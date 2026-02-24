<div align="center">
  <img src="../images/logo.png" alt="JamCapture Logo" width="100" height="100">
  <h1>JamCapture Documentation Structure</h1>
  <p><em>Rationalized and unified documentation system</em></p>
</div>

## 📁 Current Structure

```
├── README.md                    # Main project overview and quick start
├── CLAUDE.md                    # Master technical documentation (auto-maintained)
├── docs/
│   ├── web-server-guide.md     # Complete web server and API reference
│   └── DOCUMENTATION_STRUCTURE.md # This file - documentation guide
└── tests/
    └── README.md               # Testing documentation
```

## 📖 Documentation Hierarchy

### 1. **README.md** - Primary Entry Point
- **Purpose**: Project introduction and quick start for new users
- **Audience**: Developers, musicians, first-time users
- **Content**: Overview, installation, basic usage, web interface introduction
- **Focus**: Getting started quickly with both CLI and web interface

### 2. **docs/web-server-guide.md** - Web Interface Reference
- **Purpose**: Complete guide to web server functionality
- **Audience**: Users wanting smartphone control and advanced web features
- **Content**: Mobile interface, API reference, advanced features, troubleshooting
- **Focus**: Comprehensive web server documentation with emphasis on mobile usage

### 3. **CLAUDE.md** - Technical Master Documentation
- **Purpose**: Complete technical reference for development and maintenance
- **Audience**: Developers, contributors, system maintainers
- **Content**: Architecture, implementation details, testing, development guidelines
- **Focus**: Technical depth, development workflow, internal architecture

## 🔄 Changes Made

### Removed Redundant Files
- ❌ `LAUNCH.md` - Content merged into README.md
- ❌ `SESSION_INSTRUCTIONS.md` - Development-specific, not user-facing
- ❌ `docs/architecture.md` - Content integrated into CLAUDE.md

### Enhanced Documentation
- ✅ **README.md**: Streamlined with focus on quick start and web interface
- ✅ **Web Server Guide**: Comprehensive mobile-first documentation with new API details
- ✅ **CLAUDE.md**: Updated with latest web server features and architecture

### Key Improvements
1. **Mobile-First Approach**: Emphasis on smartphone control interface
2. **Unified Structure**: No duplication, clear hierarchy
3. **Web Server Focus**: Detailed coverage of the primary interface
4. **API Documentation**: Complete REST API reference with examples
5. **Technical Depth**: Maintained in CLAUDE.md for development needs

## 🎯 Usage Guidelines

### For End Users
1. Start with **README.md** for project overview
2. Use **Web Server Guide** for mobile interface and API integration
3. Refer to **CLAUDE.md** only for technical implementation details

### For Developers
1. **README.md** for quick onboarding
2. **CLAUDE.md** for architecture, development, and maintenance
3. **Web Server Guide** for API integration and web features

### For Contributors
- Update **CLAUDE.md** for technical changes
- Update **Web Server Guide** for new API endpoints or web features
- Keep **README.md** focused on essential quick-start information

## 📱 Web Server Emphasis

The documentation now reflects JamCapture's evolution toward **mobile-first recording control**:

- **Smartphone-optimized interface** for musicians
- **State-based recording workflow** (STANDBY → READY → RECORDING)
- **Real-time audio source validation**
- **Advanced file management and audio playback**
- **Complete REST API for custom integrations**

This structure supports both casual users who want smartphone control and developers who need comprehensive technical documentation.