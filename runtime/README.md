# runtime

[![GoDoc](https://pkg.go.dev/badge/github.com/fluxcd/pkg/runtime?utm_source=godoc)](https://pkg.go.dev/github.com/fluxcd/pkg/runtime)

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

To use the packages in your project, import `github.com/fluxcd/pkg/runtime` using `go get` or your dependency manager
of choice:

```shell
go get github.com/fluxcd/pkg/runtime
```

### Working with Conditions

The [`conditions`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions)
package can be used on resources that implement the [`conditions.Getter`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#Getter)
and/or [`conditions.Setter`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#Setter)
interface, to enhance the experience of working with Conditions on a Kubernetes resource object during reconcile
operations.

More specifically, it allows you to:

- Get a Condition from a Kubernetes resource, or a specific value from a Condition, using
  [`conditions.Get`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#Get)
  or one of the other available getter functions like
  [`conditions.GetMessage`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#GetMessage).
- Check if a Kubernetes resource has a Condition of a given type using
  [`conditions.Has`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#Has),
  or if it bears a Condition in a certain state, for example with
  [`conditions.IsFalse`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#IsFalse).
- Compose [`metav1.Condition`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Condition) structs in a certain
  state using e.g. [`conditions.TrueCondition`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#TrueCondition)
  or [`conditions.FalseCondition`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#FalseCondition).
- Modify the conditions on a Kubernetes resource object using [`conditions.Set`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#Set)
  or one of the available scoped functions like [`conditions.MarkTrue`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#MarkTrue).
- Compose conditions based on other state and/or configurations using
  [`conditions.SetAggregate`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#SetAggregate),
  [`conditions.SetMirror`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#SetMirror)
  and [`conditions.SetSummary`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions#SetSummary).

For all available functions, see the [package documentation](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions).