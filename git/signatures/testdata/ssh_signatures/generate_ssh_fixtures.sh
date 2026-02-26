#!/usr/bin/env bash
# generate_fixtures.sh - Script to generate SSH signature test fixtures
# Generates SSH keys in all variants and signed Git objects

set -e

# Configuration variables
TEST_USER_NAME="Test User"
TEST_USER_EMAIL="sign-user@example.com"

# Directory for temporary files
TEMP_DIR=$(mktemp -d)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== SSH Signature Test Fixtures Generator ==="
echo "Temporary directory: $TEMP_DIR"
echo "Output directory: $SCRIPT_DIR"
echo ""

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
}

# Function to create authorized_keys files
create_authorized_keys() {
    local key_name=$1
    local output_file="$SCRIPT_DIR/authorized_keys_${key_name}"
    
    echo "Creating authorized_keys for $key_name..."
    
    # Copy public key
    cp "$TEMP_DIR/${key_name}.pub" "$output_file"
    echo "  ✓ $output_file created"
}

# Function to create verified signers files with git namespace
create_verified_signers() {
    local key_name=$1
    local output_file="$SCRIPT_DIR/verified_signers_${key_name}"
    
    echo "Creating verified signers file for $key_name..."
    
    # Create verified signers file with git namespace
    echo "$TEST_USER_EMAIL namespaces=\"git\" $(cat "$TEMP_DIR/${key_name}.pub")" > "$output_file"
    echo "  ✓ $output_file created"
}

# Function to create combined authorized_keys file
create_combined_authorized_keys() {
    local output_file="$SCRIPT_DIR/authorized_keys_all"
    
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

# Function to create combined verified signers file
create_combined_verified_signers() {
    local output_file="$SCRIPT_DIR/verified_signers_all"
    
    echo "Creating combined verified signers..."
    
    # Combine all public keys with git namespace
    {
        echo "$TEST_USER_EMAIL namespaces=\"git\" $(cat "$TEMP_DIR/rsa.pub")"
        echo "$TEST_USER_EMAIL namespaces=\"git\" $(cat "$TEMP_DIR/ecdsa_p256.pub")"
        echo "$TEST_USER_EMAIL namespaces=\"git\" $(cat "$TEMP_DIR/ecdsa_p384.pub")"
        echo "$TEST_USER_EMAIL namespaces=\"git\" $(cat "$TEMP_DIR/ecdsa_p521.pub")"
        echo "$TEST_USER_EMAIL namespaces=\"git\" $(cat "$TEMP_DIR/ed25519.pub")"
    } > "$output_file"
    
    echo "  ✓ $output_file created"
}

# Function to create signed Git objects (commits and tags)
create_signed_object() {
    local object_type=$1
    local key_name=$2
    local key_type=$3
    local verified_signers_file="$SCRIPT_DIR/verified_signers_${key_name}"
    
    echo "Creating signed $object_type for $key_name..."
    
    # Create temporary Git repository
    local repo_dir="$TEMP_DIR/repo_${key_name}_${object_type}"
    mkdir -p "$repo_dir"
    cd "$repo_dir"
    
    git init
    git config user.name "$TEST_USER_NAME"
    git config user.email "$TEST_USER_EMAIL"
    git config gpg.format ssh
    git config user.signingkey "$TEMP_DIR/${key_name}.pub"
    git config gpg.ssh.allowedSignersFile "$verified_signers_file"
    
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

# Function to create unsigned commit
create_unsigned_commit() {
    local commit_file="$SCRIPT_DIR/commit_unsigned.txt"
    
    echo "Creating unsigned commit..."
    
    # Create temporary Git repository
    local repo_dir="$TEMP_DIR/repo_unsigned"
    mkdir -p "$repo_dir"
    cd "$repo_dir"
    
    git init
    git config user.name "$TEST_USER_NAME"
    git config user.email "$TEST_USER_EMAIL"
    
    # Create file and commit (without signature)
    echo "Test content unsigned" > test.txt
    git add test.txt
    git commit -m "Test commit unsigned"
    
    # Export commit object
    git cat-file commit HEAD > "$commit_file"
    
    cd "$SCRIPT_DIR"
    echo "  ✓ $commit_file created"
}

# Main program
main() {
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
    echo "Step 2: Create authorized_keys files..."
    echo "-----------------------------------------------"
    
    # Individual authorized_keys files
    create_authorized_keys "rsa"
    create_authorized_keys "ecdsa_p256"
    create_authorized_keys "ecdsa_p384"
    create_authorized_keys "ecdsa_p521"
    create_authorized_keys "ed25519"
    
    # Combined authorized_keys file
    create_combined_authorized_keys
    
    echo ""
    echo "Step 3: Create verified signers files..."
    echo "-----------------------------------------------"
    
    # Individual verified signers files with git namespace
    create_verified_signers "rsa"
    create_verified_signers "ecdsa_p256"
    create_verified_signers "ecdsa_p384"
    create_verified_signers "ecdsa_p521"
    create_verified_signers "ed25519"
    
    # Combined verified signers file
    create_combined_verified_signers
    
    echo ""
    echo "Step 4: Create signed commits..."
    echo "----------------------------------------"
    
    # Signed commits for each key type
    create_signed_object "commit" "rsa" "rsa"
    create_signed_object "commit" "ecdsa_p256" "ecdsa"
    create_signed_object "commit" "ecdsa_p384" "ecdsa"
    create_signed_object "commit" "ecdsa_p521" "ecdsa"
    create_signed_object "commit" "ed25519" "ed25519"
    
    echo ""
    echo "Step 5: Create signed tags..."
    echo "-------------------------------------"
    
    # Signed tags for each key type
    create_signed_object "tag" "rsa" "rsa"
    create_signed_object "tag" "ecdsa_p256" "ecdsa"
    create_signed_object "tag" "ecdsa_p384" "ecdsa"
    create_signed_object "tag" "ecdsa_p521" "ecdsa"
    create_signed_object "tag" "ed25519" "ed25519"
    
    echo ""
    echo "Step 6: Create unsigned commit..."
    echo "------------------------------------------"
    
    create_unsigned_commit
    
    echo ""
    echo "=== Cleanup ==="
    rm -rf "$TEMP_DIR"
    echo "Temporary directory removed"
    
    echo ""
    echo "=== Done! ==="
    echo "All test fixtures have been successfully created."
    echo ""
    echo "Created files:"
    find "$SCRIPT_DIR" -maxdepth 1 \( -name "*.txt" -o -name "key_*.pub" -o -name "authorized_keys*" -o -name "verified_signers*" \) -exec ls -lh {} \; 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
}

# Run script
main "$@"