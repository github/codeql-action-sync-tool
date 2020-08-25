package githubapiutil

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/google/go-github/v32/github"
)

func TestHasAnyScopes(t *testing.T) {
	response := github.Response{
		Response: &http.Response{Header: http.Header{}},
	}

	response.Header.Set(xOAuthScopesHeader, "gist, notifications, admin:org, repo")
	require.False(t, MissingAllScopes(&response, "public_repo", "repo"))

	response.Header.Set(xOAuthScopesHeader, "gist, notifications, public_repo, admin:org")
	require.False(t, MissingAllScopes(&response, "public_repo", "repo"))

	response.Header.Set(xOAuthScopesHeader, "gist, notifications, admin:org")
	require.True(t, MissingAllScopes(&response, "public_repo", "repo"))
}
