#!/bin/bash
# Claude Session Manager - Auto-reads documentation at start and updates at end

MASTER_DOC="CLAUDE_MASTER.md"
SESSION_LOG=".claude/session.log"
BACKUP_DIR=".claude/backups"

# Ensure directories exist
mkdir -p .claude/backups

# Session start function
session_start() {
    echo "📚 Reading JamCapture master documentation..."
    if [ -f "$MASTER_DOC" ]; then
        echo "✅ Found $MASTER_DOC - Context loaded"
        echo "$(date): Session started - Read $MASTER_DOC" >> "$SESSION_LOG"

        # Create backup
        cp "$MASTER_DOC" "$BACKUP_DIR/$(date +%Y%m%d_%H%M%S)_$MASTER_DOC"

        # Display key info
        echo ""
        echo "🎯 Quick Commands:"
        echo "  Test:  ./tests/e2e-test.sh"
        echo "  Build: go build"
        echo "  Run:   ./jamcapture --help"
        echo ""
    else
        echo "⚠️  Master documentation not found at $MASTER_DOC"
    fi
}

# Session end function
session_end() {
    echo "💾 Updating session documentation..."

    # Update timestamp in master doc
    if [ -f "$MASTER_DOC" ]; then
        # Update the last updated timestamp
        sed -i "s/Last updated: [0-9-]*/Last updated: $(date +%Y-%m-%d)/" "$MASTER_DOC"
        echo "$(date): Session ended - Updated $MASTER_DOC" >> "$SESSION_LOG"
        echo "✅ Documentation updated"
    fi

    # Show recent changes
    echo ""
    echo "📊 Recent Sessions:"
    tail -5 "$SESSION_LOG" 2>/dev/null || echo "No previous sessions"
}

# Auto-detect if this is session start or end
case "${1:-auto}" in
    start)
        session_start
        ;;
    end)
        session_end
        ;;
    auto)
        # Auto-detect based on context or default to start
        session_start
        ;;
    *)
        echo "Usage: $0 [start|end|auto]"
        echo "  start: Read documentation at session beginning"
        echo "  end:   Update documentation at session end"
        echo "  auto:  Auto-detect (default: start)"
        ;;
esac