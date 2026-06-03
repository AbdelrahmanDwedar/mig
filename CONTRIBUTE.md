# Contributing to Mig

First off, thank you for considering contributing to Mig! It's people like you that make Mig a great tool for everyone.

To maintain high code quality and a clear project history, please follow these guidelines.

---

## 🛠 Development Setup

1. **Fork and Clone**: Fork the repository on GitHub and clone it locally.
2. **Go Version**: Ensure you are using Go **1.24** or higher.
3. **Dependencies**: Run `go mod download` to fetch the required packages.
4. **Build**: Verify you can build the project:
   ```bash
   go build ./cmd/mig
   ```

---

## 🌿 Branch Naming Convention

We use a prefix-based naming convention for branches. Always create a new branch for your work:

- `feat/{feature-name}`: For new features.
- `fix/{bug-name}`: For bug fixes.
- `docs/{change-name}`: For documentation updates.
- `refactor/{change-name}`: For code refactoring without behavior changes.
- `chore/{change-name}`: For maintenance tasks (dependencies, CI, etc.).

**Example**: `feat/cockroach-db-driver` or `fix/parser-newline-bug`.

---

## 🧪 Testing Requirements

- **New Tests**: Every feature or bug fix **must** include corresponding unit tests.
- **Isolation**: New tests should verify the change without affecting unrelated parts of the codebase.
- **Regression**: Do **not** modify existing tests unless the change is a breaking architectural update that has been discussed and approved.
- **Run Tests**: Before submitting, ensure all tests pass:
  ```bash
  go test ./...
  ```

---

## 📨 Pull Request Process

### 1. PR Naming
The title of your Pull Request must clearly state its purpose using these prefixes:

- `Feature: {Description}`
- `Bug Fix: {Description}`
- `Docs: {Description}`
- `Refactor: {Description}`
- `Chore: {Description}`

### 2. Linking Issues
If your PR addresses an existing issue, please include the issue number in the title and the description.

**Examples**:
- `Feature: adding CQL parser`
- `Feature: Adding CockroachDB driver (#2)`
- `Bug Fix: Resolving connection timeout on slow networks (#15)`

### 3. Keep it Surgical
Keep your PRs focused on a single task. Avoid "cleanup" of unrelated files in the same PR. If you see something that needs refactoring, create a separate `Refactor:` PR.

---

## 💎 Code Quality Standards

- **Formatting**: Run `go fmt ./...` before committing.
- **Linting**: We recommend running `go vet ./...` to catch common mistakes.
- **Clean Code**: Prioritize readability and simplicity. Avoid complex inheritance or hidden logic.

---

## 🤝 Questions?
If you have questions or need help, feel free to open a [Question/Support issue](https://github.com/AbdelrahmanDwedar/mig/issues/new?template=question.yml) using the provided template.

Happy coding! 🚀
