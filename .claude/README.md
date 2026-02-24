# Claude Session Management

## Auto-Documentation System

Ce répertoire contient les outils pour maintenir automatiquement la documentation Claude.

### Fichiers

- `session_manager.sh` - Script de gestion des sessions
- `session.log` - Log des sessions Claude
- `backups/` - Sauvegardes automatiques de CLAUDE_MASTER.md

### Usage

```bash
# Au début d'une session
./.claude/session_manager.sh start

# À la fin d'une session
./.claude/session_manager.sh end

# Auto-détection (par défaut: start)
./.claude/session_manager.sh
```

### Integration

Le script doit être exécuté:
1. **Au début**: Lecture de CLAUDE_MASTER.md pour charger le contexte
2. **À la fin**: Mise à jour des timestamps et sauvegarde

### Master Documentation

Le fichier principal `CLAUDE_MASTER.md` contient:
- Architecture complète du projet
- Instructions de développement
- Historique des changements
- Commands de test et build
- État actuel du projet

Il remplace tous les anciens fichiers:
- ~~claude.md~~ → Fusionné dans CLAUDE_MASTER.md
- ~~CLAUDE.md~~ → Fusionné dans CLAUDE_MASTER.md
- ~~architecture.md~~ → Fusionné dans CLAUDE_MASTER.md
- ~~progress.md~~ → Fusionné dans CLAUDE_MASTER.md

### Maintenance

Le système maintient automatiquement:
- Timestamps de dernière mise à jour
- Sauvegardes horodatées
- Log des sessions Claude
- Structure unifiée et cohérente