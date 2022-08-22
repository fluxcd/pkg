# This script registers a personal authentication token (PAT) for the root user.
# This PAT is then used to make GitLab API calls.
user = User.find_by_username('root')
pat = user.personal_access_tokens.create(scopes: [:write_repository, :read_repository, :api], name: 'Flux E2E testing')
token = ARGV[0]
pat.set_token(token)
pat.save!
