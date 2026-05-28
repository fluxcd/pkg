#!/usr/bin/env bash
# generate_fixtures.sh - Script to generate SSH signature test fixtures
# Generates SSH keys in all variants and signed Git objects

set -euo pipefail

# Configuration variables
TEST_USER_NAME="Test User"
TEST_USER_EMAIL="sign-user@example.com"
FIXTURE_DATE="2026-01-01T00:00:00+0000"

# Isolate Git from user and system configuration for deterministic output
export TZ=UTC
export GIT_AUTHOR_DATE="$FIXTURE_DATE"
export GIT_COMMITTER_DATE="$FIXTURE_DATE"
export GIT_AUTHOR_NAME="$TEST_USER_NAME"
export GIT_AUTHOR_EMAIL="$TEST_USER_EMAIL"
export GIT_COMMITTER_NAME="$TEST_USER_NAME"
export GIT_COMMITTER_EMAIL="$TEST_USER_EMAIL"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null

# Where the generated fixtures will ultimately land. Used only as the
# target of the final atomic swap; nothing is written here directly.
DEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Generation goes into a fresh staging directory ($SCRIPT_DIR). A failed
# run therefore leaves the existing fixtures in $DEST_DIR untouched, and
# a successful run swaps the whole new set into place in one step,
# implicitly removing any fixtures whose name is no longer produced.
TEMP_DIR=$(mktemp -d)
SCRIPT_DIR=$(mktemp -d)

# Load-bearing external tools. Other commands (mkdir, rm, cp, ...) are
# assumed to be present as part of any sane POSIX environment.
DEPENDENCY=(
    ssh-keygen
    git
)

echo "=== SSH Signature Test Fixtures Generator ==="
echo "Staging directory:  $SCRIPT_DIR"
echo "Temporary directory: $TEMP_DIR"
echo "Output directory:    $DEST_DIR"
echo ""

# cleanup on exit, including on Ctrl-C or kill. Removes both the staging
# and the gpg/ssh scratch directory regardless of whether the swap ran.
cleanup() {
    for d in "${SCRIPT_DIR}" "${TEMP_DIR}"; do
        if [[ -d "$d" ]]; then
            rm -rf "$d"
        fi
    done
}

# swap_into_dest atomically replaces the generated fixtures in $DEST_DIR
# with the freshly generated set in $SCRIPT_DIR. Everything except the
# script itself, the README, and any dotfile is removed; the new set is
# then copied in from staging. This is intentionally driven by what was
# *just produced* rather than by an explicit allow-list of artifact name
# patterns, so adding a new key type to the script no longer requires
# remembering to update a deletion list.
swap_into_dest() {
    find "$DEST_DIR" -mindepth 1 -maxdepth 1 -type f \
        ! -name "$(basename "${BASH_SOURCE[0]}")" \
        ! -name 'README.md' \
        ! -name '.*' \
        -delete
    cp "$SCRIPT_DIR"/* "$DEST_DIR"/
}

# check necessary commands
check_dependencies() {
    local exit_state=0

    # check presence of dependencies
    for COMMAND in "${DEPENDENCY[@]}"; do
        if ! command -v "${COMMAND}" >/dev/null 2>&1; then
            echo "command '${COMMAND}' not found, needs to be installed first."
            exit_state=1
        fi
    done

    if [[ ${exit_state} -ne 0 ]]; then
        exit 1
    fi
}

# Function to generate SSH keys
generate_ssh_key() {
    local key_type=$1
    local key_bits=$2
    local key_name=$3

    echo "Generating $key_name key pair..."

    case "$key_type" in
        rsa)
            ssh-keygen -t rsa -b "$key_bits" -f "$TEMP_DIR/$key_name" -N "" -C "test-$key_name@example.com"
            ;;
        ecdsa)
            ssh-keygen -t ecdsa -b "$key_bits" -f "$TEMP_DIR/$key_name" -N "" -C "test-$key_name@example.com"
            ;;
        ed25519)
            ssh-keygen -t ed25519 -f "$TEMP_DIR/$key_name" -N "" -C "test-$key_name@example.com"
            ;;
    esac

    # Copy public key to output directory with key_ prefix
    cp "$TEMP_DIR/$key_name.pub" "$SCRIPT_DIR/key_${key_name}.pub"
    echo "  ✓ key_${key_name}.pub created"

    # Calculate and write SHA256 fingerprint to file
    ssh-keygen -lf "$TEMP_DIR/$key_name.pub" | awk '{print $2}' > "$SCRIPT_DIR/key_${key_name}.pub_fingerprint"
    echo "  ✓ key_${key_name}.pub_fingerprint created"
}

# Function to create the allowed signers file used by `git verify-commit`
# during fixture generation. These files live in $TEMP_DIR only -- they
# are not shipped alongside the test data because the Go test suite does
# not consume them.
create_temp_allowed_signers() {
    local key_name=$1
    local output_file="$TEMP_DIR/allowed_signers_${key_name}"

    echo "Creating temporary allowed signers file for $key_name..."

    echo "$TEST_USER_EMAIL namespaces=\"git\" $(cat "$TEMP_DIR/${key_name}.pub")" > "$output_file"
    echo "  ✓ $output_file created"
}

# Function to create combined authorized_keys file
create_combined_pub_keys() {
    local output_file="$SCRIPT_DIR/keys_all.pub"

    echo "Creating combined authorized_keys..."

    # Combine all public keys
    {
        cat "$TEMP_DIR/rsa.pub"
        cat "$TEMP_DIR/ecdsa_p256.pub"
        cat "$TEMP_DIR/ecdsa_p384.pub"
        cat "$TEMP_DIR/ecdsa_p521.pub"
        cat "$TEMP_DIR/ed25519.pub"
    } > "$output_file"

    echo "  ✓ $output_file created"
}


# Function to create signed Git objects (commits and tags)
create_signed_object() {
    local object_type=$1
    local key_name=$2
    local allowed_signers_file="$TEMP_DIR/allowed_signers_${key_name}"

    echo "Creating signed $object_type for $key_name..."

    # Create temporary Git repository
    local repo_dir="$TEMP_DIR/repo_${key_name}_${object_type}"
    mkdir -p "$repo_dir"
    cd "$repo_dir"

    git init -b main
    git config gpg.format ssh
    git config user.signingkey "$TEMP_DIR/${key_name}.pub"
    git config gpg.ssh.allowedSignersFile "$allowed_signers_file"

    # Create file and commit
    echo "Test content for $key_name $object_type" > test.txt
    git add test.txt
    git commit -m "Test commit for $object_type"

    if [[ "$object_type" == "commit" ]]; then
        # Sign the commit (amend)
        git commit --amend --allow-empty -S -m "Test commit signed with $key_name"

        # Verify the signed commit using git verify-commit
        echo "  Verifying signed commit with git verify-commit..."
        if git verify-commit HEAD; then
            echo "  ✓ Commit signature verified successfully"
        else
            echo "  ✗ Commit signature verification failed"
            exit 1
        fi

        # Export commit object
        local output_file="$SCRIPT_DIR/commit_${key_name}_signed.txt"
        git cat-file commit HEAD > "$output_file"
        cd "$SCRIPT_DIR"
        echo "  ✓ $output_file created"

    elif [[ "$object_type" == "tag" ]]; then
        # Create and sign tag
        git tag -a "test-tag-${key_name}" -m "Test tag signed with $key_name" -s

        # Verify the signed tag using git verify-tag
        echo "  Verifying signed tag with git verify-tag..."
        if git verify-tag "test-tag-${key_name}"; then
            echo "  ✓ Tag signature verified successfully"
        else
            echo "  ✗ Tag signature verification failed"
            exit 1
        fi

        # Export tag object
        local output_file="$SCRIPT_DIR/tag_${key_name}_signed.txt"
        git cat-file tag "test-tag-${key_name}" > "$output_file"
        cd "$SCRIPT_DIR"
        echo "  ✓ $output_file created"
    else
        echo "Error: unknown object type: ${object_type}"
    fi
}

# Function to create unsigned commit and tag
create_unsigned_commit_and_tag() {
    local commit_file="$SCRIPT_DIR/commit_unsigned.txt"
    local tag_file="$SCRIPT_DIR/tag_unsigned.txt"

    echo "Creating unsigned commit and tag..."

    # Create temporary Git repository
    local repo_dir="$TEMP_DIR/repo_unsigned"
    mkdir -p "$repo_dir"
    cd "$repo_dir"

    git init -b main

    # Create file and commit (without signature)
    echo "Test content unsigned" > test.txt
    git add test.txt
    git commit -m "Test commit unsigned"

    # Export commit object
    git cat-file commit HEAD > "$commit_file"

    # Create and export tag object
    git tag -a test-tag -m "Test tag"
    git cat-file tag test-tag > "$tag_file"

    cd "$SCRIPT_DIR"
    echo "  ✓ $commit_file and $tag_file created"
}

# Main program
main() {

    check_dependencies

    echo "Step 1: Generate SSH keys..."
    echo "-----------------------------------"

    # RSA key (4096 bits)
    generate_ssh_key "rsa" "4096" "rsa"

    # ECDSA keys (all variants: p256, p384, p521)
    generate_ssh_key "ecdsa" "256" "ecdsa_p256"
    generate_ssh_key "ecdsa" "384" "ecdsa_p384"
    generate_ssh_key "ecdsa" "521" "ecdsa_p521"

    # ED25519 key
    generate_ssh_key "ed25519" "" "ed25519"

    echo ""
    echo "Step 2: Create pub_keys files..."
    echo "-----------------------------------------------"

    # Combined pub_keys file
    create_combined_pub_keys

    echo ""
    echo "Step 3: Create temporary allowed signers files..."
    echo "-----------------------------------------------"

    # Individual allowed_signers files with git namespace, written to
    # $TEMP_DIR for `git verify-commit`'s use during fixture creation only.
    create_temp_allowed_signers "rsa"
    create_temp_allowed_signers "ecdsa_p256"
    create_temp_allowed_signers "ecdsa_p384"
    create_temp_allowed_signers "ecdsa_p521"
    create_temp_allowed_signers "ed25519"

    echo ""
    echo "Step 4: Create signed commits..."
    echo "----------------------------------------"

    # Signed commits for each key type
    create_signed_object "commit" "rsa"
    create_signed_object "commit" "ecdsa_p256"
    create_signed_object "commit" "ecdsa_p384"
    create_signed_object "commit" "ecdsa_p521"
    create_signed_object "commit" "ed25519"

    echo ""
    echo "Step 5: Create signed tags..."
    echo "-------------------------------------"

    # Signed tags for each key type
    create_signed_object "tag" "rsa"
    create_signed_object "tag" "ecdsa_p256"
    create_signed_object "tag" "ecdsa_p384"
    create_signed_object "tag" "ecdsa_p521"
    create_signed_object "tag" "ed25519"

    echo ""
    echo "Step 6: Create unsigned commit and tag..."
    echo "------------------------------------------"

    create_unsigned_commit_and_tag

    echo ""
    echo "Step 7: Atomically swap staging into output directory..."
    echo "----------------------------------------------------------"
    swap_into_dest

    echo ""
    echo "=== Done! ==="
    echo "All test fixtures have been successfully created in $DEST_DIR."
    echo ""
    echo "Created files:"
    find "$DEST_DIR" -mindepth 1 -maxdepth 1 -type f \
        ! -name "$(basename "${BASH_SOURCE[0]}")" \
        ! -name 'README.md' \
        ! -name '.*' \
        -exec ls -lh {} \; 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}' | sort
}

trap cleanup EXIT INT TERM

# Run script
main
