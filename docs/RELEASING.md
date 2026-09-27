# Publishing changes

Use a clone of the public repository and branch from `main`. The first public
commit is a sanitized snapshot; private development history is intentionally
not its parent.

The original development checkout retains a local `master` branch and private
remote for recovery. Do not merge that branch into public `main`, push all
branches, or mirror that repository. Its history was not cleared for release.
Port an intended change as a reviewed patch if it exists only in private history.

Before a push:

1. Stage only the intended files and inspect the staged diff.
2. Run the privacy audit and appropriate tests in `TESTING.md`.
3. Confirm commit author and committer use the public account identity and
   GitHub noreply address. Do not add attribution trailers.
4. Update the task board and verification report with actual results. Distinguish
   browser liveness, synthetic integration, and live hardware evidence.
5. Push only the intended public branch. In the original development checkout,
   the public remote is `github` and the command is `git push github main`.

For screenshots, create an isolated synthetic-data demo first, inspect each
image, then explicitly allow the approved files under `docs/images/` in the
ignore rules. Keep raw screenshots and test artifacts ignored.

The fork keeps its upstream license and history. Changes for this project live
on the public `submission-errors` branch. Pin the exact reviewed revision in
`go.mod` whenever updating it; do not restore a local-directory replacement.
