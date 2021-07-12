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

For all available functions, see the [package reference](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/conditions).

### Forwarding Events

The [`events`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/events) package contains an `events.Recorder` which can
be used to forward events to an external endpoint that understands the `events.Event` payload. For [GitOps
Toolkit](https://fluxcd.io/docs/components/) controllers, this is the notification-controller endpoint.

The package is best used in combination with the [`controller.Metrics` helper](#Controller-helpers), as this allows you
to record _and_ forward Kubernetes Events using the same API.

### Recording Metrics

The [`metrics`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/metrics) package offers a `metrics.Recorder` with a
set of Prometheus collectors for common [GitOps Toolkit](https://fluxcd.io/docs/components/) Kubernetes
resource objects metric points (`gotk_*`).

The package is best used in combination with the [`controller.Events` helper](#Controller-helpers), as this allows you to record metrics using controller-runtime types.

### Controller helpers

The [`controller`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/controller) package offers a collection of helpers
that can be embedded into a reconciler struct to provide them the capabilitiy to e.g. send Kubernetes Events and/or
record metrics.

| Helper | Description | Reference |
|---|---|---|
| `controller.Events` | Provides the capabilities to send Events to the Kubernetes API and an external event recorder | [![GoDoc](https://pkg.go.dev/badge/github.com/fluxcd/pkg/runtime/controller?utm_source=godoc)](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/controller#Events) |
| `controller.Metrics` | Provides the capabilities to record a set of common Prometheus metrics for a Kubernetes resource object | [![GoDoc](https://pkg.go.dev/badge/github.com/fluxcd/pkg/runtime/controller?utm_source=godoc)](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/controller#Metrics) |

### Safe patching

The [`patch`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/patch) package offers a helper utility to safely patch
a Kubernetes resource while taking into account a set of configuration options, and attempting to resolve merge
conflicts and retry before bailing.

It can be configured to understand "owned" Condition types using [`patch.WithOwnedConditions`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/patch#WithOwnedConditions),
and offers other options like [`patch.WithObservedGeneration`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/patch#WithStatusObservedGeneration).

For all available functions and examples, see the [package reference](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/patch).

### Runtime configuration options

Several packages are available to align common runtime configuration flags across a set of controllers, easing the
end-user operator experience.

| Package | Description | Reference |
|---|---|---|
| `client` | Kubernetes runtime client configurations like QPS and burst | [![GoDoc](https://pkg.go.dev/badge/github.com/fluxcd/pkg/runtime/client?utm_source=godoc)](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/client) |
| `leaderelection` | Kubernetes leader election configurations like the lease duration | [![GoDoc](https://pkg.go.dev/badge/github.com/fluxcd/pkg/runtime/leaderelection?utm_source=godoc)](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/leaderelection) |
| `logger` | Runtime logger configurations like the encoding and log level | [![GoDoc](https://pkg.go.dev/badge/github.com/fluxcd/pkg/runtime/logger?utm_source=godoc)](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/logger) |

### Debugging

The [`pprof`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/pprof) package allows setting up additional [Go `pprof`](https://golang.org/pkg/net/http/pprof/)
HTTP handlers on the metrics endpoint of a controller-runtime manager for debugging purposes. A list of exposed
endpoints can be found in [`pprof.Endpoints`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/pprof#Endpoints).

See the [package reference](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/pprof#SetupHandlers) for further
instructions on how to use the package.

### Testing

The [`testenv`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/testenv) package can be utilized to control the
lifecycle of a local Kubernetes api-server used for testing purposes, and offers a set of helper utilities to
work with resources on the cluster.

It allows control over the runtime scheme and
Custom Resource Defintions using [`testenv.WithScheme`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/testenv#WithCRDPath)
and [`testenv.WithCRDPath`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/testenv#WithCRDPath).

For all available functions, see the [package reference](https://pkg.go.dev/github.com/fluxcd/pkg/runtime/testenv).
