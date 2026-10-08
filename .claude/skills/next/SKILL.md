---
name: next
description: Pick up the highest-priority feature from notes/Next Steps.md and start building it. Use on startup or when asked "what's next" / "work on the next thing". Optional argument names a different Priority item (e.g. "2" or "Graphics Base").
---

# Work on the next priority

The plan lives in `notes/` (an Obsidian vault; see `notes/README.md` for the card schema). `notes/Next Steps.md` has a `## Priority` section: a numbered list in the order to work on. Big items link to a plan note in `notes/next/` with its own numbered `## Steps`.

## 1. Find the work

1. Read `notes/Next Steps.md`. Take the **first item under `## Priority`** (lowest number, `0.` counts). If an argument was given, take that item instead.
2. If the item has numbered sub-items, take the first one that doesn't start with "Done".
3. Follow its `[[wikilinks]]`: find the plan note (`notes/next/<Name>.md`) and pick the first step under `## Steps` not marked "Done". Priority lines often point straight at a step ("[[Terrain Layers]] step 4") — use that.
4. Read what the step touches: the plan note's "How it works" and "Open questions", the system cards it links (`notes/systems/`, `notes/tooling/`, `notes/engine/`, ...), and the matching spec in `notes/specs/` if one exists. Cards say how things work today; plan notes say what to build.
5. Check `git log --oneline -10` and `git status` — the step may be partly built, or there may be uncommitted work to pick up.

If the item is blocked (a *Needs* that doesn't exist yet), is a vague "check X" with nothing to build, or an open question decides the design, stop and ask the user, offering the next item as the alternative. Otherwise don't ask — state the pick and go.

## 2. Say what you're doing

Before writing code, give a short brief: the item and step, what "done" looks like (quote the plan's acceptance details), the files you expect to change, and any open question you're resolving with a default (and which default).

## 3. Build it

- Follow the plan note exactly; where it's silent, make the call a player would expect and note it for the log.
- Pre-release: breaking changes are fine. Change the save format, drop old fields, no compatibility shims.
- If the item is a bug or something puzzling, diagnose first: find and report the root cause before fixing.
- Verify with `go build ./...` and `go vet ./...`. Don't add `*_test.go` files. Game behavior is judged by the user in the running build, so finish by telling them exactly what to try (scenario, tool, what they should see).

## 4. Update the notes

When the step is built, update the notes the way the repo does it (see `notes/README.md`):

- Plan note: mark the step "Done: ..." with a one-line summary, move settled facts into "How it works", resolve any answered open questions, and add a dated line under `## Log` (today's date, `YYYY-MM-DD`). Set `status` if it changed.
- `notes/Next Steps.md`: shrink or delete the Priority line, renumber, and update any parent item that summarizes progress.
- System cards and specs whose behavior changed: update the description and add a `## Log` line.

## 5. Hand off

Summarize what changed and what to try in-game. Don't commit until the user has tried it and says so. Then commit in the repo's style: the code commit first (e.g. `Terrain layers: strength sliders on every row`), then a separate `Notes: ...` commit for the notes.
