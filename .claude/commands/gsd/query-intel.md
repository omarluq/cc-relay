---
name: gsd:query-intel
description: Query codebase intelligence graph for dependencies and hotspots
argument-hint: "<dependents|hotspots> [file-path]"
allowed-tools:
  - Bash
---

<objective>
Query the codebase intelligence graph database for relationship information.

**Query types:**
- `dependents <file>` — What files depend on this file? (blast radius)
- `hotspots` — Which files have the most dependents? (change carefully)

Output: Formatted query results from graph.db
</objective>

<context>
This command exposes the graph query capabilities built by Phase 4 (Semantic Intelligence).

**Use for:**
- Checking blast radius before refactoring a core file
- Identifying high-impact files that need careful changes
- Understanding dependency relationships in the codebase

**Requires:** `.planning/intel/graph.db` (created by `/gsd:analyze-codebase` with entity generation)

If graph.db doesn't exist, the command will return an error suggesting to run analyze-codebase first.
</context>

<process>

## Step 1: Parse arguments

Extract query type and optional file path from arguments.

**Arguments:** $ARGUMENTS

**Expected formats:**
- `dependents src/lib/db.ts` — query what depends on this file
- `hotspots` — query most-depended-on files
- `hotspots 10` — query top 10 hotspots (default: 5)

## Step 2: Convert file path to entity ID

For `dependents` queries, convert the file path to entity ID format:
- `src/lib/db.ts` → `src-lib-db`
- Replace `/` with `-`, remove extension

```bash
# Example conversion
FILE_PATH="src/lib/db.ts"
ENTITY_ID=$(echo "$FILE_PATH" | sed 's/\.[^.]*$//' | tr '/' '-')
```

## Step 3: Execute query

Run the appropriate query against the graph database:

**For dependents:**
```bash
echo '{"action":"query","type":"dependents","target":"'$ENTITY_ID'","limit":20}' | node hooks/gsd-intel-index.js
```

**For hotspots:**
```bash
echo '{"action":"query","type":"hotspots","limit":5}' | node hooks/gsd-intel-index.js
```

## Step 4: Format and present results

Parse the JSON response and present in readable format.

**For dependents:**
```
## Files that depend on {file-path}

Found {count} dependents:

1. src/api/users.ts
2. src/api/auth.ts
3. src/services/payment.ts
...

**Blast radius:** {count} files would be affected by changes.
```

**For hotspots:**
```
## Dependency Hotspots

These files have the most dependents — change carefully:

| Rank | File | Dependents |
|------|------|------------|
| 1 | src/lib/db.ts | 42 |
| 2 | src/types/user.ts | 35 |
| 3 | src/utils/format.ts | 28 |
```

## Step 5: Handle errors

**If graph.db doesn't exist:**
```
No graph database found at .planning/intel/graph.db

Run /gsd:analyze-codebase first to build the dependency graph.
```

**If entity not found:**
```
No entity found for: {file-path}

The file may not be indexed yet. Try:
- /gsd:analyze-codebase to rebuild the index
- Check the file path is correct
```

</process>

<success_criteria>
- [ ] Query type parsed from arguments
- [ ] File path converted to entity ID (for dependents)
- [ ] Query executed against graph.db
- [ ] Results formatted in readable markdown
- [ ] Errors handled gracefully with helpful messages
</success_criteria>
