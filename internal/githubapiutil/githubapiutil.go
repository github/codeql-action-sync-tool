package githubapiutil

import (
	"strings"

	"github.com/google/go-github/v32/github"
)

const xOAuthScopesHeader = "X-OAuth-Scopes"

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
