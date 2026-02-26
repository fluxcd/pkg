# SSH Signature Test Fixtures

This directory contains test fixtures for SSH signature validation.

## Quick Start

To generate all test fixtures at once, simply run:

```bash
./generate_ssh_fixtures.sh
```

This script will automatically create all SSH keys, authorized_keys files, verified signers files, signed commits, and signed tags.

## How to Generate Test Fixtures

### Using the Automated Script

The [`generate_ssh_fixtures.sh`](generate_ssh_fixtures.sh) script automates the entire process of creating SSH signature test fixtures. It generates:

1. **SSH Key Pairs** in all variants:
   - RSA (4096 bits)
   - ECDSA (p256, p384, p521)
   - ED25519

2. **Authorized Keys Files**:
   - Individual files for each key type
   - Combined file with all keys

3. **Verified Signers Files** (with git namespace):
   - Individual files for each key type
   - Combined file with all keys

4. **Signed Git Commits**:
   - One signed commit for each key type
   - All commits are verified using `git verify-commit`

5. **Signed Git Tags**:
   - One signed tag for each key type
   - All tags are verified using `git verify-tag`

6. **Unsigned Commit**:
   - One unsigned commit for testing negative cases

### Manual Generation

If you need to generate test fixtures manually, follow these steps:

#### 1. Generate SSH Key Pairs

```bash
# RSA key
ssh-keygen -t rsa -b 4096 -f test_rsa -N ""
mv test_rsa.pub key_rsa.pub

# ECDSA keys (all variants)
ssh-keygen -t ecdsa -b 256 -f test_ecdsa_p256 -N ""
mv test_ecdsa_p256.pub key_ecdsa_p256.pub

ssh-keygen -t ecdsa -b 384 -f test_ecdsa_p384 -N ""
mv test_ecdsa_p384.pub key_ecdsa_p384.pub

ssh-keygen -t ecdsa -b 521 -f test_ecdsa_p521 -N ""
mv test_ecdsa_p521.pub key_ecdsa_p521.pub

# ED25519 key
ssh-keygen -t ed25519 -f test_ed25519 -N ""
mv test_ed25519.pub key_ed25519.pub
```

#### 2. Create Verified Signers File

```bash
# Create verified signers file with git namespace
echo "$(git config --get user.email) namespaces=\"git\" $(cat key_ed25519.pub)" > verified_signers_ed25519
```

#### 3. Create a Test Git Repository

```bash
mkdir test_repo && cd test_repo
git init
echo "test content" > test.txt
git add test.txt
git commit -m "Test commit"
git config user.name "Test User"
git config user.email "sign-user@example.com"
git config gpg.format ssh
git config user.signingkey ../key_ed25519.pub
git config gpg.ssh.allowedSignersFile ../verified_signers_ed25519
```

#### 4. Sign a Commit with SSH

```bash
# Sign the last commit
git commit --amend --allow-empty -S -m "Test commit signed with ed25519"

# Verify the signed commit
git verify-commit HEAD
```

#### 5. Export the Signed Commit

```bash
# Get the commit object
git cat-file commit HEAD > commit_ed25519_signed.txt
```

#### 6. Create a Tag and Sign It

```bash
git tag -a test-tag -m "Test tag" -s
git verify-tag test-tag
git cat-file tag test-tag > tag_ed25519_signed.txt
```

## File Format

The signed Git objects follow the standard Git object format with SSH signatures:

### Signed Commit Format

```
tree <tree-hash>
parent <parent-hash>
author <name> <email> <timestamp> <timezone>
committer <name> <email> <timestamp> <timezone>
gpgsig -----BEGIN SSH SIGNATURE-----
 <signature data>
 -----END SSH SIGNATURE-----

<commit message>
```

### Signed Tag Format

```
object <commit-hash>
type commit
tag <tag-name>
tagger <name> <email> <timestamp> <timezone>

<tag message>
-----BEGIN SSH SIGNATURE-----
 <signature data>
-----END SSH SIGNATURE-----
```

### Verified Signers Format

```
<email> namespaces="git" <ssh-public-key>
```

## Generated Files

The script generates the following files:

### Public Keys
- `key_rsa.pub` - RSA 4096-bit public key
- `key_ecdsa_p256.pub` - ECDSA P-256 public key
- `key_ecdsa_p384.pub` - ECDSA P-384 public key
- `key_ecdsa_p521.pub` - ECDSA P-521 public key
- `key_ed25519.pub` - ED25519 public key

### Authorized Keys Files
- `authorized_keys_rsa` - RSA public key
- `authorized_keys_ecdsa_p256` - ECDSA P-256 public key
- `authorized_keys_ecdsa_p384` - ECDSA P-384 public key
- `authorized_keys_ecdsa_p521` - ECDSA P-521 public key
- `authorized_keys_ed25519` - ED25519 public key
- `authorized_keys_all` - All public keys combined

### Verified Signers Files
- `verified_signers_rsa` - RSA public key with git namespace
- `verified_signers_ecdsa_p256` - ECDSA P-256 public key with git namespace
- `verified_signers_ecdsa_p384` - ECDSA P-384 public key with git namespace
- `verified_signers_ecdsa_p521` - ECDSA P-521 public key with git namespace
- `verified_signers_ed25519` - ED25519 public key with git namespace
- `verified_signers_all` - All public keys with git namespace

### Signed Commits
- `commit_rsa_signed.txt` - RSA-signed commit
- `commit_ecdsa_p256_signed.txt` - ECDSA P-256 signed commit
- `commit_ecdsa_p384_signed.txt` - ECDSA P-384 signed commit
- `commit_ecdsa_p521_signed.txt` - ECDSA P-521 signed commit
- `commit_ed25519_signed.txt` - ED25519 signed commit

### Signed Tags
- `tag_rsa_signed.txt` - RSA-signed tag
- `tag_ecdsa_p256_signed.txt` - ECDSA P-256 signed tag
- `tag_ecdsa_p384_signed.txt` - ECDSA P-384 signed tag
- `tag_ecdsa_p521_signed.txt` - ECDSA P-521 signed tag
- `tag_ed25519_signed.txt` - ED25519 signed tag

### Unsigned Commit
- `commit_unsigned.txt` - Unsigned commit for testing negative cases

## Security Note

These test fixtures use generated test keys and should NOT be used in production.