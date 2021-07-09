# runtime

runtime offers a set of standard controller runtime packages that can be used on their own, but are best (and at times,
must be) used together to help with common operations.

### Goals

- Provide a better development and review experience while working with a set of controllers by creating simple
  APIs for common controller and reconciliation operations, like working with Conditions and Events, and debugging.
- Provide utilities to make it easier to adhere to the
  [Kubernetes API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
  and a selective set of other Kubernetes (SIG) standards like
  [kstatus](https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus).
- Prefer adoption of existing standards and types (like
  [`metav1.Condition`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Condition)) over creating new ones.
- Provide solutions to common difficulties while performing certain Kubernetes operations as a controller, like
  patching.
- Standardise how a controller communicates with the outside world, to improve observation and operation experience.
- Standardise the way controller runtime settings are configured, to improve end-user experience.

### Non-goals

- Become a toolbox for all problems, packages must be of interest to a wide range of controllers (and specifically,
  their runtime operations) before introduction should be considered.
- Adopt conflicting standards without breaking MAJOR version; opinions with versioning.

## Supported standards

The packages build upon the following standards:

- [Kubernetes API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [Kubernetes meta API conditions](https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/1623-standardize-conditions/README.md)
- [kstatus](https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus)

## Usage

To use the packages in your project, import `runtime` using `go get` or your dependency manager of choice:

```shell
go get github.com/fluxcd/pkg/runtime
```
