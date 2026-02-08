# Code Cleanup Skill

This skill helps safely remove unused code with user confirmation.

## Workflow

1. **Analyze** the codebase to identify unused code:
   - Unused imports
   - Unreferenced functions/classes
   - Dead routes/endpoints
   - Orphaned database models

2. **List ALL items** you plan to delete/remove in a clear checklist format:
   ```markdown
   ## Items to Delete:
   - [ ] File: path/to/file.py - Reason: never imported
   - [ ] Function: `unused_function()` in module.py - Reason: no callers
   - [ ] Model: `OldModel` in models.py - Reason: no migrations reference it
   ```

3. **Present the list to the user** and ask:
   - "Are there any items in this list you want to KEEP?"
   - "Should I proceed with deleting these items?"

4. **Only after explicit approval**, proceed with deletion:
   - Delete files/functions/classes in the approved list
   - Remove related imports
   - Clean up migration dependencies if applicable

5. **After deletion, verify nothing broke**:
   - Run type checks: `cd sdk-py && uv run mypy` or `npx tsc --noEmit`
   - Run tests: `make test` or language-specific test command
   - Check for broken imports or references

6. **Commit ONLY the cleanup-related files**:
   - Use `git status` to review staged files
   - Exclude any unrelated changes
   - Use descriptive commit message: `refactor: remove unused [category]`

## Important Notes

- **Never assume something is unused** without verification
- **Always confirm before deleting** — user may have plans for that code
- **Check for indirect references** (reflection, dynamic imports, config files)
- **Preserve any code explicitly marked for keeping** by the user
