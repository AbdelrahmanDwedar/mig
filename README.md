# Mig

![Mig Logo](docs/Logo.png)

Mig – Your simple, language-agnostic, migration management tool!

## Features
- **Language Agnostic:** Currently supports SQL, with plans for YAML, JSON, and more.
- **Multi-Driver Support:** Built-in support for **PostgreSQL**, **MySQL**, and **SQLite**.
- **Easy Workflow:** Simple `setup`, `create`, `migrate`, `rollback`, `reset`, `fresh`, `refresh`, and `status` commands.
- **Configurable:** Driven by `mig.yml` and environment variables.

## Getting Started

1. **Initialize your project:**
   ```bash
   mig setup
   ```
   Follow the interactive prompts or use flags (`--driver`, `--dbname`, `--dir`).

2. **Create a new migration:**
   ```bash
   mig create create_users_table
   ```
   Generates `YYYY_MM_DD_HHMMSS_create_users_table.sql`.

3. **Run migrations:**
   ```bash
   mig migrate
   ```

4. **Rollback migrations:**
   ```bash
   # Rollback 1 step (default)
   mig rollback
   
   # Rollback 2 steps
   mig rollback --steps=2
   
   # Rollback specific migration
   mig rollback --migration=create_users_table
   ```

5. **Reset and Refresh:**
   ```bash
   mig reset      # Rollback all
   mig fresh      # Reset and migrate
   mig refresh    # Alias for fresh
   ```

6. **Status:**
   ```bash
   mig status
   ```

## Configuration
Configure your database in `mig.yml` or use environment variables like `MIG_DB_DRIVER`.
