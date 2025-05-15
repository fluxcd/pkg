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

	// envVarAzureDevOpsOrg is the name of the terraform environment variable
	// containing the Azure DevOps organization name.
	envVarAzureDevOpsOrg = "TF_VAR_azuredevops_org"

	// envVarAzureDevOpsPAT is the name of the terraform environment variable
	// containing the Azure DevOps personal access token.
	envVarAzureDevOpsPAT = "TF_VAR_azuredevops_pat"

	// wiSANamespace is the namespace of the service account that will be created and annotated for workload
	// identity. It is set from the terraform variable (`TF_VAR_wi_k8s_sa_ns`)
	wiSANamespace = "default"

	// wiServiceAccount is the name of the service account that will be created and annotated for workload
	// identity. It is set from the terraform variable (`TF_VAR_wi_k8s_sa_name`)
	wiServiceAccount = "test-workload-id"

	// wiServiceAccountDirectAccess is the name of the service account used for
	// workload identity direct access.
	wiServiceAccountDirectAccess = "test-workload-id-direct-access"

	// controllerWIRBACName is the name used for RBAC resources a controller needs
	// for impersonating the workload identity service account while obtaining
	// cloud provider credentials. This is needed for testing object-level
	// workload identity.
	controllerWIRBACName = "flux-controller"
)

var (
	// targetProvider is the name of the kubernetes provider to test against.
	targetProvider = flag.String("provider", "", "one of aws, azure or gcp")

	// testGit tells whether to run the git tests or not.
	testGit bool

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

	// testGitCfg is a struct containing different variables needed for running git tests.
	testGitCfg *gitTestConfig
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

// grantPermissionsToGitRepository calls provider specific API to add additional permissions to the git repository/project
type grantPermissionsToGitRepository func(ctx context.Context, cfg *gitTestConfig, output map[string]*tfjson.StateOutput) error

// revokePermissionsToGitRepository calls provider specific API to revoke permissions to the git repository/project
type revokePermissionsToGitRepository func(ctx context.Context, cfg *gitTestConfig, output map[string]*tfjson.StateOutput) error

// getGitTestConfig gets the configuration for the tests
type getGitTestConfig func(output map[string]*tfjson.StateOutput) (*gitTestConfig, error)

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
	// grantPermissionsToGitRepository is used to give the identity access to the Git repository
	grantPermissionsToGitRepository grantPermissionsToGitRepository
	// revokePermissionsToGitRepository is used to revoke the identity access to the Git repository
	revokePermissionsToGitRepository revokePermissionsToGitRepository
	// getGitTestConfig is used to return provider specific test configuration
	getGitTestConfig getGitTestConfig
	// supportsWIDirectAccess is a boolean that indicates if the test should run
	// for workload identity direct access.
	supportsWIDirectAccess bool
	// supportsGit is a boolean that indicates if the test should run for git.
	supportsGit bool
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
	}
	testGit = providerCfg.supportsGit

	os.Setenv(envVarWISANamespace, wiSANamespace)
	os.Setenv(envVarWISAName, wiServiceAccount)
	os.Setenv(envVarWISANameDirectAccess, wiServiceAccountDirectAccess)

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
			grantPermissionsToGitRepository:  grantPermissionsToGitRepositoryAWS,
			revokePermissionsToGitRepository: revokePermissionsToGitRepositoryAWS,
			getGitTestConfig:                 getGitTestConfigAWS,
		}
	case "azure":
		providerCfg := &ProviderConfig{
			terraformPath:                    terraformPathAzure,
			registryLogin:                    registryLoginACR,
			pushAppTestImages:                pushAppTestImagesACR,
			createKubeconfig:                 createKubeConfigAKS,
			getWISAAnnotations:               getWISAAnnotationsAzure,
			grantPermissionsToGitRepository:  grantPermissionsToGitRepositoryAzure,
			revokePermissionsToGitRepository: revokePermissionsToGitRepositoryAzure,
			getGitTestConfig:                 getGitTestConfigAzure,
			supportsGit:                      enableWI, // azure only supports git with workload identity
		}
		return providerCfg
	case "gcp":
		return &ProviderConfig{
			terraformPath:                    terraformPathGCP,
			registryLogin:                    registryLoginGCR,
			pushAppTestImages:                pushAppTestImagesGCR,
			createKubeconfig:                 createKubeconfigGKE,
			getWISAAnnotations:               getWISAAnnotationsGCP,
			grantPermissionsToGitRepository:  grantPermissionsToGitRepositoryGCP,
			revokePermissionsToGitRepository: revokePermissionsToGitRepositoryGCP,
			getGitTestConfig:                 getGitTestConfigGCP,
			supportsWIDirectAccess:           true,
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
	}

	if enableWI {
		log.Println("Workload identity is enabled, initializing service account with annotations")
		annotations, err := providerCfg.getWISAAnnotations(tfOutput)
		if err != nil {
			panic(fmt.Sprintf("Failed to get service account func for workload identity: %v", err))
		}

		if err := createWorkloadIDServiceAccount(ctx, annotations); err != nil {
			panic(err)
		}

		if providerCfg.supportsWIDirectAccess {
			if err := createDirectAccessWorkloadIdentityServiceAccount(ctx); err != nil {
				panic(err)
			}
		}

		if err := createControllerWorkloadIdentityServiceAccount(ctx); err != nil {
			panic(err)
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
	if err := testEnv.Client.Create(ctx, sa); err != nil {
		return fmt.Errorf("failed to create direct access service account for workload identity: %w", err)
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
	if err := testEnv.Client.Create(ctx, sa); err != nil {
		return fmt.Errorf("failed to create controller service account for workload identity: %w", err)
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllerWIRBACName,
		},
		Rules: []rbacv1.PolicyRule{
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
		},
	}
	if err := testEnv.Client.Create(ctx, clusterRole); err != nil {
		return fmt.Errorf("failed to create controller cluster role for workload identity: %w", err)
	}

	roleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllerWIRBACName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
	}
	if err := testEnv.Client.Create(ctx, roleBinding); err != nil {
		return fmt.Errorf("failed to create controller cluster role binding for workload identity: %w", err)
	}

	return nil
}
