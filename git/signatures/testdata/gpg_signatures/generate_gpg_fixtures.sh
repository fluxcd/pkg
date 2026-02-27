#!/usr/bin/env bash
# generate_gpg_fixtures.sh - Script to generate GPG signature test fixtures
# Generates GPG keys in all variants and signed Git objects

set -e

# Configuration variables
TEST_USER_NAME="Test User"
TEST_USER_EMAIL="sign-user@example.com"

# Directory for temporary files
TEMP_DIR=$(mktemp -d)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== GPG Signature Test Fixtures Generator ==="
echo "Temporary directory: $TEMP_DIR"
echo "Output directory: $SCRIPT_DIR"
echo ""

# GPG home directory for test keys
export GNUPGHOME="$TEMP_DIR/gnupg"
mkdir -p "$GNUPGHOME"
chmod 700 "$GNUPGHOME"

# Configure GPG for batch mode (no interaction)
echo "pinentry-mode loopback" > "$GNUPGHOME/gpg.conf"
echo "no-tty" >> "$GNUPGHOME/gpg.conf"

# Function to generate GPG key pair
generate_key() {
    local key_type=$1
    local key_param=$2
    local key_name=$3
    
    echo "Generating $key_type key pair ($key_name)..."
    
    # Create batch configuration for GPG
    local batch_file="$TEMP_DIR/batch_${key_name}.txt"
    cat > "$batch_file" <<EOF
%no-protection
Key-Type: $key_type
EOF
    
    # Add key-specific parameters
    case "$key_type" in
        RSA|DSA)
            echo "Key-Length: $key_param" >> "$batch_file"
            ;;
        ecdsa|eddsa)
            echo "Key-Curve: $key_param" >> "$batch_file"
            ;;
    esac
    
    cat >> "$batch_file" <<EOF
Name-Real: $TEST_USER_NAME
Name-Email: test-${key_name}@example.com
Expire-Date: 0
%commit
EOF
    
    # Generate the key
    gpg --batch --generate-key "$batch_file" 2>&1
    
    # Get the key ID
    local key_id
    key_id=$(gpg --list-keys --with-colons "test-${key_name}@example.com" | grep '^fpr' | head -1 | cut -d: -f10)
    
    echo "  Key ID: $key_id"
    
    # Export public key
    gpg --armor --export "test-${key_name}@example.com" > "$SCRIPT_DIR/key_${key_name}.pub"
    echo "  ✓ key_${key_name}.pub created"
    
    # Export secret key (for signing)
    gpg --armor --export-secret-keys "test-${key_name}@example.com" > "$TEMP_DIR/${key_name}.sec"
    
    # Store key ID for later use
    echo "$key_id" > "$TEMP_DIR/${key_name}_id.txt"
    
    rm -f "$batch_file"
    echo "  ✓ $key_name key pair generated successfully"
}

# Function to create signed Git objects (commits and tags)
create_signed_object() {
    local object_type=$1
    local key_name=$2
    
    echo "Creating signed $object_type for $key_name..."
    
    # Get key ID
    local key_id
    key_id=$(cat "$TEMP_DIR/${key_name}_id.txt")
    
    # Create temporary Git repository
    local repo_dir="$TEMP_DIR/repo_${key_name}_${object_type}"
    mkdir -p "$repo_dir"
    cd "$repo_dir"
    
    git init
    git config user.name "$TEST_USER_NAME"
    git config user.email "$TEST_USER_EMAIL"
    git config gpg.program gpg
    git config user.signingkey "$key_id"
    
    # Import the secret key for signing
    gpg --batch --import "$TEMP_DIR/${key_name}.sec" 2>/dev/null
    
    # Create file and commit
    echo "Test content for $key_name $object_type" > test.txt
    git add test.txt
    git commit -m "Test commit for $object_type"
    
    if [[ "$object_type" == "commit" ]]; then
        # Sign the commit (amend)
        git commit --amend --allow-empty -S -m "Test commit signed with $key_name"
        
        # Verify the signed commit
        echo "  Verifying signed commit..."
        git verify-commit HEAD 2>&1 | grep -q "Good signature"
        echo "  ✓ Commit signature verified successfully"
        
        # Export commit object
        git cat-file commit HEAD > "$SCRIPT_DIR/commit_${key_name}_signed.txt"
        cd "$SCRIPT_DIR"
        echo "  ✓ commit_${key_name}_signed.txt created"

    elif [[ "$object_type" == "tag" ]]; then
        # Create and sign tag
        git tag -a "test-tag-${key_name}" -m "Test tag signed with $key_name" -s
        
        # Verify the signed tag
        echo "  Verifying signed tag..."
        git verify-tag "test-tag-${key_name}" 2>&1 | grep -q "Good signature"
        echo "  ✓ Tag signature verified successfully"
        
        # Export tag object
        git cat-file tag "test-tag-${key_name}" > "$SCRIPT_DIR/tag_${key_name}_signed.txt"
        cd "$SCRIPT_DIR"
        echo "  ✓ tag_${key_name}_signed.txt created"
    fi
}

# Function to create unsigned commit
create_unsigned_commit() {
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
    git cat-file commit HEAD > "$SCRIPT_DIR/commit_unsigned.txt"
    
    cd "$SCRIPT_DIR"
    echo "  ✓ commit_unsigned.txt created"
}

# Main program
main() {
    echo "Step 1: Generate RSA/DSA keys..."
    echo "-----------------------------------"
    
    # RSA keys (different key lengths)
    generate_key "RSA" "2048" "rsa_2048"
    generate_key "RSA" "4096" "rsa_4096"
    
    # DSA key (legacy, but still supported)
    generate_key "DSA" "2048" "dsa_2048"
    
    echo ""
    echo "Step 2: Generate ECC keys..."
    echo "-----------------------------------"
    
    # ECDSA keys (different curves)
    generate_key "ecdsa" "NIST P-256" "ecdsa_p256"
    generate_key "ecdsa" "NIST P-384" "ecdsa_p384"
    generate_key "ecdsa" "NIST P-521" "ecdsa_p521"
    
    # Brainpool curves
    generate_key "ecdsa" "brainpoolP256r1" "brainpool_p256"
    generate_key "ecdsa" "brainpoolP384r1" "brainpool_p384"
    generate_key "ecdsa" "brainpoolP512r1" "brainpool_p512"
    
    # Ed25519 (modern elliptic curve)
    generate_key "eddsa" "Ed25519" "ed25519"
    
    # Ed448 (less common)
    generate_key "eddsa" "Ed448" "ed448"
    
    echo ""
    echo "Step 3: Create signed commits..."
    echo "----------------------------------------"
    
    # Get list of successfully generated keys
    local keys=() key_name=""
    for key_file in "$TEMP_DIR"/*_id.txt; do
        if [[ -f "$key_file" ]]; then
            key_name=$(basename "$key_file" "_id.txt")
            keys+=("$key_name")
        fi
    done
    
    # Signed commits for each key type
    for key_name in "${keys[@]}"; do
        create_signed_object "commit" "$key_name"
    done
    
    echo ""
    echo "Step 4: Create signed tags..."
    echo "-------------------------------------"
    
    # Signed tags for each key type
    for key_name in "${keys[@]}"; do
        create_signed_object "tag" "$key_name"
    done
    
    echo ""
    echo "Step 5: Create unsigned commit..."
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
    find "$SCRIPT_DIR" -maxdepth 1 \( -name "*.txt" -o -name "key_*.pub" \) -exec ls -lh {} \; 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
}

main