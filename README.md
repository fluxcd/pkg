# pkg

[![godev](https://img.shields.io/static/v1?label=godev&message=reference&color=00add8)](https://pkg.go.dev/github.com/fluxcd/pkg)
[![build](https://github.com/fluxcd/pkg/workflows/build/badge.svg)](https://github.com/fluxcd/pkg/actions)

## GitOps Toolkit Go SDK

### APIs
- **[github.com/fluxcd/pkg/apis/acl](./apis/acl)** - API types for defining access control lists
- **[github.com/fluxcd/pkg/apis/event](./apis/event)** - API Schema definitions for Flux eventing  
- **[github.com/fluxcd/pkg/apis/kustomize](./apis/kustomize)** - API types for Kustomize resources
- **[github.com/fluxcd/pkg/apis/meta](./apis/meta)** - Generic metadata APIs for Kubernetes resources

### Authentication & Security
- **[github.com/fluxcd/pkg/auth](./auth)** - OIDC-based authentication with cloud providers (AWS, Azure, GCP)
- **[github.com/fluxcd/pkg/masktoken](./masktoken)** - Token redaction utilities for secure logging
- **[github.com/fluxcd/pkg/ssh](./ssh)** - SSH host key scanning and management

### Artifact Storage & Serving

- **[github.com/fluxcd/pkg/artifact](./artifact)** - Artifact Management SDK
- **[github.com/fluxcd/pkg/artifact/config](./artifact/config)** - Configuration management of artifact storage and serving
- **[github.com/fluxcd/pkg/artifact/digest](./artifact/digest)** - Multi-algorithm digest computation (SHA1, SHA256, SHA512, BLAKE3)
- **[github.com/fluxcd/pkg/artifact/server](./artifact/server)** - HTTP file server for serving artifacts in-cluster
- **[github.com/fluxcd/pkg/artifact/storage](./artifact/storage)** - Storage management (artifact packaging, retention policies, integrity verification)

### Controller Runtime
- **[github.com/fluxcd/pkg/runtime](./runtime)** - Controller Runtime SDK
    - **[runtime/acl](./runtime/acl)** - Cross-namespace access control utilities
    - **[runtime/cel](./runtime/cel)** - Common Expression Language (CEL) evaluation utilities
    - **[runtime/client](./runtime/client)** - Kubernetes client runtime configuration options
    - **[runtime/conditions](./runtime/conditions)** - Status conditions manipulation utilities
    - **[runtime/controller](./runtime/controller)** - Controller embeddable structs for GitOps Toolkit conventions
    - **[runtime/dependency](./runtime/dependency)** - Dependency sorting for Kubernetes resources
    - **[runtime/errors](./runtime/errors)** - Generic controller and reconciler runtime errors
    - **[runtime/events](./runtime/events)** - Kubernetes Events recording on external HTTP endpoints
    - **[runtime/features](./runtime/features)** - Feature gate management
    - **[runtime/jitter](./runtime/jitter)** - Jitter utilities for reconciliation intervals
    - **[runtime/leaderelection](./runtime/leaderelection)** - Leader election runtime configuration
    - **[runtime/logger](./runtime/logger)** - Logging runtime configuration options
    - **[runtime/metrics](./runtime/metrics)** - Standard metrics recording for GitOps Toolkit components
    - **[runtime/object](./runtime/object)** - Helpers for interacting with GitOps Toolkit objects
    - **[runtime/patch](./runtime/patch)** - Patch utilities for conflict-free object patching
    - **[runtime/pprof](./runtime/pprof)** - pprof endpoints registration helper
    - **[runtime/predicates](./runtime/predicates)** - Controller-runtime predicates for event filtering
    - **[runtime/probes](./runtime/probes)** - Health and readiness probes configuration
    - **[runtime/reconcile](./runtime/reconcile)** - Reconciliation helpers and result finalization
    - **[runtime/secrets](./runtime/secrets)** - Kubernetes secrets handling utilities (TLS, auth, tokens)
    - **[runtime/statusreaders](./runtime/statusreaders)** - Status readers for Kubernetes resources
    - **[runtime/testenv](./runtime/testenv)** - Setup helpers for local Kubernetes test environment
    - **[runtime/transform](./runtime/transform)** - Type transformation utilities
- **[github.com/fluxcd/pkg/ssa](./ssa)** - Kubernetes resources management using server-side apply

### Source Management
- **[github.com/fluxcd/pkg/git](./git)** - Git repository operations, commit verification, and reference handling
- **[github.com/fluxcd/pkg/sourceignore](./sourceignore)** - Gitignore-like functionality for source filtering

### Package Management
- **[github.com/fluxcd/pkg/chartutil](./chartutil)** - Helm chart values management from Kubernetes resources
- **[github.com/fluxcd/pkg/kustomize](./kustomize)** - Generic helpers for Kustomize operations
- **[github.com/fluxcd/pkg/oci](./oci)** - OCI registry operations (push, pull, tag artifacts)

### Utilities
- **[github.com/fluxcd/pkg/cache](./cache)** - Generic cache implementations (expiring and LRU)
- **[github.com/fluxcd/pkg/envsubst](./envsubst)** - Variable expansion in strings using `${var}` syntax
- **[github.com/fluxcd/pkg/lockedfile](./lockedfile)** - Atomic file operations with locking
- **[github.com/fluxcd/pkg/tar](./tar)** - Secure tarball extraction utilities
- **[github.com/fluxcd/pkg/version](./version)** - Semantic version parsing and sorting

### HTTP & Transport
- **[github.com/fluxcd/pkg/http/fetch](./http/fetch)** - Archive fetcher for HTTP resources
- **[github.com/fluxcd/pkg/http/transport](./http/transport)** - HTTP transport utilities

