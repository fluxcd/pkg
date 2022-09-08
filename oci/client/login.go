package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/oci/auth/login"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
)

// LoginWithCredentials configures the client with static credentials, accepts a single token
// or a user:password format.
func (c *Client) LoginWithCredentials(credentials string) error {
	var authConfig authn.AuthConfig

	if credentials == "" {
		return errors.New("credentials cannot be empty")
	}

	parts := strings.SplitN(credentials, ":", 2)

	if len(parts) == 1 {
		authConfig = authn.AuthConfig{RegistryToken: parts[0]}
	} else {
		authConfig = authn.AuthConfig{Username: parts[0], Password: parts[1]}
	}

	c.options = append(c.options, crane.WithAuth(authn.FromConfig(authConfig)))
	return nil
}

// AutoLogin configures the client to autologin with current supported providers based on the URL
func (c *Client) AutoLogin(ctx context.Context, mgr *login.Manager, url string) error {
	ref, err := name.ParseReference(url)
	if err != nil {
		return fmt.Errorf("could not create reference from url '%s': %w", url, err)
	}

	authenticator, err := mgr.Login(ctx, url, ref, login.ProviderOptions{
		AwsAutoLogin:   true,
		GcpAutoLogin:   true,
		AzureAutoLogin: true,
	})

	if err != nil {
		return fmt.Errorf("could not auto-login to registry with url %s: %w", url, err)
	}

	c.options = append(c.options, crane.WithAuth(authenticator))
	return nil
}
