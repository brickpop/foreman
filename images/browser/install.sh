#!/usr/bin/env bash
# Installs a headless browser and the `screenshot` and `screenshot-diff`
# commands into a folder, with no root: Playwright's Chromium headless shell,
# and the system libraries and fonts it needs, unpacked from Debian packages.
# Runs in the agent image (Debian 13). See README.md.
#
#   images/browser/install.sh <folder>
#
# Each install goes into its own folder, <playwright version>-<cpu>-<hash of
# these files>, which never changes once made, and current-<cpu> points to the
# newest. So an update, or a machine with another CPU, never breaks a folder an
# agent is using. A second run with the same files only checks the pointer.
set -euo pipefail

dest=$(realpath -m "${1:?usage: install.sh <folder>}")
src=$(cd "$(dirname "$0")" && pwd)
files=(browser.mjs screenshot.mjs screenshot-diff.mjs browser.test.mjs package.json package-lock.json README.md)

. /etc/os-release
[ "$ID $VERSION_ID" = "debian 13" ] || { echo "install.sh: needs Debian 13 (the agent image), not $PRETTY_NAME" >&2; exit 1; }
cpu=$(dpkg --print-architecture)
version=$(node -p 'require(process.argv[1]).dependencies.playwright' "$src/package.json")
hash=$(cd "$src" && cat install.sh "${files[@]}" | sha256sum | cut -c1-8)
name="$version-$cpu-$hash"
mkdir -p "$dest"
# One install at a time into a folder.
exec 9>"$dest/.lock"
flock 9

if [ ! -e "$dest/$name/PASSED" ]; then
  rm -rf "${dest:?}/$name" # an install cut short: nobody uses it
  new=$(mktemp -d "$dest/.new-XXXXXX")
  # Debian packages unpack here, in the container: tar can't make their
  # symlinks in a folder shared from the Mac.
  work=$(mktemp -d)
  trap 'rm -rf "$new" "$work"' EXIT
  final="$dest/$name"

  echo "Node packages"
  (cd "$src" && cp "${files[@]}" "$new"/)
  (cd "$new" && npm ci --ignore-scripts --no-audit --no-fund --loglevel=error)

  # The Chromium build this Playwright version expects, laid out as its own
  # installer would, but checked against a pinned SHA-256 first (Audy's L-1).
  # A Playwright bump changes the build: update the sums with the version.
  echo "Chromium headless shell"
  read -r revision build < <(node -p 'const b = require(process.argv[1]).browsers.find((b) => b.name === "chromium-headless-shell"); b.revision + " " + b.browserVersion' "$new/node_modules/playwright-core/browsers.json")
  case $cpu in
    arm64) platform=linux-arm64 sum=e1262e9f4f244c441dcb54aa0e9abe4cf7b266ab02dc666fd975bd55aba9b3dc ;;
    amd64) platform=linux64 sum=77a6442fd3c8acd3bef9e1067fc26f5ef464277d22f0e8cb58dc3e4077a3e458 ;;
    *) echo "install.sh: no Chromium build for $cpu" >&2; exit 1 ;;
  esac
  [ "$build" = "159.0.8154.6" ] || { echo "install.sh: Playwright wants Chromium $build: update the pinned sums" >&2; exit 1; }
  zip="$work/chrome-headless-shell-$platform.zip"
  curl -fsSL --retry 3 -o "$zip" "https://cdn.playwright.dev/builds/cft/$build/$platform/chrome-headless-shell-$platform.zip"
  echo "$sum  $zip" | sha256sum --check --quiet || { echo "install.sh: the Chromium zip doesn't match its pinned SHA-256" >&2; exit 1; }
  dir="$new/browsers/chromium_headless_shell-$revision"
  mkdir -p "$dir"
  unzip -q "$zip" -d "$dir"
  touch "$dir/INSTALLATION_COMPLETE" "$dir/DEPENDENCIES_VALIDATED"
  shell="$dir/chrome-headless-shell-$platform/chrome-headless-shell"

  # The libraries Playwright installs with --with-deps on Debian, less the ones
  # for a headed browser, and two font families: Liberation (the metrics of
  # Arial, Times and Courier) and colour emoji. apt resolves them against the
  # packages the image already has, keeping its own state in a scratch folder.
  echo "Libraries and fonts"
  apt="$work/apt"
  mkdir -p "$apt/lists/partial" "$apt/archives/partial" "$apt/parts" "$work/debs" "$work/root"
  # Without the image's apt.conf.d, whose cleanup hooks would empty /var/cache.
  echo "Dir::Etc::Parts \"$apt/parts\";" > "$apt/apt.conf"
  export APT_CONFIG="$apt/apt.conf"
  opts=(-qq -o "Dir::State::Lists=$apt/lists" -o "Dir::Cache=$apt" -o "Dir::Cache::archives=$apt/archives" -o Debug::NoLocking=1)
  apt-get "${opts[@]}" update
  packages=(
    libasound2t64 libatk-bridge2.0-0t64 libatk1.0-0t64 libatspi2.0-0t64 libcairo2 libcups2t64
    libdbus-1-3 libdrm2 libgbm1 libglib2.0-0t64 libnspr4 libnss3 libpango-1.0-0 libx11-6 libxcb1
    libxcomposite1 libxdamage1 libxext6 libxfixes3 libxkbcommon0 libxrandr2 libfontconfig1 libfreetype6
    fontconfig fonts-liberation fonts-noto-color-emoji
  )
  missing=$(apt-get "${opts[@]}" -s install --no-install-recommends "${packages[@]}" | awk '/^Inst/{print $2}')
  # shellcheck disable=SC2086 # one package per word
  [ -z "$missing" ] || (cd "$work/debs" && apt-get "${opts[@]}" download $missing)
  unset APT_CONFIG
  for deb in "$work"/debs/*.deb; do [ -e "$deb" ] && dpkg-deb -x "$deb" "$work/root"; done
  rm -rf "$work/root/usr/share/doc" "$work/root/usr/share/man"
  cp -a "$work/root" "$new/root"

  # Fonts come only from the folder, so screenshots look the same in every
  # container. The paths are where the folder ends up.
  mkdir -p "$new/fontcache"
  cat > "$new/fonts.conf" <<EOF
<?xml version="1.0"?>
<fontconfig>
  <dir>$final/root/usr/share/fonts</dir>
  <cachedir>$final/fontcache</cachedir>
  <alias><family>sans-serif</family><prefer><family>Liberation Sans</family><family>Noto Color Emoji</family></prefer></alias>
  <alias><family>system-ui</family><prefer><family>Liberation Sans</family><family>Noto Color Emoji</family></prefer></alias>
  <alias><family>serif</family><prefer><family>Liberation Serif</family><family>Noto Color Emoji</family></prefer></alias>
  <alias><family>monospace</family><prefer><family>Liberation Mono</family><family>Noto Color Emoji</family></prefer></alias>
  <alias><family>sans</family><prefer><family>Liberation Sans</family></prefer></alias>
  <!-- fontconfig's own rule (49-sansserif.conf): a family it doesn't know falls back to sans-serif. -->
  <match target="pattern">
    <test qual="all" name="family" compare="not_eq"><string>sans-serif</string></test>
    <test qual="all" name="family" compare="not_eq"><string>serif</string></test>
    <test qual="all" name="family" compare="not_eq"><string>monospace</string></test>
    <edit name="family" mode="append_last"><string>sans-serif</string></edit>
  </match>
</fontconfig>
EOF

  # env.sh is for the commands, and for anyone running Playwright from here.
  cat > "$new/env.sh" <<'EOF'
# Source this to run Playwright with the browser in this folder.
browser_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
export PLAYWRIGHT_BROWSERS_PATH="$browser_dir/browsers"
export LD_LIBRARY_PATH="$(find "$browser_dir/root" -name '*.so*' -printf '%h\n' | sort -u | paste -sd:)${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export FONTCONFIG_FILE="$browser_dir/fonts.conf"
EOF
  mkdir -p "$new/bin"
  for cmd in screenshot screenshot-diff; do
    printf '#!/usr/bin/env bash\n. "$(dirname "$(realpath "$0")")/../env.sh"\nexec node "$browser_dir/%s.mjs" "$@"\n' "$cmd" > "$new/bin/$cmd"
    chmod +x "$new/bin/$cmd"
  done

  # Missing libraries show here, not at the first screenshot.
  (. "$new/env.sh" && ! ldd "$shell" | grep 'not found') || { echo "install.sh: the libraries above are missing" >&2; exit 1; }

  chmod -R a+rX "$new"
  mv "$new" "$final"
  rm -rf "$work"
  trap - EXIT
  # The font cache, built once here rather than by the first screenshot.
  (. "$final/env.sh" && "$final/root/usr/bin/fc-cache") || echo "install.sh: fc-cache failed: the first screenshot builds the font cache" >&2

  # A folder that can reach the internet, or runs without Chromium's sandbox,
  # never becomes current.
  echo "Browser test"
  (. "$final/env.sh" && cd "$final" && node --test --test-reporter=dot browser.test.mjs) || { rm -rf "$final"; echo "install.sh: the new install failed its test, and is removed" >&2; exit 1; }
  touch "$final/PASSED"
fi

# The pointer and the commands that follow it, replaced atomically.
ln -sfn "$name" "$dest/.current-$cpu"
mv -T "$dest/.current-$cpu" "$dest/current-$cpu"
mkdir -p "$dest/bin"
for cmd in screenshot screenshot-diff; do
  printf '#!/usr/bin/env bash\nexec "$(dirname "$(realpath "$0")")/../current-$(dpkg --print-architecture)/bin/%s" "$@"\n' "$cmd" > "$dest/bin/.$cmd"
  chmod +x "$dest/bin/.$cmd"
  mv "$dest/bin/.$cmd" "$dest/bin/$cmd"
done
cp "$src/README.md" "$dest/.README.md" && mv "$dest/.README.md" "$dest/README.md"
echo "current-$cpu: $name ($(du -sh "$dest/$name" | cut -f1))"
