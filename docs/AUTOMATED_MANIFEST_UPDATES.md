# Automated Manifest Updates

This document describes the automated systems for updating OpenDataHub component manifest references.

## Overview

The `manifests-config.yaml` file contains references to various OpenDataHub component repositories and pinned ODH/RHOAI Build-Config repositories.
Since our e2e tests rely on these references, they need to be kept up to date but pointing to a branch instead of a specific commit SHA could lead to breakages in e2e tests, and block PRs to being merged.

These references can become outdated as components receive updates, so we provide an automated solution to keep these references current.

## Reference Updates Workflow

It will be executed by a Github Action workflow.

### Location

`.github/workflows/update-manifest-shas.yml`

### Features

- ✅ Runs daily at 6:00 AM UTC
- ✅ Can be triggered manually via workflow_dispatch
- ✅ Only updates components using branch@sha tracking format
- ✅ Fetches latest commit SHAs for tracked branches
- ✅ Creates independent pull requests for component updates
- ✅ Automatic branch cleanup after PR merge
- ✅ Works directly with manifests-config.yaml format
- ✅ Keeps ODH and RHOAI refs for a component together

### Pull Request Units

Each changed component gets its own pull request, so a component-specific check failure does not hold up unrelated component updates. Components with both ODH and RHOAI refs are updated together in that component's pull request.

Two shared units remain grouped:

- **CCM charts** are updated together because their changes regenerate shared Cloud Manager RBAC. A fetch failure for one CCM chart delays the other CCM chart updates until the next successful run.
- **Build-Config** refs are updated together, and their CSV-derived image override updates are resolved in the same pull request.

The workflow continues processing other units when a unit fails. It reports failures in the workflow summary and marks the overall run as failed after the remaining unit jobs finish. Bot pull requests use `sync/upgrade-manifest-shas/` branches and skip the E2E test update requirement.

## Configuration Details

### Components Tracked

The automation only processes components that use the branch@sha tracking format:

- **Components with branch@sha format** (e.g., `main@a1b2c3d4`) - Automatically updated to latest commit SHA
- **Components without @ format** (plain branches, tags) - Skipped

Examples from `manifests-config.yaml`:

- ✅ `"main@1d777fe9b25240f0bd02de90b012c514309c6e63"` - Will be updated
- ❌ `"release-v0.15"` - Will be skipped  
- ❌ `"stable"` - Will be skipped

### Update Frequency

- **Scheduled runs**: Daily at 6:00 AM UTC
- **Manual triggers**: Available via GitHub Actions UI

## Checking imageOverrides

`make resolve-image-digests` fails if an `imageOverrides` row has an illegal key (must start with `RELATED_IMAGE_`), or if `component`, `tagTemplate`, `paramsEnvKey`, or `source` is set to a value that does not match its rules, and it does not rewrite `manifests-config.yaml`. PRs that change that YAML or `cmd/manifest-tools` run the unit tests that cover these errors.

The command fetches the release CSV from each platform's pinned `buildConfig` commit. ODH entries resolve from the ODH CSV and may use downloaded `params.env` values. RHOAI entries do not use those ODH defaults; they resolve from the RHOAI CSV. RHOAI production references under `registry.redhat.io/rhoai/` are stored using the pullable `quay.io/rhoai/` mirror with the same digest.

For `source: csv` entries, existing platform images are refreshed, platforms no longer published are removed, and newly published platforms are added when their image registry matches the import allowlist. Images listed under `odh_exceptions` or `rhoai_exceptions` in `component-params-env.yaml` are not automatically imported for that platform. An existing platform mapping remains explicitly managed until it is removed from `imageOverrides`.
