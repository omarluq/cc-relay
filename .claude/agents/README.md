# Agent Configurations for cc-relay

Specialized agents for specific tasks in cc-relay development.

## Available Agents

### go-code-fixer
**Purpose**: Fix Go code quality issues

**When to use**:
- Lint errors that need manual fixes
- Test failures in Go code
- Code refactoring needed

**What it does**:
1. Reads lint/test errors
2. Understands the issue
3. Fixes the actual code (not config)
4. Verifies fix works

**Skills**:
- fix-lint-errors
- fix-test-failures
- auto-fix-code

---

### test-debugger
**Purpose**: Debug and fix failing tests

**When to use**:
- Tests failing
- Race conditions detected
- Coverage gaps

**What it does**:
1. Runs failing test with verbose output
2. Identifies root cause
3. Fixes code or test logic
4. Verifies all tests pass

**Skills**:
- testing
- fix-test-failures

---

### code-formatter
**Purpose**: Auto-format all code

**When to use**:
- Before committing
- After major changes
- Pre-commit hook failures

**What it does**:
1. Runs all formatters (gofmt, goimports, gofumpt, yamlfmt, markdownlint)
2. Auto-fixes format issues
3. Stages fixed files

**Skills**:
- fix-format-issues
- auto-fix-code

---

### security-auditor
**Purpose**: Fix security issues

**When to use**:
- Security scan failures
- gosec errors
- govulncheck warnings

**What it does**:
1. Runs security scanners
2. Identifies vulnerabilities
3. Fixes security issues in code
4. Verifies fixes

**Skills**:
- fix-lint-errors (gosec)
- security scanning

---

### proto-generator
**Purpose**: Generate and lint proto files

**When to use**:
- Proto file changes
- gRPC code generation needed
- Proto lint errors

**What it does**:
1. Formats proto files
2. Lints proto files
3. Generates Go code
4. Fixes lint issues

**Skills**:
- Proto tools (buf)

---

## Agent Usage

### In Claude Code

```
Use the go-code-fixer agent to fix lint errors
```

Claude will:
1. Activate the agent
2. Use appropriate skills
3. Fix the code issues
4. Verify fixes work

### Agent Workflow

```
1. User request → Select agent
2. Agent activates → Loads skills
3. Agent analyzes → Understands issue
4. Agent fixes → Modifies code (not config)
5. Agent verifies → Runs tests/linters
6. Agent reports → Shows what was fixed
```

## Agent Guidelines

All agents follow these principles:

✅ **DO**:
- Fix actual code issues
- Use auto-fix tools first
- Verify fixes work
- Explain what was fixed

❌ **DON'T**:
- Modify configuration files
- Disable linters/tests
- Skip verification
- Make changes without understanding

## Creating Custom Agents

To create a custom agent:

1. Define agent purpose
2. List required skills
3. Document when to use
4. Add to this README

Example:
```markdown
### my-agent
**Purpose**: Specific task description

**When to use**:
- Condition 1
- Condition 2

**What it does**:
1. Step 1
2. Step 2

**Skills**:
- skill-1
- skill-2
```
