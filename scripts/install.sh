#!/bin/sh
# Download, verify, atomically install, upgrade, or uninstall Skillstation.
set -eu

SKILLSTATION_SCRIPT_NAME=${0##*/}
SKILLSTATION_PREFIX=${SKILLSTATION_PREFIX:-"$HOME/.local"}
SKILLSTATION_RELEASE_BASE=${SKILLSTATION_RELEASE_BASE:-https://github.com/vantt/skillstation/releases/download}
DEFAULT_VERSION='@SKILLSTATION_VERSION@'
SKILLSTATION_VERSION=${SKILLSTATION_VERSION:-$DEFAULT_VERSION}
SKILLSTATION_OS=${SKILLSTATION_OS:-}
SKILLSTATION_ARCH=${SKILLSTATION_ARCH:-}
SKILLSTATION_INSTALL_DIR=${SKILLSTATION_INSTALL_DIR:-}
SKILLSTATION_CONFIG_DIR=${SKILLSTATION_CONFIG_DIR:-}
SKILLSTATION_LOCAL_FIXTURE=
SKILLSTATION_DRY_RUN=${SKILLSTATION_DRY_RUN:-0}
SKILLSTATION_UNINSTALL=${SKILLSTATION_UNINSTALL:-0}
SKILLSTATION_PURGE_CONFIG=${SKILLSTATION_PURGE_CONFIG:-0}
SKILLSTATION_YES=${SKILLSTATION_YES:-0}
SKILLSTATION_NO_MODIFY_PATH=${SKILLSTATION_NO_MODIFY_PATH:-0}
SKILLSTATION_REQUIRE_SIGNATURE=${SKILLSTATION_REQUIRE_SIGNATURE:-0}

SKILLSTATION_CONNECT_TIMEOUT=10
SKILLSTATION_DOWNLOAD_TIMEOUT=120
SKILLSTATION_CHECKSUM_MAX_BYTES=1048576
SKILLSTATION_BUNDLE_MAX_BYTES=4194304
SKILLSTATION_ARCHIVE_MAX_BYTES=134217728
SKILLSTATION_SIGSTORE_ISSUER=https://token.actions.githubusercontent.com
SKILLSTATION_SIGSTORE_WORKFLOW=https://github.com/vantt/skillstation/.github/workflows/release.yml
SKILLSTATION_FIXTURE_VERIFIER=${SKILLSTATION_FIXTURE_VERIFIER:-}

skillstation_error() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

skillstation_usage_error() {
	printf 'ERROR: %s\n' "$*" >&2
	printf 'Run %s --help for usage.\n' "$SKILLSTATION_SCRIPT_NAME" >&2
	exit 2
}

skillstation_command_exists() {
	command -v "$1" >/dev/null 2>&1
}

skillstation_resolve_install_paths() {
	if [ -n "${SKILLSTATION_INSTALL_DIR:-}" ]; then
		SKILLSTATION_BIN_DIR=$SKILLSTATION_INSTALL_DIR
	else
		SKILLSTATION_BIN_DIR=$SKILLSTATION_PREFIX/bin
	fi
	SKILLSTATION_BIN_DIR=${SKILLSTATION_BIN_DIR%/}
	SKILLSTATION_TARGET=$SKILLSTATION_BIN_DIR/skillstation
	SKILLSTATION_MARKER=$SKILLSTATION_BIN_DIR/.skillstation-managed
}

skillstation_validate_platform() {
	case "$SKILLSTATION_OS-$SKILLSTATION_ARCH" in
		linux-amd64|linux-arm64|darwin-amd64|darwin-arm64) ;;
		*) skillstation_error "Unsupported shell installer platform '$SKILLSTATION_OS/$SKILLSTATION_ARCH'; supported platforms are linux/amd64, linux/arm64, darwin/amd64, and darwin/arm64. Windows releases are not installed by this shell installer." ;;
	esac
}

skillstation_detect_platform() {
	if [ -z "${SKILLSTATION_OS:-}" ]; then
		case "$(uname -s)" in
			Linux) SKILLSTATION_OS=linux ;;
			Darwin) SKILLSTATION_OS=darwin ;;
			*) skillstation_error "Unsupported operating system '$(uname -s)'." ;;
		esac
	fi
	if [ -z "${SKILLSTATION_ARCH:-}" ]; then
		case "$(uname -m)" in
			x86_64|amd64) SKILLSTATION_ARCH=amd64 ;;
			arm64|aarch64) SKILLSTATION_ARCH=arm64 ;;
			*) skillstation_error "Unsupported architecture '$(uname -m)'." ;;
		esac
	fi
	skillstation_validate_platform
}

skillstation_validate_version() {
	[ -n "$SKILLSTATION_VERSION" ] && [ "$SKILLSTATION_VERSION" != '@SKILLSTATION_VERSION@' ] || skillstation_error "A release version is required. Pass --version VERSION or set SKILLSTATION_VERSION."
	version=$SKILLSTATION_VERSION
	case "$version" in
		v*) version=${version#v} ;;
	esac
	invalid_version="Invalid release version '$SKILLSTATION_VERSION'; expected SemVer such as 1.2.3, v1.2.3, or v1.2.3-rc.1+build.5."
	case "$version" in
		''|*[!0-9A-Za-z.+-]*) skillstation_error "$invalid_version" ;;
	esac

	core_prerelease=$version
	case "$version" in
		*+*)
			build=${version#*+}
			core_prerelease=${version%%+*}
			case "$build" in
				''|*+*|.*|*.|*..*|*[!0-9A-Za-z.-]*) skillstation_error "$invalid_version" ;;
			esac
			old_ifs=$IFS
			IFS=.
			set -- $build
			IFS=$old_ifs
			for identifier in "$@"; do
				case "$identifier" in
					''|*[!0-9A-Za-z-]*) skillstation_error "$invalid_version" ;;
				esac
			done
			;;
	esac

	core=$core_prerelease
	case "$core_prerelease" in
		*-*)
			prerelease=${core_prerelease#*-}
			core=${core_prerelease%%-*}
			case "$prerelease" in
				''|.*|*.|*..*|*[!0-9A-Za-z.-]*) skillstation_error "$invalid_version" ;;
			esac
			old_ifs=$IFS
			IFS=.
			set -- $prerelease
			IFS=$old_ifs
			for identifier in "$@"; do
				case "$identifier" in
					''|*[!0-9A-Za-z-]*) skillstation_error "$invalid_version" ;;
					0|*[!0-9]*) ;;
					0*) skillstation_error "$invalid_version" ;;
				esac
			done
			;;
	esac

	case "$core" in
		''|.*|*.|*..*|*[!0-9.]*) skillstation_error "$invalid_version" ;;
	esac
	old_ifs=$IFS
	IFS=.
	set -- $core
	IFS=$old_ifs
	[ "$#" -eq 3 ] || skillstation_error "$invalid_version"
	for component in "$@"; do
		case "$component" in
			0|[1-9]*) ;;
			*) skillstation_error "$invalid_version" ;;
		esac
		case "$component" in
			*[!0-9]*|0?*) skillstation_error "$invalid_version" ;;
		esac
	done
	SKILLSTATION_ARTIFACT_VERSION=$version
	SKILLSTATION_RELEASE_TAG=v$version
	SKILLSTATION_VERSION=$version
}

skillstation_validate_source() {
	if [ -n "$SKILLSTATION_LOCAL_FIXTURE" ]; then
		[ -d "$SKILLSTATION_LOCAL_FIXTURE" ] || skillstation_error "Local fixture directory not found: $SKILLSTATION_LOCAL_FIXTURE"
		[ -n "$SKILLSTATION_FIXTURE_VERIFIER" ] || skillstation_error "Local fixture mode requires SKILLSTATION_FIXTURE_VERIFIER."
		[ -f "$SKILLSTATION_FIXTURE_VERIFIER" ] && [ -x "$SKILLSTATION_FIXTURE_VERIFIER" ] || skillstation_error "Fixture verifier is not an executable file: $SKILLSTATION_FIXTURE_VERIFIER"
		return
	fi
	case "$SKILLSTATION_RELEASE_BASE" in
		https://*) ;;
		*) skillstation_error "Release base must use HTTPS. Use --local-fixture DIR for explicit local test data." ;;
	esac
}

skillstation_require_tools() {
	skillstation_command_exists tar || skillstation_error "Required command not found: tar"
	if ! skillstation_command_exists sha256sum && ! skillstation_command_exists shasum && ! skillstation_command_exists openssl; then
		skillstation_error "SHA-256 verification requires sha256sum, shasum, or openssl."
	fi
	if [ -z "$SKILLSTATION_LOCAL_FIXTURE" ]; then
		skillstation_command_exists curl || skillstation_error "Downloading releases requires curl."
		if [ "$SKILLSTATION_REQUIRE_SIGNATURE" -eq 1 ] && ! skillstation_command_exists cosign; then
			skillstation_error "Signature verification is required (SKILLSTATION_REQUIRE_SIGNATURE=1) but cosign is not installed. Install cosign from https://github.com/sigstore/cosign to continue."
		fi
	fi
}

skillstation_make_temp_dir() {
	SKILLSTATION_TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/skillstation-install.XXXXXXXX") || skillstation_error "Could not create a temporary directory."
}

skillstation_cleanup_temp() {
	if [ -n "${SKILLSTATION_TEMP_DIR:-}" ] && [ -d "$SKILLSTATION_TEMP_DIR" ]; then
		rm -rf "$SKILLSTATION_TEMP_DIR"
	fi
}

skillstation_file_size() {
	wc -c < "$1" | tr -d '[:space:]'
}

skillstation_enforce_size() {
	file=$1
	max_bytes=$2
	label=$3
	size=$(skillstation_file_size "$file") || skillstation_error "Could not determine size of $label."
	case "$size" in
		''|*[!0-9]*) skillstation_error "Could not determine size of $label." ;;
	esac
	[ "$size" -le "$max_bytes" ] || skillstation_error "$label exceeds the maximum allowed size of $max_bytes bytes."
}

skillstation_download() {
	source_name=$1
	destination=$2
	max_bytes=$3
	if [ -n "$SKILLSTATION_LOCAL_FIXTURE" ]; then
		[ -f "$SKILLSTATION_LOCAL_FIXTURE/$source_name" ] || skillstation_error "Fixture file not found: $SKILLSTATION_LOCAL_FIXTURE/$source_name"
		skillstation_enforce_size "$SKILLSTATION_LOCAL_FIXTURE/$source_name" "$max_bytes" "$source_name"
		cp "$SKILLSTATION_LOCAL_FIXTURE/$source_name" "$destination" || skillstation_error "Could not copy fixture file: $source_name"
		return
	fi
	url=${SKILLSTATION_RELEASE_BASE%/}/$SKILLSTATION_RELEASE_TAG/$source_name
	case "$url" in
		https://*) ;;
		*) skillstation_error "Refusing non-HTTPS download: $url" ;;
	esac
	curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error \
		--connect-timeout "$SKILLSTATION_CONNECT_TIMEOUT" --max-time "$SKILLSTATION_DOWNLOAD_TIMEOUT" \
		--max-filesize "$max_bytes" --output "$destination" "$url" || skillstation_error "Download failed: $url"
	skillstation_enforce_size "$destination" "$max_bytes" "$source_name"
}

skillstation_sha256() {
	file=$1
	if skillstation_command_exists sha256sum; then
		sha256sum "$file" | awk '{print $1}'
	elif skillstation_command_exists shasum; then
		shasum -a 256 "$file" | awk '{print $1}'
	else
		openssl dgst -sha256 "$file" | awk '{print $NF}'
	fi
}

skillstation_verify_manifest() {
	manifest=$1
	bundle=$2
	identity=$SKILLSTATION_SIGSTORE_WORKFLOW@refs/tags/$SKILLSTATION_RELEASE_TAG
	if [ -n "$SKILLSTATION_LOCAL_FIXTURE" ]; then
		"$SKILLSTATION_FIXTURE_VERIFIER" "$manifest" "$bundle" "$identity" "$SKILLSTATION_SIGSTORE_ISSUER" || skillstation_error "Fixture checksum manifest authentication failed."
	else
		cosign verify-blob --bundle "$bundle" --certificate-identity "$identity" \
			--certificate-oidc-issuer "$SKILLSTATION_SIGSTORE_ISSUER" "$manifest" >/dev/null || skillstation_error "Sigstore verification failed for checksums.txt."
	fi
	printf 'Authenticated checksums.txt with Sigstore bundle\n'
}

skillstation_verify_archive() {
	archive=$1
	manifest=$2
	archive_name=$3
	expected=
	matches=0
	while IFS=' ' read -r hash listed_name extra; do
		[ -n "$hash" ] || continue
		listed_name=${listed_name#\*}
		case "$listed_name" in
			"$archive_name")
				[ -z "${extra:-}" ] || skillstation_error "Malformed checksum entry for $archive_name."
				expected=$hash
				matches=$((matches + 1))
				;;
		esac
	done < "$manifest"
	[ "$matches" -eq 1 ] || skillstation_error "Checksum manifest must contain exactly one entry for $archive_name."
	case "$expected" in
		*[!0-9A-Fa-f]*|'') skillstation_error "Invalid SHA-256 checksum for $archive_name." ;;
	esac
	[ "${#expected}" -eq 64 ] || skillstation_error "Invalid SHA-256 checksum length for $archive_name."
	actual=$(skillstation_sha256 "$archive") || skillstation_error "Could not calculate SHA-256 for $archive_name."
	expected=$(printf '%s' "$expected" | tr 'A-F' 'a-f')
	[ "$actual" = "$expected" ] || skillstation_error "SHA-256 verification failed for $archive_name."
	printf 'Verified SHA-256 for %s\n' "$archive_name"
}

skillstation_fetch_candidate() {
	archive_name=skillstation-$SKILLSTATION_ARTIFACT_VERSION-$SKILLSTATION_OS-$SKILLSTATION_ARCH.tar.gz
	archive=$SKILLSTATION_TEMP_DIR/$archive_name
	manifest=$SKILLSTATION_TEMP_DIR/checksums.txt
	bundle=$SKILLSTATION_TEMP_DIR/checksums.txt.sigstore.json
	extract_dir=$SKILLSTATION_TEMP_DIR/extract
	mkdir "$extract_dir" || skillstation_error "Could not prepare extraction directory."
	skillstation_download checksums.txt "$manifest" "$SKILLSTATION_CHECKSUM_MAX_BYTES"

	identity=$SKILLSTATION_SIGSTORE_WORKFLOW@refs/tags/$SKILLSTATION_RELEASE_TAG
	if [ -n "$SKILLSTATION_LOCAL_FIXTURE" ]; then
		skillstation_download checksums.txt.sigstore.json "$bundle" "$SKILLSTATION_BUNDLE_MAX_BYTES"
		skillstation_verify_manifest "$manifest" "$bundle"
	elif skillstation_command_exists cosign; then
		skillstation_download checksums.txt.sigstore.json "$bundle" "$SKILLSTATION_BUNDLE_MAX_BYTES"
		skillstation_verify_manifest "$manifest" "$bundle"
	else
		printf 'Signature not checked (cosign not installed). To verify: %s\n' \
			"cosign verify-blob --bundle checksums.txt.sigstore.json --certificate-identity $identity --certificate-oidc-issuer $SKILLSTATION_SIGSTORE_ISSUER checksums.txt"
	fi

	skillstation_download "$archive_name" "$archive" "$SKILLSTATION_ARCHIVE_MAX_BYTES"
	skillstation_verify_archive "$archive" "$manifest" "$archive_name"
	entries=$(tar -tzf "$archive") || skillstation_error "Could not inspect release archive: $archive_name"
	case "$entries" in
		skillstation) archive_member=skillstation ;;
		./skillstation) archive_member=./skillstation ;;
		*) skillstation_error "Release archive must contain exactly one non-traversing skillstation binary." ;;
	esac
	entry_details=$(LC_ALL=C tar -tvzf "$archive") || skillstation_error "Could not inspect release archive type: $archive_name"
	case "$entry_details" in
		-*) ;;
		*) skillstation_error "Release archive's skillstation entry must be a regular file, not a link or special file." ;;
	esac
	tar -xzf "$archive" -C "$extract_dir" "$archive_member" || skillstation_error "Could not extract release archive: $archive_name"
	SKILLSTATION_CANDIDATE=$extract_dir/skillstation
	[ -f "$SKILLSTATION_CANDIDATE" ] && [ ! -L "$SKILLSTATION_CANDIDATE" ] || skillstation_error "Release archive does not contain a regular skillstation binary."
	chmod 755 "$SKILLSTATION_CANDIDATE" || skillstation_error "Could not make the downloaded binary executable."
	"$SKILLSTATION_CANDIDATE" version >/dev/null 2>&1 || skillstation_error "Downloaded skillstation binary failed its version check."
}

skillstation_is_managed() {
	[ -f "$SKILLSTATION_MARKER" ] && [ ! -L "$SKILLSTATION_MARKER" ] && [ "$(sed -n '1p' "$SKILLSTATION_MARKER")" = "skillstation-managed-v1" ]
}

skillstation_path_contains() {
	check_dir=$1
	case ":$PATH:" in
		*:"$check_dir":*) return 0 ;;
		*) return 1 ;;
	esac
}

skillstation_append_marked_block() {
	target_file=$1
	entry_line=$2

	if [ -f "$target_file" ] && grep -q '# >>> skillstation >>>' "$target_file" 2>/dev/null; then
		return 0
	fi

	if [ -e "$target_file" ] && [ ! -w "$target_file" ]; then
		printf 'Notice: %s is read-only; skipping shell profile update. Add this line manually:\n  %s\n' "$target_file" "$entry_line"
		return 0
	fi

	target_dir=$(dirname "$target_file")
	if [ ! -d "$target_dir" ]; then
		mkdir -p "$target_dir" || {
			printf 'Notice: could not create directory %s; skipping shell profile update.\n' "$target_dir"
			return 0
		}
	fi
	if [ ! -w "$target_dir" ]; then
		printf 'Notice: directory %s is read-only; skipping shell profile update.\n' "$target_dir"
		return 0
	fi

	if [ -s "$target_file" ] && [ -n "$(tail -c 1 "$target_file" 2>/dev/null)" ]; then
		printf '\n' >> "$target_file"
	fi

	cat >> "$target_file" <<EOF
# >>> skillstation >>>
$entry_line
# <<< skillstation <<<
EOF
	[ -n "${SKILLSTATION_PRIMARY_MODIFIED:-}" ] || SKILLSTATION_PRIMARY_MODIFIED=$target_file
}

skillstation_configure_path() {
	if skillstation_path_contains "$SKILLSTATION_BIN_DIR"; then
		return 0
	fi

	if [ "$SKILLSTATION_NO_MODIFY_PATH" -eq 1 ]; then
		printf 'Notice: %s is not in your PATH.\n' "$SKILLSTATION_BIN_DIR"
		printf 'Add it to your shell configuration file by running:\n'
		case "${SHELL:-}" in
			*fish*)
				printf '  fish_add_path "%s"\n' "$SKILLSTATION_BIN_DIR"
				;;
			*)
				printf '  export PATH="%s:$PATH"\n' "$SKILLSTATION_BIN_DIR"
				;;
		esac
		return 0
	fi

	SKILLSTATION_PRIMARY_MODIFIED=""

	posix_line="export PATH=\"$SKILLSTATION_BIN_DIR:\$PATH\""
	fish_line="fish_add_path \"$SKILLSTATION_BIN_DIR\""

	bashrc="$HOME/.bashrc"
	bash_profile="$HOME/.bash_profile"
	zdotdir=${ZDOTDIR:-$HOME}
	zshrc="$zdotdir/.zshrc"
	fish_conf_dir="${XDG_CONFIG_HOME:-$HOME/.config}/fish/conf.d"
	fish_rc="$fish_conf_dir/skillstation.fish"
	profile="$HOME/.profile"

	has_bash=0
	has_zsh=0
	has_fish=0

	case "${SHELL:-}" in
		*bash*) has_bash=1 ;;
		*zsh*) has_zsh=1 ;;
		*fish*) has_fish=1 ;;
	esac

	[ -f "$bashrc" ] && has_bash=1
	[ -f "$zshrc" ] && has_zsh=1
	[ -f "$fish_rc" ] || [ -d "${XDG_CONFIG_HOME:-$HOME/.config}/fish" ] && has_fish=1

	edited_any=0

	if [ "$has_bash" -eq 1 ]; then
		skillstation_append_marked_block "$bashrc" "$posix_line"
		edited_any=1
		if [ "$SKILLSTATION_OS" = "darwin" ] && [ -f "$bash_profile" ]; then
			skillstation_append_marked_block "$bash_profile" "$posix_line"
		fi
	fi

	if [ "$has_zsh" -eq 1 ]; then
		skillstation_append_marked_block "$zshrc" "$posix_line"
		edited_any=1
	fi

	if [ "$has_fish" -eq 1 ]; then
		skillstation_append_marked_block "$fish_rc" "$fish_line"
		edited_any=1
	fi

	if [ "$edited_any" -eq 0 ]; then
		skillstation_append_marked_block "$profile" "$posix_line"
	fi

	primary_source=""
	case "${SHELL:-}" in
		*fish*)
			primary_source="source $fish_rc"
			;;
		*zsh*)
			primary_source="source $zshrc"
			;;
		*bash*)
			if [ "$SKILLSTATION_OS" = "darwin" ] && [ -f "$bash_profile" ]; then
				primary_source="source $bash_profile"
			else
				primary_source="source $bashrc"
			fi
			;;
		*)
			if [ -n "$SKILLSTATION_PRIMARY_MODIFIED" ]; then
				primary_source="source $SKILLSTATION_PRIMARY_MODIFIED"
			else
				primary_source="source $profile"
			fi
			;;
	esac

	printf 'Open a new terminal or run: %s\n' "$primary_source"
}

skillstation_remove_marked_block() {
	file=$1
	[ -f "$file" ] || return 0
	grep -q '# >>> skillstation >>>' "$file" 2>/dev/null || return 0

	if [ "$SKILLSTATION_DRY_RUN" -eq 1 ]; then
		printf 'Would remove skillstation PATH block from %s\n' "$file"
		return 0
	fi

	if [ ! -w "$file" ]; then
		printf 'Notice: %s is read-only; could not remove skillstation PATH block.\n' "$file"
		return 0
	fi

	tmp_file=$file.tmp.$$
	sed '/# >>> skillstation >>>/,/# <<< skillstation <<</d' "$file" > "$tmp_file"
	mv -f "$tmp_file" "$file"

	case "$file" in
		*/fish/conf.d/skillstation.fish)
			if [ ! -s "$file" ]; then
				rm -f "$file"
				printf 'Removed %s\n' "$file"
				return 0
			fi
			;;
	esac
	printf 'Removed skillstation PATH configuration from %s\n' "$file"
}

skillstation_uninstall() {
	skillstation_resolve_install_paths
	SKILLSTATION_CONFIG_DIR=${SKILLSTATION_CONFIG_DIR:-"${XDG_CONFIG_HOME:-$HOME/.config}/skillstation"}

	if [ "$SKILLSTATION_PURGE_CONFIG" -eq 1 ]; then
		case "$SKILLSTATION_CONFIG_DIR" in
			''|/|"$HOME"|"$HOME/") skillstation_error "Refusing unsafe application config path: $SKILLSTATION_CONFIG_DIR" ;;
		esac
		case "/$SKILLSTATION_CONFIG_DIR/" in
			*/../*|*/./*) skillstation_error "Refusing application config path containing dot segments: $SKILLSTATION_CONFIG_DIR" ;;
		esac
		printf 'Config purge preview: %s\n' "$SKILLSTATION_CONFIG_DIR"
		printf '%s\n' 'Canonical workspaces and host configuration will not be removed.'
		if [ "$SKILLSTATION_DRY_RUN" -ne 1 ] && [ "$SKILLSTATION_YES" -ne 1 ]; then
			skillstation_error "Config purge requires --yes after reviewing the preview."
		fi
	elif [ "$SKILLSTATION_YES" -eq 1 ]; then
		skillstation_usage_error "--yes is valid only with --purge-config."
	fi

	managed_target=0
	if [ -e "$SKILLSTATION_TARGET" ] || [ -L "$SKILLSTATION_TARGET" ]; then
		skillstation_is_managed || skillstation_error "Refusing to remove unmanaged path: $SKILLSTATION_TARGET"
		managed_target=1
		if [ "$SKILLSTATION_DRY_RUN" -eq 1 ]; then
			action='Would remove'
		else
			action='Removing'
		fi
		printf '%s %s\n' "$action" "$SKILLSTATION_TARGET"
	elif [ -e "$SKILLSTATION_MARKER" ] || [ -L "$SKILLSTATION_MARKER" ]; then
		skillstation_is_managed || skillstation_error "Invalid installation marker: $SKILLSTATION_MARKER"
		managed_target=1
		printf 'Managed binary is already absent: %s\n' "$SKILLSTATION_TARGET"
	else
		printf 'No managed skillstation installation found at %s\n' "$SKILLSTATION_TARGET"
	fi

	skillstation_remove_marked_block "$HOME/.bashrc"
	skillstation_remove_marked_block "$HOME/.bash_profile"
	skillstation_remove_marked_block "${ZDOTDIR:-$HOME}/.zshrc"
	if [ -n "${ZDOTDIR:-}" ] && [ "$ZDOTDIR" != "$HOME" ]; then
		skillstation_remove_marked_block "$HOME/.zshrc"
	fi
	skillstation_remove_marked_block "${XDG_CONFIG_HOME:-$HOME/.config}/fish/conf.d/skillstation.fish"
	skillstation_remove_marked_block "$HOME/.profile"
	if [ "$SKILLSTATION_DRY_RUN" -eq 1 ]; then
		[ -e "$SKILLSTATION_MARKER" ] && printf 'Would remove %s\n' "$SKILLSTATION_MARKER"
		[ "$SKILLSTATION_PURGE_CONFIG" -eq 1 ] && [ -e "$SKILLSTATION_CONFIG_DIR" ] && printf 'Would remove application config %s\n' "$SKILLSTATION_CONFIG_DIR"
		exit 0
	fi

	if [ "$managed_target" -eq 1 ] || skillstation_is_managed; then
		rm -f "$SKILLSTATION_TARGET" || skillstation_error "Could not remove managed binary: $SKILLSTATION_TARGET"
		rm -f "$SKILLSTATION_MARKER" || skillstation_error "Could not remove installation marker: $SKILLSTATION_MARKER"
	fi

	if [ "$SKILLSTATION_PURGE_CONFIG" -eq 1 ] && [ -e "$SKILLSTATION_CONFIG_DIR" ]; then
		rm -rf "$SKILLSTATION_CONFIG_DIR" || skillstation_error "Could not remove application config: $SKILLSTATION_CONFIG_DIR"
		printf 'Removed application config %s\n' "$SKILLSTATION_CONFIG_DIR"
	fi

	printf '%s\n' 'Uninstall complete. Canonical workspaces and host configuration were preserved.'
	exit 0
}

usage() {
	cat <<'USAGE'
Usage: install.sh [options]

Download, verify, atomically install, upgrade, or uninstall Skillstation.

Options:
  -v, --version VERSION      Release version to install (e.g. 1.0.0 or v1.0.0).
                             Overrides the default version baked into this script.
      --uninstall            Remove managed binary, marker, and installer PATH modifications.
      --purge-config         When uninstalling, also remove application configuration.
  -y, --yes                  Confirm configuration purge without prompt.
      --prefix DIR           Installation prefix (default: $HOME/.local). Binary installed to DIR/bin.
      --install-dir DIR      Install directly into DIR (overrides --prefix).
      --config-dir DIR       Application configuration directory (default: $XDG_CONFIG_HOME/skillstation).
      --no-modify-path       Do not append PATH configuration to shell profile files.
      --require-signature    Fail if cosign is not installed instead of skipping signature check.
      --os OS                Target OS (linux, darwin).
      --arch ARCH            Target architecture (amd64, arm64).
      --release-base URL     Base URL for release downloads.
      --local-fixture DIR    Install from local test fixture directory.
      --dry-run              Print actions without modifying the filesystem.
  -h, --help                 Show this help message.

Re-running install.sh with an existing managed installation automatically upgrades it.
If the installed version matches the target version, the installer is a no-op.
USAGE
}

while [ "$#" -gt 0 ]; do
	case "$1" in
		--help|-h)
			usage
			exit 0
			;;
		--version|-v)
			[ "$#" -ge 2 ] || skillstation_usage_error "Missing value for $1"
			SKILLSTATION_VERSION=$2
			shift 2
			;;
		--version=*)
			SKILLSTATION_VERSION=${1#*=}
			shift
			;;
		--uninstall)
			SKILLSTATION_UNINSTALL=1
			shift
			;;
		--purge-config)
			SKILLSTATION_PURGE_CONFIG=1
			shift
			;;
		--yes|-y)
			SKILLSTATION_YES=1
			shift
			;;
		--no-modify-path)
			SKILLSTATION_NO_MODIFY_PATH=1
			shift
			;;
		--require-signature)
			SKILLSTATION_REQUIRE_SIGNATURE=1
			shift
			;;
		--prefix)
			[ "$#" -ge 2 ] || skillstation_usage_error "Missing value for $1"
			SKILLSTATION_PREFIX=$2
			shift 2
			;;
		--prefix=*)
			SKILLSTATION_PREFIX=${1#*=}
			shift
			;;
		--install-dir)
			[ "$#" -ge 2 ] || skillstation_usage_error "Missing value for $1"
			SKILLSTATION_INSTALL_DIR=$2
			shift 2
			;;
		--install-dir=*)
			SKILLSTATION_INSTALL_DIR=${1#*=}
			shift
			;;
		--config-dir)
			[ "$#" -ge 2 ] || skillstation_usage_error "Missing value for $1"
			SKILLSTATION_CONFIG_DIR=$2
			shift 2
			;;
		--config-dir=*)
			SKILLSTATION_CONFIG_DIR=${1#*=}
			shift
			;;
		--os)
			[ "$#" -ge 2 ] || skillstation_usage_error "Missing value for $1"
			SKILLSTATION_OS=$2
			shift 2
			;;
		--os=*)
			SKILLSTATION_OS=${1#*=}
			shift
			;;
		--arch)
			[ "$#" -ge 2 ] || skillstation_usage_error "Missing value for $1"
			SKILLSTATION_ARCH=$2
			shift 2
			;;
		--arch=*)
			SKILLSTATION_ARCH=${1#*=}
			shift
			;;
		--release-base)
			[ "$#" -ge 2 ] || skillstation_usage_error "Missing value for $1"
			SKILLSTATION_RELEASE_BASE=$2
			shift 2
			;;
		--release-base=*)
			SKILLSTATION_RELEASE_BASE=${1#*=}
			shift
			;;
		--local-fixture)
			[ "$#" -ge 2 ] || skillstation_usage_error "Missing value for $1"
			SKILLSTATION_LOCAL_FIXTURE=$2
			shift 2
			;;
		--local-fixture=*)
			SKILLSTATION_LOCAL_FIXTURE=${1#*=}
			shift
			;;
		--dry-run)
			SKILLSTATION_DRY_RUN=1
			shift
			;;
		--)
			shift
			[ "$#" -eq 0 ] || skillstation_usage_error "Unexpected positional argument: $1"
			break
			;;
		*)
			skillstation_usage_error "Unknown option or argument: $1"
			;;
	esac
done

if [ "$SKILLSTATION_UNINSTALL" -eq 1 ]; then
	skillstation_uninstall
fi

if [ "$SKILLSTATION_PURGE_CONFIG" -eq 1 ] || [ "$SKILLSTATION_YES" -eq 1 ]; then
	skillstation_usage_error "--purge-config and --yes are valid only with --uninstall."
fi

skillstation_validate_version
skillstation_detect_platform
skillstation_validate_source
skillstation_resolve_install_paths

is_upgrade=0
is_same_version=0

if [ -e "$SKILLSTATION_TARGET" ] || [ -L "$SKILLSTATION_TARGET" ]; then
	skillstation_is_managed || skillstation_error "Refusing to overwrite existing unmanaged binary at $SKILLSTATION_TARGET. Only installations managed by skillstation installer (with a valid .skillstation-managed marker) can be upgraded."
	is_upgrade=1
	installed_version=
	if [ -f "$SKILLSTATION_MARKER" ]; then
		installed_version=$(sed -n 's/^version=//p' "$SKILLSTATION_MARKER" | head -n 1)
	fi
	if [ -n "$installed_version" ] && [ "$installed_version" = "$SKILLSTATION_VERSION" ]; then
		is_same_version=1
	fi
elif [ -e "$SKILLSTATION_MARKER" ] || [ -L "$SKILLSTATION_MARKER" ]; then
	skillstation_error "Refusing installation because a marker already exists without a binary: $SKILLSTATION_MARKER"
fi

if [ "$SKILLSTATION_DRY_RUN" -eq 1 ]; then
	if [ "$is_same_version" -eq 1 ]; then
		printf 'Dry run: skillstation %s is already installed at %s\n' "$SKILLSTATION_VERSION" "$SKILLSTATION_TARGET"
	elif [ "$is_upgrade" -eq 1 ]; then
		printf 'Dry run: would upgrade skillstation to %s at %s\n' "$SKILLSTATION_VERSION" "$SKILLSTATION_TARGET"
	else
		printf 'Dry run: would install skillstation %s to %s\n' "$SKILLSTATION_VERSION" "$SKILLSTATION_TARGET"
	fi
	exit 0
fi

if [ "$is_same_version" -eq 1 ]; then
	printf 'skillstation %s is already installed at %s\n' "$SKILLSTATION_VERSION" "$SKILLSTATION_TARGET"
	skillstation_configure_path
	printf 'Next step:\n  mkdir -p ~/skillstation-hub && cd ~/skillstation-hub && skillstation init\n'
	exit 0
fi

skillstation_require_tools
skillstation_make_temp_dir
trap 'skillstation_cleanup_temp' EXIT HUP INT TERM
skillstation_fetch_candidate

mkdir -p "$SKILLSTATION_BIN_DIR" || skillstation_error "Could not create install directory: $SKILLSTATION_BIN_DIR"

if [ "$is_upgrade" -eq 1 ]; then
	install_temp=$SKILLSTATION_BIN_DIR/.skillstation.new.$$
	backup=$SKILLSTATION_BIN_DIR/.skillstation.rollback.$$
	marker_temp=$SKILLSTATION_BIN_DIR/.skillstation-managed.tmp.$$
	marker_backup=$SKILLSTATION_TEMP_DIR/managed-marker.backup
	trap 'rm -f "$install_temp" "$marker_temp"; skillstation_cleanup_temp' EXIT HUP INT TERM

	cp "$SKILLSTATION_CANDIDATE" "$install_temp" || skillstation_error "Could not stage the upgraded binary."
	chmod 755 "$install_temp" || skillstation_error "Could not set executable permissions on the staged binary."
	cp -p "$SKILLSTATION_TARGET" "$backup" || skillstation_error "Could not create rollback copy of the installed binary."
	cp "$SKILLSTATION_MARKER" "$marker_backup" || skillstation_error "Could not preserve the installation marker."
	printf 'skillstation-managed-v1\nversion=%s\n' "$SKILLSTATION_VERSION" > "$marker_temp" || skillstation_error "Could not stage the updated installation marker."
	chmod 600 "$marker_temp" || skillstation_error "Could not protect the updated installation marker."

	mv -f "$install_temp" "$SKILLSTATION_TARGET" || skillstation_error "Could not atomically replace $SKILLSTATION_TARGET"
	if ! "$SKILLSTATION_TARGET" version >/dev/null 2>&1; then
		if mv -f "$backup" "$SKILLSTATION_TARGET" && cp "$marker_backup" "$SKILLSTATION_MARKER"; then
			skillstation_error "Upgraded binary failed its version check; restored the previous installation."
		fi
		skillstation_error "Upgraded binary failed its version check and automatic rollback failed. Recovery copy: $backup"
	fi
	if ! mv -f "$marker_temp" "$SKILLSTATION_MARKER"; then
		if mv -f "$backup" "$SKILLSTATION_TARGET" && cp "$marker_backup" "$SKILLSTATION_MARKER"; then
			skillstation_error "Could not update the installation marker; restored the previous installation."
		fi
		skillstation_error "Could not update the installation marker and automatic rollback failed. Recovery copy: $backup"
	fi
	rm -f "$backup"
	trap 'skillstation_cleanup_temp' EXIT HUP INT TERM
	printf 'Upgraded skillstation to %s at %s\n' "$SKILLSTATION_VERSION" "$SKILLSTATION_TARGET"
else
	install_temp=$SKILLSTATION_BIN_DIR/.skillstation.new.$$
	marker_temp=$SKILLSTATION_BIN_DIR/.skillstation-managed.tmp.$$
	trap 'rm -f "$install_temp" "$marker_temp"; skillstation_cleanup_temp' EXIT HUP INT TERM
	cp "$SKILLSTATION_CANDIDATE" "$install_temp" || skillstation_error "Could not stage the binary in $SKILLSTATION_BIN_DIR"
	chmod 755 "$install_temp" || skillstation_error "Could not set executable permissions on the staged binary."
	printf 'skillstation-managed-v1\nversion=%s\n' "$SKILLSTATION_VERSION" > "$marker_temp" || skillstation_error "Could not stage the installation marker."
	chmod 600 "$marker_temp" || skillstation_error "Could not protect the installation marker."

	mv "$install_temp" "$SKILLSTATION_TARGET" || skillstation_error "Could not atomically install $SKILLSTATION_TARGET"
	if ! mv "$marker_temp" "$SKILLSTATION_MARKER"; then
		rm -f "$SKILLSTATION_TARGET"
		skillstation_error "Could not atomically write $SKILLSTATION_MARKER"
	fi
	trap 'skillstation_cleanup_temp' EXIT HUP INT TERM
	printf 'Installed skillstation %s to %s\n' "$SKILLSTATION_VERSION" "$SKILLSTATION_TARGET"
fi

skillstation_configure_path
printf 'Next step:\n  mkdir -p ~/skillstation-hub && cd ~/skillstation-hub && skillstation init\n'
