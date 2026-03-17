# Build And Docker Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Shrink the Codex-only release binary and Docker image without changing runtime behavior.

**Architecture:** Keep the existing Go builder stage, but make the release flags explicit and reusable across local builds and Docker builds. Switch the runtime image to `scratch`, while preserving bootstrap config generation and CA certificates for outbound HTTPS.

**Tech Stack:** Go 1.26, Docker multi-stage builds, vendored Go modules, embedded management UI.

---

### Task 1: Unify Release Build Flags

**Files:**
- Create: `build-release.sh`
- Modify: `README.md`
- Modify: `README_CN.md`

**Step 1:** Add a local release script that builds `./bin/codex-proxy` with static linking, stripped symbols, `-trimpath`, and build metadata flags.

**Step 2:** Point both READMEs to the release script instead of the plain `go build` example so local and Docker builds use the same release posture.

### Task 2: Optimize Docker Packaging

**Files:**
- Modify: `Dockerfile`

**Step 1:** Enable modern Dockerfile syntax so BuildKit cache mounts can be used.

**Step 2:** Add a Go build cache mount and pass `-buildvcs=false` plus the same release flags used by the local release script.

**Step 3:** Replace the Alpine runtime stage with `scratch`, keeping the binary, CA bundle, and example config in the final image.

### Task 3: Verify Outputs

**Files:**
- None

**Step 1:** Run the local release build script and confirm the output binary exists.

**Step 2:** Build the Docker image and confirm it succeeds with the updated Dockerfile.

**Step 3:** Run the container and confirm `/` still redirects to `/management.html`.
