/*
Copyright 2026 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package spiffe

// Controller flags. All SPIFFE ID flags are repeatable. Each value is parsed by
// cryptov1.ParseSPIFFEID and may be an exact SPIFFE ID, a SPIFFE ID prefix, a
// SPIFFE trust domain or the `spiffe://self` symbol.
const (
	// ControllerFlagSPIFFEBrokerEndpoint defines the flag for the gRPC URL of
	// the SPIFFE Broker endpoint, e.g.
	// dns:///spire-server.spire.svc.cluster.local:443. The Broker API may be
	// served over TCP, unlike the Workload API.
	ControllerFlagSPIFFEBrokerEndpoint = "spiffe-broker-endpoint"

	// ControllerFlagSPIFFEBrokerID defines the flag for the SPIFFE IDs of the
	// SPIFFE Broker endpoint. This flag is repeatable. Each value may be an
	// exact SPIFFE ID or a SPIFFE ID prefix, as parsed by
	// cryptov1.ParseSPIFFEID.
	ControllerFlagSPIFFEBrokerID = "spiffe-broker-id"

	// ControllerFlagAuthorizedArtifactClientSPIFFEID defines the flag for the
	// SPIFFE IDs authorized to fetch artifacts from an artifact server. This
	// flag is repeatable.
	ControllerFlagAuthorizedArtifactClientSPIFFEID = "authorized-artifact-client-spiffe-id"

	// ControllerFlagAuthorizedArtifactServerSPIFFEID defines the flag for the
	// SPIFFE IDs of artifact servers authorized by an artifact client. This
	// flag is repeatable.
	ControllerFlagAuthorizedArtifactServerSPIFFEID = "authorized-artifact-server-spiffe-id"

	// ControllerFlagAuthorizedNotifierSPIFFEID defines the flag for the
	// SPIFFE IDs authorized to send events to an event server. This flag is
	// repeatable.
	ControllerFlagAuthorizedNotifierSPIFFEID = "authorized-notifier-spiffe-id"

	// ControllerFlagAuthorizedEventServerSPIFFEID defines the flag for the
	// SPIFFE IDs of event servers authorized by an event client. This flag is
	// repeatable.
	ControllerFlagAuthorizedEventServerSPIFFEID = "authorized-event-server-spiffe-id"

	// ControllerFlagAuthorizedReceiverSPIFFEID defines the flag for the
	// SPIFFE IDs authorized to send requests to a receiver server. This flag
	// is repeatable.
	ControllerFlagAuthorizedReceiverSPIFFEID = "authorized-receiver-spiffe-id"

	// ControllerFlagAuthorizedKubeAPIServerSPIFFEID defines the flag for the
	// SPIFFE IDs of Kubernetes API servers authorized by a client. This flag
	// is repeatable.
	//
	// Note: Kubernetes does not natively support SPIFFE for API server TLS.
	// We define this flag here as per the RFC for it to be implemented after
	// Kubernetes implements native support, or Flux users request this feature
	// with maybe some custom solution they have to put a SPIFFE proxy in front
	// of their API servers.
	ControllerFlagAuthorizedKubeAPIServerSPIFFEID = "authorized-kube-apiserver-spiffe-id"
)
