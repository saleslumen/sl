#!/bin/sh
set -eu
REPO="${SL_INSTALL_REPO:-saleslumen/sl}"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*)
	echo "sl: unsupported architecture: $(uname -m)" >&2
	exit 1
	;;
esac
case "$os" in
linux | darwin) ;;
*)
	echo "sl: unsupported OS: $(uname -s)" >&2
	exit 1
	;;
esac
asset="sl-${os}-${arch}.gz"
if [ -n "${SL_INSTALL_VERSION:-}" ]; then
	base="https://github.com/${REPO}/releases/download/${SL_INSTALL_VERSION}"
else
	base="https://github.com/${REPO}/releases/latest/download"
fi
dest="${SL_INSTALL_DIR:-}"
if [ -z "$dest" ]; then
	if [ -w /usr/local/bin ]; then
		dest=/usr/local/bin
	else
		dest="${HOME}/.local/bin"
	fi
fi
tmp=$(mktemp -d)
staged=""
cleanup() {
	rm -rf "$tmp"
	if [ -n "$staged" ]; then
		rm -f "$staged"
	fi
}
trap cleanup EXIT
echo "sl: downloading ${base}/${asset}"
curl -fsSL -o "${tmp}/${asset}" "${base}/${asset}"
curl -fsSL -o "${tmp}/checksums.sha256" "${base}/checksums.sha256"
want=$(awk -v asset="$asset" '$2 == asset { print $1; found=1 } END { exit !found }' "${tmp}/checksums.sha256")
if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "${tmp}/${asset}" | awk '{ print $1 }')
else
	got=$(shasum -a 256 "${tmp}/${asset}" | awk '{ print $1 }')
fi
if [ "$want" != "$got" ]; then
	echo "sl: checksum mismatch for ${asset}" >&2
	exit 1
fi
mkdir -p "$dest"
staged=$(mktemp "${dest}/sl.XXXXXX")
gzip -dc "${tmp}/${asset}" >"$staged"
chmod 0755 "$staged"
mv -f "$staged" "${dest}/sl"
staged=""
echo "sl: installed ${dest}/sl"
"${dest}/sl" version
