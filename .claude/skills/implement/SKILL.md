# Implementation Skill

This skill enforces action-over-planning mode for feature implementation.

## Rules

1. **No planning documents** — go straight to implementation
2. **Start coding within the first 2-3 messages**
3. **If analysis is needed, keep it under 10 lines** then start coding
4. **Work incrementally** — implement, test, commit, repeat

## Workflow

1. **Quick read** (max 5 files or 2 minutes):
   - Understand the relevant code structure
   - Identify integration points

2. **Implement immediately**:
   - Write the feature code
   - Add necessary imports/dependencies
   - Update configuration if needed

3. **Write tests as you go**:
   - Unit tests for new functions
   - Integration tests for new endpoints/features
   - Run tests after each module: `make test-sdk` or `make test-server`

4. **Fix failures in tight loops**:
   - If tests fail, read the error output carefully
   - Fix the root cause immediately
   - Re-run tests until they pass
   - Do NOT move to the next module until current tests pass

5. **Verify quality**:
   - Run linters: `make lint-sdk` or `make lint-server`
   - Fix any linting issues
   - Run type checks if applicable

6. **Commit atomically**:
   - Single commit per feature/fix
   - Descriptive message following conventional commits
   - Only include files relevant to this feature

## When to Stop Planning and Start Coding

- ❌ Writing analysis documents
- ❌ Creating architecture diagrams
- ❌ Discussing multiple alternative approaches
- ❌ Planning more than 1 phase ahead

- ✅ Reading 2-5 files to understand context
- ✅ Listing 3-5 bullet points of what to implement
- ✅ Writing actual code within the first response

## Example Good Workflow

```
User: "Add cost tracking to LLM spans"

Claude:
1. Quick read: _llm.py, _span.py, types.ts (done)
2. Implementation plan (brief):
   - Add cost_usd parameter to llm_span()
   - Store in SpanData.attributes["cost.usd"]
   - Update TypeScript types for dashboard
3. [STARTS CODING IMMEDIATELY]
   - Edit _llm.py: add cost_usd param...
   - Edit _span.py: propagate to attributes...
   - Run tests...
```

## Example Bad Workflow (DON'T DO THIS)

```
User: "Add cost tracking to LLM spans"

Claude: [Writes 50-line analysis document]
Let me analyze the current cost tracking architecture...
[10 paragraphs of analysis]
Now let me create a comprehensive plan...
[Another planning document]

❌ No code written yet!
```
