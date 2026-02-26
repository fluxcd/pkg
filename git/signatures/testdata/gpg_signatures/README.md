# GPG Signature Test Fixtures

This directory contains test fixtures for GPG signature validation.

## Quick Start

To generate all test fixtures at once, simply run:

```bash
./generate_gpg_fixtures.sh
```

This script will automatically create all GPG keys, signed commits, and signed tags.

## How to Generate Test Fixtures

### Using the Automated Script

The [`generate_gpg_fixtures.sh`](generate_gpg_fixtures.sh) script automates the entire process of creating GPG signature test fixtures. It generates:

1. **GPG Key Pairs** in supported variants:
   - RSA (2048 and 4096 bits)
   - DSA (2048 bits)
   - ECC/ECDSA (NIST P-256, P-384, P-521)
   - Brainpool curves (P-256, P-384, P-512)
   - EdDSA (Ed25519, Ed448)

   **Note:** Some key types (like Ed448) require GnuPG 2.3 or higher. The script will report any failures and continue with successfully generated keys.

2. **Public Keys**:
   - Individual public key files for each key type

3. **Signed Git Commits**:
   - One signed commit for each key type
   - All commits are verified using `git verify-commit`

4. **Signed Git Tags**:
   - One signed tag for each key type
   - All tags are verified using `git verify-tag`

5. **Unsigned Commit**:
   - One unsigned commit for testing negative cases

### Manual Generation

If you need to generate test fixtures manually, follow these steps:

#### 1. Generate GPG Key Pairs

```bash
# Set up a temporary GPG home directory
export GNUPGHOME=$(mktemp -d)
mkdir -p "$GNUPGHOME"
chmod 700 "$GNUPGHOME"

# Configure GPG for batch mode
echo "pinentry-mode loopback" > "$GNUPGHOME/gpg.conf"
echo "no-tty" >> "$GNUPGHOME/gpg.conf"

# RSA 2048-bit key
cat > batch_rsa_2048.txt <<EOF
%no-protection
Key-Type: RSA
Key-Length: 2048
Name-Real: Test User
Name-Email: test-rsa-2048@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_rsa_2048.txt

# RSA 4096-bit key
cat > batch_rsa_4096.txt <<EOF
%no-protection
Key-Type: RSA
Key-Length: 4096
Name-Real: Test User
Name-Email: test-rsa-4096@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_rsa_4096.txt

# DSA 2048-bit key
cat > batch_dsa_2048.txt <<EOF
%no-protection
Key-Type: DSA
Key-Length: 2048
Name-Real: Test User
Name-Email: test-dsa-2048@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_dsa_2048.txt

# ECDSA P-256 key
cat > batch_ecdsa_p256.txt <<EOF
%no-protection
Key-Type: ECC
Key-Curve: NIST P-256
Name-Real: Test User
Name-Email: test-ecdsa-p256@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_ecdsa_p256.txt

# ECDSA P-384 key
cat > batch_ecdsa_p384.txt <<EOF
%no-protection
Key-Type: ECC
Key-Curve: NIST P-384
Name-Real: Test User
Name-Email: test-ecdsa-p384@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_ecdsa_p384.txt

# ECDSA P-521 key
cat > batch_ecdsa_p521.txt <<EOF
%no-protection
Key-Type: ECC
Key-Curve: NIST P-521
Name-Real: Test User
Name-Email: test-ecdsa-p521@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_ecdsa_p521.txt

# Brainpool P-256 key
cat > batch_brainpool_p256.txt <<EOF
%no-protection
Key-Type: ECC
Key-Curve: brainpoolP256r1
Name-Real: Test User
Name-Email: test-brainpool-p256@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_brainpool_p256.txt

# Brainpool P-384 key
cat > batch_brainpool_p384.txt <<EOF
%no-protection
Key-Type: ECC
Key-Curve: brainpoolP384r1
Name-Real: Test User
Name-Email: test-brainpool-p384@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_brainpool_p384.txt

# Brainpool P-512 key
cat > batch_brainpool_p512.txt <<EOF
%no-protection
Key-Type: ECC
Key-Curve: brainpoolP512r1
Name-Real: Test User
Name-Email: test-brainpool-p512@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_brainpool_p512.txt

# Ed25519 key
cat > batch_ed25519.txt <<EOF
%no-protection
Key-Type: ECC
Key-Curve: Ed25519
Name-Real: Test User
Name-Email: test-ed25519@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_ed25519.txt

# Ed448 key
cat > batch_ed448.txt <<EOF
%no-protection
Key-Type: ECC
Key-Curve: Ed448
Name-Real: Test User
Name-Email: test-ed448@example.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key batch_ed448.txt

# Export public keys
gpg --armor --export test-rsa-2048@example.com > key_rsa_2048.pub
gpg --armor --export test-rsa-4096@example.com > key_rsa_4096.pub
gpg --armor --export test-dsa-2048@example.com > key_dsa_2048.pub
gpg --armor --export test-ecdsa-p256@example.com > key_ecdsa_p256.pub
gpg --armor --export test-ecdsa-p384@example.com > key_ecdsa_p384.pub
gpg --armor --export test-ecdsa-p521@example.com > key_ecdsa_p521.pub
gpg --armor --export test-brainpool-p256@example.com > key_brainpool_p256.pub
gpg --armor --export test-brainpool-p384@example.com > key_brainpool_p384.pub
gpg --armor --export test-brainpool-p512@example.com > key_brainpool_p512.pub
gpg --armor --export test-ed25519@example.com > key_ed25519.pub
gpg --armor --export test-ed448@example.com > key_ed448.pub
```

#### 2. Create a Test Git Repository

```bash
mkdir test_repo && cd test_repo
git init
echo "test content" > test.txt
git add test.txt
git commit -m "Test commit"
git config user.name "Test User"
git config user.email "sign-user@example.com"
git config gpg.program gpg

# Get the key ID for the key you want to use
KEY_ID=$(gpg --list-keys --with-colons test-ed25519@example.com | grep '^fpr' | head -1 | cut -d: -f10)
git config user.signingkey "$KEY_ID"
```

#### 3. Sign a Commit with GPG

```bash
# Sign the last commit
git commit --amend --allow-empty -S -m "Test commit signed with ed25519"

# Verify the signed commit
git verify-commit HEAD
```

#### 4. Export the Signed Commit

```bash
# Get the commit object
git cat-file commit HEAD > commit_ed25519_signed.txt
```

#### 5. Create a Tag and Sign It

```bash
git tag -a test-tag -m "Test tag" -s
git verify-tag test-tag
git cat-file tag test-tag > tag_ed25519_signed.txt
```

## File Format

The signed Git objects follow the standard Git object format with GPG signatures:

### Signed Commit Format

```
tree <tree-hash>
parent <parent-hash>
author <name> <email> <timestamp> <timezone>
committer <name> <email> <timestamp> <timezone>
gpgsig -----BEGIN PGP SIGNATURE-----
 <signature data>
 -----END PGP SIGNATURE-----

<commit message>
```

### Signed Tag Format

```
object <commit-hash>
type commit
tag <tag-name>
tagger <name> <email> <timestamp> <timezone>

<tag message>
-----BEGIN PGP SIGNATURE-----
 <signature data>
 -----END PGP SIGNATURE-----
```

## Generated Files

The script generates the following files:

### Public Keys
- `key_rsa_2048.pub` - RSA 2048-bit public key
- `key_rsa_4096.pub` - RSA 4096-bit public key
- `key_dsa_2048.pub` - DSA 2048-bit public key
- `key_ecdsa_p256.pub` - ECDSA P-256 public key
- `key_ecdsa_p384.pub` - ECDSA P-384 public key
- `key_ecdsa_p521.pub` - ECDSA P-521 public key
- `key_brainpool_p256.pub` - Brainpool P-256 public key
- `key_brainpool_p384.pub` - Brainpool P-384 public key
- `key_brainpool_p512.pub` - Brainpool P-512 public key
- `key_ed25519.pub` - Ed25519 public key
- `key_ed448.pub` - Ed448 public key

### Signed Commits
- `commit_rsa_2048_signed.txt` - RSA 2048-bit signed commit
- `commit_rsa_4096_signed.txt` - RSA 4096-bit signed commit
- `commit_dsa_2048_signed.txt` - DSA 2048-bit signed commit
- `commit_ecdsa_p256_signed.txt` - ECDSA P-256 signed commit
- `commit_ecdsa_p384_signed.txt` - ECDSA P-384 signed commit
- `commit_ecdsa_p521_signed.txt` - ECDSA P-521 signed commit
- `commit_brainpool_p256_signed.txt` - Brainpool P-256 signed commit
- `commit_brainpool_p384_signed.txt` - Brainpool P-384 signed commit
- `commit_brainpool_p512_signed.txt` - Brainpool P-512 signed commit
- `commit_ed25519_signed.txt` - Ed25519 signed commit
- `commit_ed448_signed.txt` - Ed448 signed commit

### Signed Tags
- `tag_rsa_2048_signed.txt` - RSA 2048-bit signed tag
- `tag_rsa_4096_signed.txt` - RSA 4096-bit signed tag
- `tag_dsa_2048_signed.txt` - DSA 2048-bit signed tag
- `tag_ecdsa_p256_signed.txt` - ECDSA P-256 signed tag
- `tag_ecdsa_p384_signed.txt` - ECDSA P-384 signed tag
- `tag_ecdsa_p521_signed.txt` - ECDSA P-521 signed tag
- `tag_brainpool_p256_signed.txt` - Brainpool P-256 signed tag
- `tag_brainpool_p384_signed.txt` - Brainpool P-384 signed tag
- `tag_brainpool_p512_signed.txt` - Brainpool P-512 signed tag
- `tag_ed25519_signed.txt` - Ed25519 signed tag
- `tag_ed448_signed.txt` - Ed448 signed tag

### Unsigned Commit
- `commit_unsigned.txt` - Unsigned commit for testing negative cases

## Key Types Explained

### RSA (Rivest-Shamir-Adleman)
- **RSA 2048**: Standard RSA key with 2048-bit modulus
- **RSA 4096**: Stronger RSA key with 4096-bit modulus
- Widely supported, but slower than ECC keys

### DSA (Digital Signature Algorithm)
- **DSA 2048**: Legacy algorithm, 2048-bit key
- Less secure than modern alternatives, included for compatibility testing

### ECDSA (Elliptic Curve Digital Signature Algorithm)
- **P-256**: NIST P-256 curve (secp256r1)
- **P-384**: NIST P-384 curve (secp384r1)
- **P-521**: NIST P-521 curve (secp521r1)
- Efficient and secure, widely supported

### Brainpool Curves
- **P-256**: brainpoolP256r1 curve
- **P-384**: brainpoolP384r1 curve
- **P-512**: brainpoolP512r1 curve
- Alternative to NIST curves with different security properties

### EdDSA (Edwards-curve Digital Signature Algorithm)
- **Ed25519**: Modern, fast, and secure curve
- **Ed448**: Higher security variant
- Recommended for new applications

## Security Note

These test fixtures use generated test keys and should NOT be used in production. The keys are created without passphrases for testing purposes only.

## Requirements

- GnuPG (gpg) version 2.0 or higher
- Git with GPG support
- Bash shell

## Troubleshooting

### GPG version compatibility
Some key types (like Ed448) require GnuPG 2.3 or higher. If you encounter errors, check your GPG version:

```bash
gpg --version
```

### Key generation failures
The script now includes comprehensive error handling:
- Each key generation attempt is logged
- Failed keys are reported with detailed error messages
- The script continues with successfully generated keys
- An error log is created in the temporary directory

If key generation fails, ensure that:
1. You have sufficient entropy on your system
2. The GPG home directory has proper permissions (700)
3. No other GPG agents are interfering
4. Your GPG version supports the requested key type

### Script structure
The script uses separate functions for different key types:
- `generate_rsa_dsa_key()` - For RSA and DSA keys with key length validation
- `generate_ecc_key()` - For ECC/ECDSA/EdDSA keys with curve validation
- `create_signed_object()` - For creating signed commits and tags
- `create_unsigned_commit()` - For creating unsigned test commits

Each function includes parameter validation and proper error handling.

### Signature verification failures
If signature verification fails, ensure that:
1. The public key is properly imported
2. The GPG trust database is configured correctly
3. The signature was created with the corresponding private key