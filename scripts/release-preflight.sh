#!/bin/sh
set -eu

push_mode=0
target_version=""

for arg in "$@"; do
    case "$arg" in
        --push)
            push_mode=1
            ;;
        --help|-h)
            printf 'Usage: %s [--push] <version>\n\n' "$0"
            printf 'Verifies release readiness for <version> against:\n'
            printf '  1. Clean working tree\n'
            printf '  2. Current branch is main\n'
            printf '  3. HEAD is the commit at origin/main (fetched now)\n'
            printf '  4. Tag does not already exist locally or remotely\n'
            printf '  5. CI is green for HEAD commit\n\n'
            printf 'Without --push, reports check status and prints the command to push.\n'
            printf 'With --push, creates an annotated tag and pushes it to origin.\n'
            exit 0
            ;;
        -*)
            printf 'ERROR: Unknown option %s\n' "$arg" >&2
            exit 1
            ;;
        *)
            if [ -n "$target_version" ]; then
                printf 'ERROR: Multiple version arguments specified: "%s" and "%s"\n' "$target_version" "$arg" >&2
                exit 1
            fi
            target_version="$arg"
            ;;
    esac
done

if [ -z "$target_version" ]; then
    printf 'ERROR: A version argument is required (e.g. 1.0.0 or v1.0.0).\n' >&2
    printf 'Usage: %s [--push] <version>\n' "$0" >&2
    exit 1
fi

case "$target_version" in
    v*)
        tag="$target_version"
        raw_version="${target_version#v}"
        ;;
    *)
        tag="v$target_version"
        raw_version="$target_version"
        ;;
esac

# Validate SemVer format
semver_regex='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
if ! printf '%s\n' "$raw_version" | grep -Eq "$semver_regex"; then
    printf 'ERROR: Version "%s" is not valid SemVer.\n' "$raw_version" >&2
    exit 1
fi

# Check numeric prerelease identifiers for leading zeros
version_without_build="${raw_version%%+*}"
if [ "$version_without_build" != "${version_without_build#*-}" ]; then
    prerelease_identifiers="${version_without_build#*-}"
    old_ifs="$IFS"
    IFS='.'
    for id in $prerelease_identifiers; do
        case "$id" in
            0[0-9]*)
                IFS="$old_ifs"
                printf 'ERROR: Numeric prerelease identifier "%s" must not contain leading zeros.\n' "$id" >&2
                exit 1
                ;;
        esac
    done
    IFS="$old_ifs"
fi

printf 'Running release preflight checks for %s (version: %s)...\n\n' "$tag" "$raw_version"

# 1. Working tree clean
printf '[1/5] Checking working tree cleanliness... '
dirty="$(git status --porcelain 2>/dev/null)"
if [ -n "$dirty" ]; then
    printf 'FAILED\n'
    printf 'ERROR: Working tree is not clean. Uncommitted changes:\n%s\n' "$dirty" >&2
    exit 1
fi
printf 'OK\n'

# 2. On main branch
printf '[2/5] Checking current branch... '
current_branch="$(git branch --show-current 2>/dev/null || true)"
if [ "$current_branch" != "main" ]; then
    printf 'FAILED\n'
    printf 'ERROR: Releases must be cut from "main" branch (currently on "%s").\n' "$current_branch" >&2
    exit 1
fi
printf 'OK (main)\n'

# 3. HEAD is what origin/main holds. CI also runs on pull requests, so a green
# run alone does not prove the commit reached main.
printf '[3/5] Checking HEAD against origin/main... '
if ! fetch_err="$(git fetch --quiet origin main 2>&1)"; then
    printf 'FAILED\n'
    printf 'ERROR: Could not fetch main from origin:\n%s\n' "$fetch_err" >&2
    exit 1
fi
head_sha="$(git rev-parse HEAD)"
remote_sha="$(git rev-parse --verify -q refs/remotes/origin/main || true)"
if [ -z "$remote_sha" ]; then
    printf 'FAILED\n'
    printf 'ERROR: origin/main does not exist after fetching.\n' >&2
    exit 1
fi
if [ "$head_sha" != "$remote_sha" ]; then
    printf 'FAILED\n'
    if git merge-base --is-ancestor "$remote_sha" "$head_sha"; then
        printf 'ERROR: HEAD (%s) has commits that are not on origin/main (%s). Push main first, wait for CI, then rerun.\n' "$head_sha" "$remote_sha" >&2
    elif git merge-base --is-ancestor "$head_sha" "$remote_sha"; then
        printf 'ERROR: HEAD (%s) is behind origin/main (%s). Pull main, then rerun.\n' "$head_sha" "$remote_sha" >&2
    else
        printf 'ERROR: HEAD (%s) and origin/main (%s) have diverged. Reconcile main with origin, then rerun.\n' "$head_sha" "$remote_sha" >&2
    fi
    exit 1
fi
printf 'OK (%s)\n' "$head_sha"

# 4. Tag does not already exist locally or remotely
printf '[4/5] Checking tag availability (%s)... ' "$tag"
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null 2>&1; then
    printf 'FAILED\n'
    printf 'ERROR: Tag "%s" already exists locally.\n' "$tag" >&2
    exit 1
fi
remote_tag="$(git ls-remote --tags origin "refs/tags/$tag" 2>/dev/null || true)"
if [ -n "$remote_tag" ]; then
    printf 'FAILED\n'
    printf 'ERROR: Tag "%s" already exists on remote origin.\n' "$tag" >&2
    exit 1
fi
printf 'OK\n'

# 5. CI is green for HEAD
printf '[5/5] Checking CI status for HEAD... '
if ! command -v gh >/dev/null 2>&1; then
    printf 'FAILED\n'
    printf 'ERROR: "gh" CLI is required to verify CI status.\n' >&2
    exit 1
fi
ci_res="$(gh run list --commit "$head_sha" --workflow "CI" --json status,conclusion -q '.[0].status + " " + .[0].conclusion' 2>/dev/null || true)"
if [ -z "$ci_res" ] || [ "$ci_res" = "null null" ]; then
    printf 'FAILED\n'
    printf 'ERROR: No CI workflow run found for HEAD commit %s.\n' "$head_sha" >&2
    exit 1
fi
ci_status="${ci_res%% *}"
ci_conclusion="${ci_res#* }"

if [ "$ci_status" != "completed" ]; then
    printf 'FAILED\n'
    printf 'ERROR: CI run for HEAD (%s) is not completed (current status: %s).\n' "$head_sha" "$ci_status" >&2
    exit 1
fi

if [ "$ci_conclusion" != "success" ]; then
    printf 'FAILED\n'
    printf 'ERROR: CI run for HEAD (%s) did not succeed (conclusion: %s).\n' "$head_sha" "$ci_conclusion" >&2
    exit 1
fi
printf 'OK (success)\n\n'

printf 'All preflight checks PASSED for %s.\n\n' "$tag"

if [ "$push_mode" -eq 1 ]; then
    printf 'Creating annotated tag %s...\n' "$tag"
    git tag -a "$tag" -m "Release $tag"
    printf 'Pushing tag %s to origin...\n' "$tag"
    git push origin "$tag"
    printf 'Successfully tagged and pushed %s. Release workflow triggered!\n' "$tag"
else
    printf 'Preflight verification succeeded. Ready to push release tag.\n'
    printf 'To create and push the release tag, run:\n\n'
    printf '  %s --push %s\n\n' "$0" "$raw_version"
    printf 'Or execute the Git commands manually:\n\n'
    printf '  git tag -a %s -m "Release %s"\n' "$tag" "$tag"
    printf '  git push origin %s\n\n' "$tag"
fi
