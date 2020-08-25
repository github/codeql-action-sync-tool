package githubapiutil

import (
	"strings"

	"github.com/google/go-github/v32/github"
)

const xOAuthScopesHeader = "X-OAuth-Scopes"

func MissingAllScopes(response *github.Response, requiredAnyScopes ...string) bool {
	if response == nil {
		return false
	}
	if len(response.Header.Values(xOAuthScopesHeader)) == 0 {
		return false
	}
	actualScopes := strings.Split(response.Header.Get(xOAuthScopesHeader), ",")
	for _, actualScope := range actualScopes {
		actualScope = strings.Trim(actualScope, " ")
		for _, requiredAnyScope := range requiredAnyScopes {
			if actualScope == requiredAnyScope {
				return false
			}
		}
	}
	return true
}
