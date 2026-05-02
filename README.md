# Mig

![Mig Logo](docs/Logo.png)

> Your simple, language-agnostic, migration management tool!

[![Build Status](https://img.shields.io/github/actions/workflow/status/mig-tool/mig/build.yml?branch=main&label=Build&style=flat-square)](https://github.com/mig-tool/mig/actions)
[![Test Status](https://img.shields.io/github/actions/workflow/status/mig-tool/mig/tests.yml?branch=main&label=Tests&style=flat-square)](https://github.com/mig-tool/mig/actions)
![License](https://img.shields.io/github/license/mig-tool/mig?style=flat-square)
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
- [Getting Started](#-getting-started)
- [Usage](#-usage)
- [Configuration](#-configuration)
- [Architecture](#-architecture)

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

## ⚙️ Configuration
Configure your database in `mig.yml`. You can use environment variables directly using `${VAR_NAME}` syntax:

```yaml
database:
  driver: ${DB_DRIVER}
  dbname: ${DB_NAME}
migrations:
  parser: sql
  dir: migrations
```

---

## 🏗 Architecture
Mig utilizes a modular architecture based on **Drivers** (DB connection) and **Parsers** (file format handling). Check out [ARCHITECTURE.md](ARCHITECTURE.md) for a deep dive and visual diagrams.

---

Made with ❤️ by the Mig team.
