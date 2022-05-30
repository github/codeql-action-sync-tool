package githubapiutil

import (
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/pkg/errors"
)

const xOAuthScopesHeader = "X-OAuth-Scopes"
const xGitHubRequestIDHeader = "X-GitHub-Request-ID"

func HasAnyScope(response *github.Response, scopes ...string) bool {
	if response == nil {
		return false
	}
	if len(response.Header.Values(xOAuthScopesHeader)) == 0 {
		return false
	}
	actualScopes := strings.Split(response.Header.Get(xOAuthScopesHeader), ",")
	for _, actualScope := range actualScopes {
		actualScope = strings.Trim(actualScope, " ")
		for _, requiredScope := range scopes {
			if actualScope == requiredScope {
				return true
			}
		}
	}
	return false
}

func EnrichResponseError(response *github.Response, err error, message string) error {
	requestID := ""
	if response != nil {
		requestID = response.Header.Get(xGitHubRequestIDHeader)
	}
	if requestID != "" {
		message = message + " (" + requestID + ")"
	}
	return errors.Wrap(err, message)
}
