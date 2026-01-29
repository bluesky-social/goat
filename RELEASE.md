
How to Publish a Release
========================

1. Get repo in a fully clean state: no "dirty" or untracked files, all lints, builds, and tests passing. Remove the `dist/` directory if it exits.
2. Update changelog, including section with new version number, and commit it.
3. Tag a release: `git tag v0.1.2 -asm "new release"`. Release must be run against exactly the tagged commit!
4. Run gorelease: `goreleaser release` (setup credentials in env var if needed)
5. Push to origin, and push tags: `git push origin main` and `git push origin --tags`
