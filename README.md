# Mig

![Mig Logo](docs/Logo.png)

> Your simple, language-agnostic, migration management tool!

[![Build Status](https://img.shields.io/github/actions/workflow/status/AbdelrahmanDwedar/mig/build.yml?branch=main&label=Build&style=flat-square)](https://github.com/AbdelrahmanDwedar/mig/actions)
[![Test Status](https://img.shields.io/github/actions/workflow/status/AbdelrahmanDwedar/mig/tests.yml?branch=main&label=Tests&style=flat-square)](https://github.com/AbdelrahmanDwedar/mig/actions)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](https://github.com/AbdelrahmanDwedar/mig/blob/main/LICENSE)
![Language](https://img.shields.io/badge/language-Go-blue?style=flat-square)

---

## 🚀 Why Mig?
Stop wrestling with complex migration tools. **Mig** gives you a streamlined, driver-based approach to managing your database schema, no matter the language you use. 

### ✨ Key Features
- **Language Agnostic:** Currently supports SQL with a robust, directive-based parser (`+migrate Up`/`Down`).
- **Driver-First:** First-class support for **PostgreSQL**, **MySQL**, and **SQLite**.
- **Dev-Friendly:** Interactive `setup` with sensible defaults.
- **Advanced Control:** Selective rollback (`--steps`), specific file targeting (`--migration`), and safe `fresh`/`refresh` cycles.

---

## 📋 Table of Contents
- [Installation](#-installation)
- [Getting Started](#-getting-started)
- [Usage](#-usage)
- [Configuration](#-configuration)
- [AI Agent Support](#-ai-agent-support)
- [Architecture](#-architecture)

---

## 📦 Installation

### Universal Installer (Linux)
The easiest way to install Mig is via our universal installation script:

```bash
curl -sSL https://raw.githubusercontent.com/AbdelrahmanDwedar/mig/main/scripts/install.sh | bash
```

### Package Managers
If you prefer using your system's package manager, you can install Mig via **apt** or **dnf** after setting up our repository:

#### Debian / Ubuntu
```bash
curl -1sLf 'https://dl.cloudsmith.io/public/AbdelrahmanDwedar/stable/setup.deb.sh' | sudo -E bash
sudo apt install mig
```

#### Fedora / CentOS / RHEL
```bash
curl -1sLf 'https://dl.cloudsmith.io/public/AbdelrahmanDwedar/stable/setup.rpm.sh' | sudo -E bash
sudo dnf install mig
```

### Manual Installation
You can also download the pre-compiled binaries directly from our [Releases](https://github.com/AbdelrahmanDwedar/mig/releases) page.

---

## 🏁 Getting Started

### Quick Start
```bash
# 1. Setup your project (interactive)
mig setup

# 2. Create your first migration
mig create add_users_table

# 3. Apply changes!
mig migrate
```

---

## 🛠 Usage

| Command | Description |
| :--- | :--- |
| `setup` | Initialize the project (or run for config-check) |
| `create` | Generate a new timestamped migration |
| `migrate` | Run all pending migrations |
| `rollback` | Reverse migrations (--steps, --migration) |
| `reset` | Rollback *all* applied migrations |
| `fresh` | Reset the DB and re-run all migrations |
| `status` | View applied/pending migration list |

---

Every command accepts a global `--json` flag, printing a single `{"success", "data", "error"}` line to stdout instead of prose — useful for scripts and AI agents driving `mig` programmatically.

---

## 🤖 AI Agent Support
Mig ships with an agent-facing skill doc: **[docs/AGENT_SKILL.md](docs/AGENT_SKILL.md)**. It documents `--json` output shapes, non-interactive `setup` flags, parser quirks, and other gotchas an agent needs to drive `mig` correctly without trial and error.

To use it with **Claude Code**, copy it into a skill directory so Claude can discover and load it automatically:

```bash
mkdir -p ~/.claude/skills/mig
cp docs/AGENT_SKILL.md ~/.claude/skills/mig/SKILL.md
```

(Use `.claude/skills/mig/SKILL.md` inside a specific project instead of `~/.claude/skills` to scope it to that project.)

For other agents (Cursor, Copilot, etc.), just point them at `docs/AGENT_SKILL.md` or paste its contents into your agent's context/rules file — it's plain Markdown with no Claude-specific dependencies.

## ⚙️ Configuration
Configure your database in `mig.yml`. Mig supports advanced Docker Compose-style environment variable interpolation:

- **Basic**: `${VAR_NAME}`
- **Default values**: `${VAR_NAME:-default_value}` (uses `default_value` if `VAR_NAME` is unset or empty)
- **Mandatory variables**: `${VAR_NAME:?error_message}` (exits with `error_message` if `VAR_NAME` is unset or empty)

Example `mig.yml`:
```yaml
database:
  driver: ${DB_DRIVER:-mysql}
  host: ${DB_HOST:-localhost}
  port: ${DB_PORT:-3306}
  user: ${DB_USER:-root}
  password: ${DB_PASSWORD:?database password is required}
  dbname: ${DB_NAME:-mydatabase}
migrations:
  parser: sql
  dir: migrations
```

---

## 🏗 Architecture
Mig utilizes a modular architecture based on **Drivers** (DB connection) and **Parsers** (file format handling). Check out [ARCHITECTURE.md](ARCHITECTURE.md) for a deep dive and visual diagrams.

---

Made with ❤️ by the Mig team.
