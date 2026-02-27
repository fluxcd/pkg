//go:build integration
// +build integration

/*
Copyright 2022 The Flux authors

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

package integration

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/test-infra/tftestenv"
)

const (
	// terraformPathAWS is the path to the terraform working directory
	// containing the aws terraform configurations.
	terraformPathAWS = "./terraform/aws"
	// terraformPathAzure is the path to the terraform working directory
	// containing the azure terraform configurations.
	terraformPathAzure = "./terraform/azure"
	// terraformPathGCP is the path to the terraform working directory
	// containing the gcp terraform configurations.
	terraformPathGCP = "./terraform/gcp"
	// kubeconfigPath is the path where the cluster kubeconfig is written to and
	// used from.
	kubeconfigPath = "./build/kubeconfig"

	resultWaitTimeout = 2 * time.Minute

	// envVarWISANamespace is the name of the terraform environment variable containing
	// the service account namespace used for workload identity.
	envVarWISANamespace = "TF_VAR_wi_k8s_sa_ns"

	// envVarWISAName is the name of the terraform environment variable containing
	// the service account name used for workload identity.
	envVarWISAName = "TF_VAR_wi_k8s_sa_name"

	// envVarWISANameDirectAccess is the name of the terraform environment variable containing
	// the service account name used for workload identity direct access.
	envVarWISANameDirectAccess = "TF_VAR_wi_k8s_sa_name_direct_access"

	// envVarWISANameFederation is the name of the terraform environment variable containing
	// the service account name used for workload identity federation.
	envVarWISANameFederation = "TF_VAR_wi_k8s_sa_name_federation"

	// envVarWISANameFederationDirectAccess is the name of the terraform environment variable containing
	// the service account name used for workload identity federation direct access.
	envVarWISANameFederationDirectAccess = "TF_VAR_wi_k8s_sa_name_federation_direct_access"

	// envVarAzureDevOpsOrg is the name of the terraform environment variable
	// containing the Azure DevOps organization name.
	envVarAzureDevOpsOrg = "TF_VAR_azuredevops_org"

	// envVarAzureDevOpsPAT is the name of the terraform environment variable
	// containing the Azure DevOps personal access token.
	envVarAzureDevOpsPAT = "TF_VAR_azuredevops_pat"

	// wiSANamespace is the namespace of the service account that will be created and annotated for workload
	// identity.
	wiSANamespace = "default"

	// wiServiceAccount is the name of the service account that will be created and annotated for workload
	// identity.
	wiServiceAccount = "test-workload-id"

	// wiServiceAccountDirectAccess is the name of the service account used for
	// workload identity direct access.
	wiServiceAccountDirectAccess = "test-workload-id-direct-access"

	// wiServiceAccountFederation is the name of the service account that will be created and annotated for workload
	// identity federation.
	wiServiceAccountFederation = "test-workload-id-federation"

	// wiServiceAccountFederationDirectAccess is the name of the service account used for
	// workload identity federation direct access.
	wiServiceAccountFederationDirectAccess = "test-workload-id-federation-direct-access"

	// controllerWIRBACName is the name used for RBAC resources a controller needs
	// for impersonating the workload identity service account while obtaining
	// cloud provider credentials. This is needed for testing object-level
	// workload identity.
	controllerWIRBACName = "flux-controller"

	// wiControllerIRSA is the name of the controller service account with
	// IRSA credentials for impersonation testing.
	wiControllerIRSA = "flux-controller-irsa"

	// wiControllerPodIdentity is the name of the controller service account
	// with Pod Identity credentials for impersonation testing.
	wiControllerPodIdentity = "flux-controller-pod-identity"

	// wiControllerGCPSA is the name of the controller service account with
	// GCP service account annotation for impersonation testing.
	wiControllerGCPSA = "flux-controller-gcp-sa"

	// wiAssumeRoleCtrlSA is the name of the target SA for controller-level
	// AWS AssumeRole impersonation (useServiceAccount: false).
	wiAssumeRoleCtrlSA = "test-workload-id-assume-role-ctrl"

	// wiAssumeRoleSA is the name of the target SA for object-level
	// AWS AssumeRole impersonation (useServiceAccount: true).
	wiAssumeRoleSA = "test-workload-id-assume-role"

	// wiImpersonateCtrlSA is the name of the target SA for controller-level
	// GCP impersonation (useServiceAccount: false).
	wiImpersonateCtrlSA = "test-workload-id-impersonate-ctrl"

	// wiImpersonateSA is the name of the target SA for object-level GCP
	// impersonation with GCP SA (useServiceAccount: true).
	wiImpersonateSA = "test-workload-id-impersonate"

	// wiImpersonateDirectAccessSA is the name of the target SA for
	// object-level GCP impersonation with direct access federation
	// (useServiceAccount: true).
	wiImpersonateDirectAccessSA = "test-workload-id-impersonate-da"

	// envVarWISANameAssumeRole is the name of the terraform environment
	// variable for the assume-role SA name (object-level IRSA → AssumeRole).
	envVarWISANameAssumeRole = "TF_VAR_wi_k8s_sa_name_assume_role"

	// envVarWISANameControllerIRSA is the name of the terraform environment
	// variable for the controller IRSA SA name.
	envVarWISANameControllerIRSA = "TF_VAR_wi_k8s_sa_name_controller_irsa"

	// envVarWISANameControllerPodIdentity is the name of the terraform environment
	// variable for the controller Pod Identity SA name.
	envVarWISANameControllerPodIdentity = "TF_VAR_wi_k8s_sa_name_controller_pod_identity"

	// envVarWISANameImpersonationTarget is the name of the terraform environment
	// variable for the GCP impersonation target SA name (with GCP SA annotation).
	envVarWISANameImpersonationTarget = "TF_VAR_wi_k8s_sa_name_impersonation_target"

	// envVarWISANameImpersonationDA is the name of the terraform environment
	// variable for the GCP impersonation target SA name (with WIF direct access).
	envVarWISANameImpersonationDA = "TF_VAR_wi_k8s_sa_name_impersonation_da"

	// envVarWISANameController is the name of the terraform environment
	// variable for the default controller SA name (for GKE WIF principal).
	envVarWISANameController = "TF_VAR_wi_k8s_sa_name_controller"

	// envVarWISANameControllerGCPSA is the name of the terraform environment
	// variable for the controller SA with GCP SA annotation.
	envVarWISANameControllerGCPSA = "TF_VAR_wi_k8s_sa_name_controller_gcp_sa"

	// skippedMessage is the message used to skip tests for features
	// that are not supported by the provider or cluster configuration.
	skippedMessage = "Skipping test, feature not supported by the provider or by the current cluster configuration"
)

var (
	// targetProvider is the name of the kubernetes provider to test against.
	targetProvider = flag.String("provider", "", "one of aws, azure or gcp")

	// testGit tells whether to run the git tests or not.
	// It can only be set to true when enableWI is true,
	// as we only support Git authentication through
	// workload identity.
	testGit bool

	// testRESTConfig tells whether to run the REST config tests or not.
	// It can only be set to true when enableWI is true,
	// as we only support cluster authentication through
	// workload identity.
	testRESTConfig bool

	// retain flag to prevent destroy and retaining the created infrastructure.
	retain = flag.Bool("retain", false, "retain the infrastructure for debugging purposes")

	// existing flag to use existing infrastructure terraform state.
	existing = flag.Bool("existing", false, "use existing infrastructure state for debugging purposes")

	// verbose flag to enable output of terraform execution.
	verbose = flag.Bool("verbose", false, "verbose output of the environment setup")

	// destroyOnly flag to destroy any provisioned infrastructure.
	destroyOnly = flag.Bool("destroy-only", false, "run in destroy-only mode and delete any existing infrastructure")

	// testRepos is a map of registry common name and URL of the test
	// repositories. This is used as the test cases to run the tests against.
	// The registry common name need not be the actual registry address but an
	// identifier to identify the test case without logging any sensitive
	// account IDs in the subtest names.
	// For example, map[string]string{"ecr", "xxxxx.dkr.ecr.xxxx.amazonaws.com/foo:v1"}
	// would result in subtest name TestImageRepositoryScanAWS/ecr.
	testRepos map[string]string

	// testEnv is the test environment. It contains test infrastructure and
	// kubernetes client of the created cluster.
	testEnv *tftestenv.Environment

	// testImageTags are the tags used in the test for the generated images.
	testImageTags = []string{"v0.1.0", "v0.1.2", "v0.1.3", "v0.1.4"}

	testAppImage string

	// enableWI is set to true when the TF_VAR_enable_wi is set to "true", so the tests run for Workload Identity
	enableWI bool

	// testWIDirectAccess is set by the provider config.
	testWIDirectAccess bool

	// testWIFederation is set by the provider config.
	testWIFederation bool

	// testImpersonation is set when impersonation testing is enabled.
	testImpersonation bool

	// testGitCfg is a struct containing different variables needed for running git tests.
	testGitCfg *gitTestConfig

	// gitSSHURL is the SSH URL of the git repository used for testing.
	gitSSHURL string
)

// registryLoginFunc is used to perform registry login against a provider based
// on the terraform state output values. It returns a map of registry common
// name and test repositories to test against, read from the terraform state
// output.
type registryLoginFunc func(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error)

// pushTestImages is used to push local flux test images to a remote registry
// after logging in using registryLoginFunc. It takes a map of image name and
// local images and terraform state output. The local images are retagged and
// pushed to a corresponding registry repository for the image.
type pushTestImages func(ctx context.Context, localImgs map[string]string, output map[string]*tfjson.StateOutput) (map[string]string, error)

// getWISAAnnotations returns cloud provider specific annotations for the
// service account when workload identity is used on the cluster.
type getWISAAnnotations func(output map[string]*tfjson.StateOutput) (map[string]string, error)

// getWIFederationSAAnnotations returns cloud provider specific annotations for the
// service account when workload identity federation is used on the cluster.
type getWIFederationSAAnnotations func(output map[string]*tfjson.StateOutput) (map[string]string, error)

// getClusterConfigMap returns the cluster configmap data for kubeconfig auth tests.
type getClusterConfigMap func(output map[string]*tfjson.StateOutput) (map[string]string, error)

// getClusterUsers returns the cluster users for kubeconfig auth tests.
type getClusterUsers func(output map[string]*tfjson.StateOutput) ([]string, error)

// grantPermissionsToGitRepository calls provider specific API to add additional permissions to the git repository/project
type grantPermissionsToGitRepository func(ctx context.Context, cfg *gitTestConfig, output map[string]*tfjson.StateOutput) error

// revokePermissionsToGitRepository calls provider specific API to revoke permissions to the git repository/project
type revokePermissionsToGitRepository func(ctx context.Context, cfg *gitTestConfig, output map[string]*tfjson.StateOutput) error

// getGitTestConfig gets the configuration for the tests
type getGitTestConfig func(output map[string]*tfjson.StateOutput) (*gitTestConfig, error)

// loadGitSSHSecret is used to load the SSH key pair for git authentication.
type loadGitSSHSecret func(output map[string]*tfjson.StateOutput) (map[string]string, string, error)

// getImpersonationAnnotations returns multiple sets of annotations for creating
// impersonation-related service accounts. The returned map is keyed by SA name.
type getImpersonationAnnotations func(output map[string]*tfjson.StateOutput) (map[string]map[string]string, error)

// getControllerAnnotations returns annotations for a controller service account.
type getControllerAnnotations func(output map[string]*tfjson.StateOutput) (map[string]map[string]string, error)

// gitTestConfig hold different variable that will be needed by the different test functions.
type gitTestConfig struct {
	// authentication info for git repositories
	gitPat                           string
	gitUsername                      string
	defaultGitTransport              git.TransportType
	defaultAuthOpts                  *git.AuthOptions
	applicationRepository            string
	applicationRepositoryWithoutUser string
	organization                     string
	// permissionID is a string that represents the entity that was granted
	// permissions on the git repository
	permissionID string
}

// ProviderConfig is the test configuration of a supported cloud provider to run
// the tests against.
type ProviderConfig struct {
	// terraformPath is the path to the directory containing the terraform
	// configurations of the provider.
	terraformPath string
	// registryLogin is used to perform registry login.
	registryLogin registryLoginFunc
	// createKubeconfig is used to create kubeconfig of a cluster.
	createKubeconfig tftestenv.CreateKubeconfig
	// pushAppTestImages is used to push flux test images to a remote registry.
	pushAppTestImages pushTestImages
	// getWISAAnnotations is used to return the provider specific annotations
	// for the service account when using workload identity.
	getWISAAnnotations getWISAAnnotations
	// getWIFederationSAAnnotations is used to return the provider specific annotations
	// for the service account when using workload identity federation.
	getWIFederationSAAnnotations getWIFederationSAAnnotations
	// getClusterConfigMap is used to return the cluster resource for kubeconfig auth tests.
	getClusterConfigMap getClusterConfigMap
	// getClusterUsers is used to return the cluster users for kubeconfig auth tests.
	getClusterUsers getClusterUsers
	// grantPermissionsToGitRepository is used to give the identity access to the Git repository
	grantPermissionsToGitRepository grantPermissionsToGitRepository
	// revokePermissionsToGitRepository is used to revoke the identity access to the Git repository
	revokePermissionsToGitRepository revokePermissionsToGitRepository
	// getGitTestConfig is used to return provider specific test configuration
	getGitTestConfig getGitTestConfig
	// supportsWIDirectAccess is a boolean that indicates if the test should run
	// for workload identity direct access.
	supportsWIDirectAccess bool
	// supportsWIFederation is a boolean that indicates if the test should run
	// for workload identity federation.
	supportsWIFederation bool
	// supportsGit is a boolean that indicates if the test should run for git.
	supportsGit bool
	// loadGitSSHSecret is used to load the SSH key pair for git authentication.
	loadGitSSHSecret loadGitSSHSecret
	// supportsImpersonation indicates whether impersonation tests should run.
	supportsImpersonation bool
	// getImpersonationAnnotations returns annotations for impersonation target SAs.
	getImpersonationAnnotations getImpersonationAnnotations
	// getControllerAnnotations returns annotations for controller SAs
	// used in impersonation testing.
	getControllerAnnotations getControllerAnnotations
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func TestMain(m *testing.M) {
	flag.Parse()
	ctx := context.TODO()

	// Validate the provider.
	if *targetProvider == "" {
		log.Fatalf("-provider flag must be set to one of aws, azure or gcp")
	}

	enableWI = os.Getenv("TF_VAR_enable_wi") == "true"

	providerCfg := getProviderConfig(*targetProvider)
	if providerCfg == nil {
		log.Fatalf("Failed to get provider config for %q", *targetProvider)
	}
	if enableWI {
		testWIDirectAccess = providerCfg.supportsWIDirectAccess
		testWIFederation = providerCfg.supportsWIFederation
		testImpersonation = providerCfg.supportsImpersonation
		// we only support git with workload identity
		testGit = providerCfg.supportsGit
		// we only support cluster auth with workload identity
		testRESTConfig = true
	}

	os.Setenv(envVarWISANamespace, wiSANamespace)
	os.Setenv(envVarWISAName, wiServiceAccount)
	os.Setenv(envVarWISANameDirectAccess, wiServiceAccountDirectAccess)
	os.Setenv(envVarWISANameFederation, wiServiceAccountFederation)
	os.Setenv(envVarWISANameFederationDirectAccess, wiServiceAccountFederationDirectAccess)
	os.Setenv(envVarWISANameAssumeRole, wiAssumeRoleSA)
	os.Setenv(envVarWISANameControllerIRSA, wiControllerIRSA)
	os.Setenv(envVarWISANameControllerPodIdentity, wiControllerPodIdentity)
	os.Setenv(envVarWISANameImpersonationTarget, wiImpersonateSA)
	os.Setenv(envVarWISANameImpersonationDA, wiImpersonateDirectAccessSA)
	os.Setenv(envVarWISANameController, controllerWIRBACName)
	os.Setenv(envVarWISANameControllerGCPSA, wiControllerGCPSA)

	// Run destroy-only mode if enabled.
	if *destroyOnly {
		log.Println("Running in destroy-only mode...")
		envOpts := []tftestenv.EnvironmentOption{
			tftestenv.WithVerbose(*verbose),
			// Ignore any state lock in destroy-only mode.
			tftestenv.WithTfDestroyOptions(tfexec.Lock(false)),
		}
		if err := tftestenv.Destroy(ctx, providerCfg.terraformPath, envOpts...); err != nil {
			panic(err)
		}
		os.Exit(0)
	}

	// Check the test app image.
	appImg := os.Getenv("TEST_IMG")
	if appImg == "" {
		log.Fatal("TEST_IMG must be set to the test application image, cannot be empty")
	}

	localImgs := map[string]string{
		"app": appImg,
	}

	// Construct scheme to be added to the kubeclient.
	scheme := runtime.NewScheme()

	err := batchv1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	err = corev1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	err = rbacv1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	// Initialize with non-zero exit code to indicate failure by default unless
	// set by a successful test run.
	exitCode := 1

	// Create environment.
	envOpts := []tftestenv.EnvironmentOption{
		tftestenv.WithVerbose(*verbose),
		tftestenv.WithRetain(*retain),
		tftestenv.WithExisting(*existing),
		tftestenv.WithCreateKubeconfig(providerCfg.createKubeconfig),
	}
	testEnv, err = tftestenv.New(ctx, scheme, providerCfg.terraformPath, kubeconfigPath, envOpts...)
	if err != nil {
		panic(fmt.Sprintf("Failed to provision the test infrastructure: %v", err))
	}

	// Stop the environment before exit.
	defer func() {
		if err := testEnv.Stop(ctx); err != nil {
			log.Printf("Failed to stop environment: %v", err)
			exitCode = 1
		}

		// Log the panic error before exit to surface the cause of panic.
		if err := recover(); err != nil {
			log.Printf("panic: %v", err)
		}
		os.Exit(exitCode)
	}()

	// Get terraform state output.
	output, err := testEnv.StateOutput(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to get the terraform state output: %v", err))
	}

	// Cleanup infra that depends on terraform output before exit
	defer func() {
		if !*retain {
			if testGitCfg != nil && testGitCfg.permissionID != "" {
				err := providerCfg.revokePermissionsToGitRepository(ctx, testGitCfg, output)
				if err != nil {
					log.Printf("Failed to revoke permissions to git repository: %s", err)
					exitCode = 1
				}
			}
		}
	}()

	if testGit {
		// Populate the global git config.
		testGitCfg, err = providerCfg.getGitTestConfig(output)
		if err != nil {
			panic(fmt.Sprintf("Failed to get git test config: %v", err))
		}
	}

	pushAppImage(ctx, providerCfg, output, localImgs)
	configureAdditionalInfra(ctx, providerCfg, output)

	exitCode = m.Run()
}

// getProviderConfig returns the test configuration of supported providers.
func getProviderConfig(provider string) *ProviderConfig {
	switch provider {
	case "aws":
		return &ProviderConfig{
			terraformPath:                    terraformPathAWS,
			registryLogin:                    registryLoginECR,
			pushAppTestImages:                pushAppTestImagesECR,
			createKubeconfig:                 createKubeconfigEKS,
			getWISAAnnotations:               getWISAAnnotationsAWS,
			getClusterConfigMap:              getClusterConfigMapAWS,
			getClusterUsers:                  getClusterUsersAWS,
			grantPermissionsToGitRepository:  grantPermissionsToGitRepositoryAWS,
			revokePermissionsToGitRepository: revokePermissionsToGitRepositoryAWS,
			getGitTestConfig:                 getGitTestConfigAWS,
			supportsImpersonation:            true,
			getImpersonationAnnotations:      getImpersonationAnnotationsAWS,
			getControllerAnnotations:         getControllerAnnotationsAWS,
		}
	case "azure":
		providerCfg := &ProviderConfig{
			terraformPath:                    terraformPathAzure,
			registryLogin:                    registryLoginACR,
			pushAppTestImages:                pushAppTestImagesACR,
			createKubeconfig:                 createKubeConfigAKS,
			getWISAAnnotations:               getWISAAnnotationsAzure,
			getClusterConfigMap:              getClusterConfigMapAzure,
			getClusterUsers:                  getClusterUsersAzure,
			grantPermissionsToGitRepository:  grantPermissionsToGitRepositoryAzure,
			revokePermissionsToGitRepository: revokePermissionsToGitRepositoryAzure,
			getGitTestConfig:                 getGitTestConfigAzure,
			loadGitSSHSecret:                 loadGitSSHSecretAzure,
			supportsGit:                      true,
		}
		return providerCfg
	case "gcp":
		return &ProviderConfig{
			terraformPath:                    terraformPathGCP,
			registryLogin:                    registryLoginGAR,
			pushAppTestImages:                pushAppTestImagesGAR,
			createKubeconfig:                 createKubeconfigGKE,
			getWISAAnnotations:               getWISAAnnotationsGCP,
			getWIFederationSAAnnotations:     getWIFederationSAAnnotationsGCP,
			getClusterConfigMap:              getClusterConfigMapGCP,
			getClusterUsers:                  getClusterUsersGCP,
			grantPermissionsToGitRepository:  grantPermissionsToGitRepositoryGCP,
			revokePermissionsToGitRepository: revokePermissionsToGitRepositoryGCP,
			getGitTestConfig:                 getGitTestConfigGCP,
			supportsWIDirectAccess:           true,
			supportsWIFederation:             true,
			supportsImpersonation:            true,
			getImpersonationAnnotations:      getImpersonationAnnotationsGCP,
			getControllerAnnotations:         getControllerAnnotationsGCP,
		}
	}
	return nil
}

func pushAppImage(ctx context.Context, providerCfg *ProviderConfig, tfOutput map[string]*tfjson.StateOutput, localImgs map[string]string) {
	_, err := providerCfg.registryLogin(ctx, tfOutput)
	if err != nil {
		panic(fmt.Sprintf("Failed to log into registry: %v", err))
	}
	pushedImages, err := providerCfg.pushAppTestImages(ctx, localImgs, tfOutput)
	if err != nil {
		panic(fmt.Sprintf("Failed to push test images: %v", err))
	}

	if len(pushedImages) != 1 {
		panic(fmt.Sprintf("Unexpected number of app images pushed: %d", len(pushedImages)))
	}

	if appImg, ok := pushedImages["app"]; !ok {
		panic(fmt.Sprintf("Could not find pushed app image in %v", pushedImages))
	} else {
		testAppImage = appImg
	}
}

func configureAdditionalInfra(ctx context.Context, providerCfg *ProviderConfig, tfOutput map[string]*tfjson.StateOutput) {
	log.Println("push oci test images")
	pushOciTestImages(ctx, providerCfg, tfOutput)

	if testGit && testGitCfg != nil {
		// Call provider specific API to configure permisions for the git repository
		log.Println("Git is enabled, granting permissions to workload identity to access repository")
		if err := providerCfg.grantPermissionsToGitRepository(ctx, testGitCfg, tfOutput); err != nil {
			panic(fmt.Sprintf("Failed to grant permissions to repository: %v", err))
		}

		// Load the SSH key pair for git authentication, create a secret with it
		// and allow all service accounts in the cluster to read it.
		if providerCfg.loadGitSSHSecret != nil {
			secretData, sshURL, err := providerCfg.loadGitSSHSecret(tfOutput)
			if err != nil {
				panic(err)
			}
			gitSSHURL = sshURL
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh-key",
					Namespace: wiSANamespace,
				},
				StringData: secretData,
			}
			_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, secret, func() error {
				secret.StringData = secretData
				return nil
			})
			if err != nil {
				panic(err)
			}
			rules := []rbacv1.PolicyRule{{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				Verbs:         []string{"get"},
				ResourceNames: []string{"git-ssh-key"},
			}}
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh-key-reader",
					Namespace: wiSANamespace,
				},
				Rules: rules,
			}
			_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, role, func() error {
				role.Rules = rules
				return nil
			})
			if err != nil {
				panic(err)
			}
			roleRef := rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "Role",
				Name:     role.Name,
			}
			subjects := []rbacv1.Subject{{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "Group",
				Name:     "system:serviceaccounts:" + wiSANamespace,
			}}
			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh-key-reader",
					Namespace: wiSANamespace,
				},
				Subjects: subjects,
				RoleRef:  roleRef,
			}
			_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, roleBinding, func() error {
				roleBinding.Subjects = subjects
				roleBinding.RoleRef = roleRef
				return nil
			})
			if err != nil {
				panic(err)
			}
		}
	}

	if enableWI {
		log.Println("Workload identity is enabled, initializing service account with annotations")

		annotations, err := providerCfg.getWISAAnnotations(tfOutput)
		if err != nil {
			panic(err)
		}

		var federationAnnotations map[string]string
		if providerCfg.supportsWIFederation {
			federationAnnotations, err = providerCfg.getWIFederationSAAnnotations(tfOutput)
			if err != nil {
				panic(err)
			}
		}

		if err := createWorkloadIDServiceAccount(ctx, annotations); err != nil {
			panic(err)
		}

		if providerCfg.supportsWIDirectAccess {
			if err := createDirectAccessWorkloadIdentityServiceAccount(ctx); err != nil {
				panic(err)
			}

			if providerCfg.supportsWIFederation {
				if err := createDirectAccessWorkloadIdentityFederationServiceAccount(ctx, federationAnnotations); err != nil {
					panic(err)
				}
			}
		}

		if providerCfg.supportsWIFederation {
			if err := createWorkloadIdentityFederationServiceAccount(ctx, annotations, federationAnnotations); err != nil {
				panic(err)
			}
		}

		if err := createControllerWorkloadIdentityServiceAccount(ctx); err != nil {
			panic(err)
		}

		clusterCMData, err := providerCfg.getClusterConfigMap(tfOutput)
		if err != nil {
			panic(err)
		}
		if err := createClusterConfigMapAndConfigureRBAC(ctx, clusterCMData); err != nil {
			panic(err)
		}

		clusterUsers, err := providerCfg.getClusterUsers(tfOutput)
		if err != nil {
			panic(err)
		}
		if err := grantNamespaceAdminToClusterUsers(ctx, clusterUsers); err != nil {
			panic(err)
		}

		if testImpersonation {
			log.Println("Impersonation is enabled, creating controller and target service accounts")

			// Create impersonation target SAs (provider-specific annotations).
			impersonationAnnotations, err := providerCfg.getImpersonationAnnotations(tfOutput)
			if err != nil {
				panic(err)
			}
			for saName, saAnnotations := range impersonationAnnotations {
				if err := createServiceAccountWithAnnotations(ctx, saName, saAnnotations); err != nil {
					panic(fmt.Sprintf("failed to create impersonation SA %s: %v", saName, err))
				}
			}

			// Create controller SAs for impersonation (provider-specific annotations).
			controllerAnnotations, err := providerCfg.getControllerAnnotations(tfOutput)
			if err != nil {
				panic(err)
			}
			for saName, saAnnotations := range controllerAnnotations {
				if err := createControllerServiceAccount(ctx, saName, saAnnotations); err != nil {
					panic(fmt.Sprintf("failed to create controller SA %s: %v", saName, err))
				}
			}
		}
	}
}

func pushOciTestImages(ctx context.Context, providerCfg *ProviderConfig, tfOutput map[string]*tfjson.StateOutput) {
	var err error
	testRepos, err = providerCfg.registryLogin(ctx, tfOutput)
	if err != nil {
		panic(fmt.Sprintf("Failed to log into registry: %v", err))
	}

	// Create and push test images.
	if err := tftestenv.CreateAndPushImages(testRepos, testImageTags); err != nil {
		panic(fmt.Sprintf("Failed to create and push images: %v", err))
	}
}

// creatWorkloadIDServiceAccount creates the service account (name and namespace specified in the terraform
// variables) with the annotations passed into the function.
func createWorkloadIDServiceAccount(ctx context.Context, annotations map[string]string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wiServiceAccount,
			Namespace: wiSANamespace,
		},
	}

	sa.Annotations = annotations
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, sa, func() error {
		sa.Annotations = annotations
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create service account for workload identity: %w", err)
	}

	return nil
}

// createDirectAccessWorkloadIdentityServiceAccount creates a service account
// for testing direct access to workload identity.
func createDirectAccessWorkloadIdentityServiceAccount(ctx context.Context) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wiServiceAccountDirectAccess,
			Namespace: wiSANamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, sa, func() error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create direct access service account for workload identity: %w", err)
	}
	return nil
}

// createDirectAccessWorkloadIdentityFederationServiceAccount creates a service account
// for testing workload identity federation.
func createDirectAccessWorkloadIdentityFederationServiceAccount(ctx context.Context, federationAnnotations map[string]string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        wiServiceAccountFederationDirectAccess,
			Namespace:   wiSANamespace,
			Annotations: federationAnnotations,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, sa, func() error {
		sa.Annotations = federationAnnotations
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create direct access service account for workload identity federation: %w", err)
	}
	return nil
}

// createWorkloadIdentityFederationServiceAccount creates a service account
// for testing workload identity federation.
func createWorkloadIdentityFederationServiceAccount(ctx context.Context, annotations, federationAnnotations map[string]string) error {
	allAnnotations := make(map[string]string, len(annotations)+len(federationAnnotations))
	for k, v := range annotations {
		allAnnotations[k] = v
	}
	for k, v := range federationAnnotations {
		allAnnotations[k] = v
	}
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        wiServiceAccountFederation,
			Namespace:   wiSANamespace,
			Annotations: allAnnotations,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, sa, func() error {
		sa.Annotations = allAnnotations
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create service account for workload identity federation: %w", err)
	}
	return nil
}

// createControllerWorkloadIdentityServiceAccount creates a service account
// with RBAC permissions for impersonating the workload identity service account
// while obtaining cloud provider credentials. This service account is needed
// for testing object-level workload identity.
func createControllerWorkloadIdentityServiceAccount(ctx context.Context) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerWIRBACName,
			Namespace: wiSANamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, sa, func() error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create controller service account for workload identity: %w", err)
	}

	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"serviceaccounts"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"serviceaccounts/token"},
			Verbs:     []string{"create"},
		},
	}
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllerWIRBACName,
		},
		Rules: rules,
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, clusterRole, func() error {
		clusterRole.Rules = rules
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create controller cluster role for workload identity: %w", err)
	}

	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.SchemeGroupVersion.Group,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}
	subjects := []rbacv1.Subject{{
		Kind:      "ServiceAccount",
		Name:      sa.Name,
		Namespace: sa.Namespace,
	}}
	roleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllerWIRBACName,
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, roleBinding, func() error {
		roleBinding.RoleRef = roleRef
		roleBinding.Subjects = subjects
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create controller cluster role binding for workload identity: %w", err)
	}

	return nil
}

// createServiceAccountWithAnnotations creates a service account with the given
// name and annotations in the default namespace.
func createServiceAccountWithAnnotations(ctx context.Context, name string, annotations map[string]string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: wiSANamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, sa, func() error {
		sa.Annotations = annotations
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create service account %s: %w", name, err)
	}
	return nil
}

// createControllerServiceAccount creates a controller service account with the
// given name and annotations, and binds it to the existing controller ClusterRole
// so it can impersonate workload identity service accounts.
func createControllerServiceAccount(ctx context.Context, name string, annotations map[string]string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: wiSANamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, sa, func() error {
		sa.Annotations = annotations
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create controller service account %s: %w", name, err)
	}

	// Bind the controller SA to the existing ClusterRole.
	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.SchemeGroupVersion.Group,
		Kind:     "ClusterRole",
		Name:     controllerWIRBACName,
	}
	subjects := []rbacv1.Subject{{
		Kind:      "ServiceAccount",
		Name:      name,
		Namespace: wiSANamespace,
	}}
	roleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, roleBinding, func() error {
		roleBinding.RoleRef = roleRef
		roleBinding.Subjects = subjects
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create controller cluster role binding for %s: %w", name, err)
	}

	return nil
}

// createClusterConfigMapAndConfigureRBAC creates a configmap with the cluster
// kubeconfig and configures RBAC to allow the test jobs to read it.
func createClusterConfigMapAndConfigureRBAC(ctx context.Context, cmData map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig",
			Namespace: wiSANamespace,
		},
		Data: cmData,
	}
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, cm, func() error {
		cm.Data = cmData
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create configmap for kubeconfig auth tests: %w", err)
	}

	rules := []rbacv1.PolicyRule{{
		APIGroups:     []string{""},
		Resources:     []string{"configmaps"},
		ResourceNames: []string{cm.Name},
		Verbs:         []string{"get"},
	}}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-configmap",
			Namespace: cm.Namespace,
		},
		Rules: rules,
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, role, func() error {
		role.Rules = rules
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create role for kubeconfig auth tests: %w", err)
	}

	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.SchemeGroupVersion.Group,
		Kind:     "Role",
		Name:     role.Name,
	}
	subjects := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      wiServiceAccount,
			Namespace: cm.Namespace,
		},
		{
			Kind:      "ServiceAccount",
			Name:      controllerWIRBACName,
			Namespace: cm.Namespace,
		},
		{
			Kind:      "ServiceAccount",
			Name:      wiControllerIRSA,
			Namespace: cm.Namespace,
		},
		{
			Kind:      "ServiceAccount",
			Name:      wiControllerPodIdentity,
			Namespace: cm.Namespace,
		},
		{
			Kind:      "ServiceAccount",
			Name:      wiControllerGCPSA,
			Namespace: cm.Namespace,
		},
	}
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-configmap",
			Namespace: cm.Namespace,
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}
	if _, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, roleBinding, func() error {
		roleBinding.RoleRef = roleRef
		roleBinding.Subjects = subjects
		return nil
	}); err != nil {
		return fmt.Errorf("failed to create role binding for kubeconfig auth tests: %w", err)
	}

	return nil
}

// grantNamespaceAdminToClusterUsers creates a role binding for the
// cluster users to have namespace admin permissions in the default
// namespace. This is needed for kubeconfig auth tests.
func grantNamespaceAdminToClusterUsers(ctx context.Context, clusterUsers []string) error {
	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.SchemeGroupVersion.Group,
		Kind:     "ClusterRole",
		Name:     "cluster-admin",
	}
	subjects := make([]rbacv1.Subject, 0, len(clusterUsers))
	for _, clusterUser := range clusterUsers {
		subjects = append(subjects, rbacv1.Subject{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     rbacv1.UserKind,
			Name:     clusterUser,
		})
	}
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restconfig-tests",
			Namespace: wiSANamespace,
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, roleBinding, func() error {
		roleBinding.RoleRef = roleRef
		roleBinding.Subjects = subjects
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create role binding for cluster users %v: %w", clusterUsers, err)
	}
	return nil
}
