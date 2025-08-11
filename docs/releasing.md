## Releasing lash

This document describes how to cut a new release using GitHub CLI and update the Homebrew tap to make the version installable via Homebrew. It assumes you have push access to `lacymorrow/lash` and `lacymorrow/homebrew-tap`.

### Prerequisites
- GitHub CLI installed and authenticated: `gh auth login`
- Local git configured and up to date with `origin` pointing to `https://github.com/lacymorrow/lash.git`
- Homebrew tap repo exists: `github.com/lacymorrow/homebrew-tap` (public, empty or with `Formula/`)

### One-time setup
```bash
# Set default repo for gh in this directory
gh repo set-default lacymorrow/lash
```

If the tap repository does not exist yet, create it:
```bash
gh repo create lacymorrow/homebrew-tap --public --description "Homebrew tap for lash"
```

### Release steps
Use the following steps to tag, create the GitHub release, and update the Homebrew formula. These commands are safe to run in zsh; where noted, they disable zsh correction/globbing.

1) Choose a new version and tag it
```bash
VERSION=v0.4.3
git tag "$VERSION"
git push origin "$VERSION"
```

2) Create the GitHub release with notes
```bash
gh release create "$VERSION" --generate-notes --latest --title "$VERSION"
```

3) Update the Homebrew tap formula

This generates the Ruby formula that builds from source, installs shell completions and manpage, and pushes it to `lacymorrow/homebrew-tap`.

Note for zsh users: the first two commands disable correction and globbing to avoid quoting issues.
```bash
# (zsh only) make quoting predictable for the block below
unsetopt correct_all 2>/dev/null || true
set -f

URL="https://github.com/lacymorrow/lash/archive/refs/tags/${VERSION}.tar.gz"
SHA=$(curl -fsSL "$URL" | shasum -a 256 | awk '{print $1}')

FORMULA_CONTENT="$(cat <<'RUBY'
class Lash < Formula
  desc "Terminal-based AI assistant for developers"
  homepage "https://github.com/lacymorrow/lash"
  url "__URL__"
  sha256 "__SHA__"
  license "MIT"
  head "https://github.com/lacymorrow/lash.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/lacymorrow/lash/internal/version.Version=v\#{version}"
    system "go", "build", "-trimpath", "-ldflags=\#{ldflags}", "-o", bin/"lash", "."

    (buildpath/"lash.bash").write Utils.safe_popen_read(bin/"lash", "completion", "bash")
    (buildpath/"_lash").write Utils.safe_popen_read(bin/"lash", "completion", "zsh")
    (buildpath/"lash.fish").write Utils.safe_popen_read(bin/"lash", "completion", "fish")
    bash_completion.install buildpath/"lash.bash" => "lash"
    zsh_completion.install buildpath/"_lash"
    fish_completion.install buildpath/"lash.fish"

    (buildpath/"lash.1").write Utils.safe_popen_read(bin/"lash", "man")
    system "gzip", buildpath/"lash.1"
    man1.install buildpath/"lash.1.gz"
  end

  test do
    assert_match version.to_s, shell_output("\#{bin}/lash --version")
  end
end
RUBY
)"

# Fill in URL and SHA, then base64 encode for GitHub API
FORMULA_FILLED="${FORMULA_CONTENT/__URL__/$URL}"
FORMULA_FILLED="${FORMULA_FILLED/__SHA__/$SHA}"
B64=$(printf '%s' "$FORMULA_FILLED" | base64 | tr -d '\n')

# Create or update Formula/lash.rb on main branch of tap repo
CUR_SHA=$(gh api repos/lacymorrow/homebrew-tap/contents/Formula/lash.rb -q .sha 2>/dev/null || true)
ARGS=( -X PUT repos/lacymorrow/homebrew-tap/contents/Formula/lash.rb \
  -f message="lash ${VERSION}: formula" -f content="$B64" -f branch=main )
[ -n "$CUR_SHA" ] && ARGS+=( -f sha="$CUR_SHA" )
gh api "${ARGS[@]}" | cat
```

4) Verify install
```bash
brew tap lacymorrow/tap
brew install lacymorrow/tap/lash
lash --version
```

### Fallback install
If Homebrew is not desired or while waiting for the tap update:
```bash
go install github.com/lacymorrow/lash@latest
```

### Notes
- Releases are tagged with `vX.Y.Z` and embed the version via `-ldflags`.
- The formula builds from the tagged source tarball and generates completions/manpage at install time.
- If you rename branches or repos, adjust URLs accordingly.


