# Git E2E Tests

This package contains E2E tests for `pkg/git/gogit`. The current
tests are run against the following providers:

* [GitHub](https://github.com)
* [Gitlab](https://gitlab.com)
* GitLab CE (self-hosted)
* Bitbucket Server (self-hosted)
* Gitkit (a test Git server to test custom configuration)

## Usage

### Gitkit

```shell
GO_TEST_PREFIX='TestGitKitE2E' ./run.sh
```

### GitLab CE

GitLab CE is run inside a Docker container. You can specify the following environment variables
related to the container:

* `PERSIST_GITLAB=true`: Persist the GitLab container and reuse it in subsequent test executions. To destroy
   the container after the test, omit this environment variable.
* `REGISTRY=harbor.example.com/dockerhub`: Registry proxy for the container image. Defaults to empty.
* `GITLAB_CONTAINER`: Name of the contianer running GitLab. Defaults to gitlab-flux-e2e.

```shell
GO_TEST_PREFIX='TestGitLabCEE2E' ./run.sh
```

### GitHub

You need to create a PAT (classic) associated with your account. This is used to
create and delete test repositories and to grant permissions to the github app.
You can do so by following this
[guide](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token).
The token should have the following permission scopes:
* `repo`: Full control of private repositories
* `admin:public_key`: Full control of user public keys
* `delete_repo`: Delete repositories

You need to
[register](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app)
a new GitHub App and [generate a private
key](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)
for the app by following the linked guides. The GitHub App is used for
authenticating as an app when cloning and pushing the git repository. The app
should be granted the following repository permissions:
* `Contents`: Read and Write access 

[Install](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app)
the app in the organization/account. Get the following information:
* Get the App ID from the app settings page at
  `https://github.com/settings/apps/<app-name>`. 
* Get the App Installation ID from the app installations page at
`https://github.com/settings/installations`. Click the installed app, the URL
will contain the installation ID
`https://github.com/settings/installations/<installation-id>`. For
organizations, the first part of the URL may be different, but it follows the
same pattern.
* The private key that was generated in a previous step.

Specify the token, username, org name, github app id, github installation id and
private key as environment variables for the script. The private key can be
stored in a file and read into the environment variable. Please make sure that
the org already exists as it won't be created by the script itself.

```shell
GO_TEST_PREFIX='TestGitHubE2E' GITHUB_USER='***' GITHUB_ORG='***' GITHUB_TOKEN='***' GHAPP_ID='***' GHAPP_INSTALL_ID='***' GHAPP_PRIVATE_KEY=`cat private-key-file.pem` ./run.sh 
```

### GitLab

You need to create an access token associated with your account. You can do so by following this
[guide](https://docs.gitlab.com/ee/user/project/settings/project_access_tokens.html).
The token should have the following permisssion scopes:
* `api`
* `read_api`
* `read_repository`
* `write_repository`

Specify the token, username and group name as environment variables for the script. Please make sure that the
group already exists as it won't be created by the script itself.

```shell
GO_TEST_PREFIX='TestGitLabE2E' GITLAB_USER='***' GITLAB_GROUP='***' GITLAB_PAT='***' ./run.sh 
```

### Bitbucket Server

You need to create an HTTP Access Token associated with your account. You can do so by following this
[guide](https://confluence.atlassian.com/bitbucketserver/personal-access-tokens-939515499.html).
The token should have the following permission scopes:
* Project permissions: `admin`
* Repository permissions: `admin`

Specify the token, username, project key and the domain where your server can be reached as
environment variables for the script. Please make sure that the project already exists as it
won't be created by the script itself.

```shell
GO_TEST_PREFIX='TestBitbucketServerE2E' STASH_USER='***' STASH_TOKEN='***' STASH_DOMAIN='***' STASH_PROJECT_KEY='***' ./run.sh
```
