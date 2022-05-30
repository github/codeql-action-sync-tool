package githubapiutil

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/google/go-github/v32/github"
)

func TestHasAnyScope(t *testing.T) {
	response := github.Response{
		Response: &http.Response{Header: http.Header{}},
	}

	response.Header.Set(xOAuthScopesHeader, "gist, notifications, admin:org, repo")
	require.True(t, HasAnyScope(&response, "public_repo", "repo"))

	response.Header.Set(xOAuthScopesHeader, "gist, notifications, public_repo, admin:org")
	require.True(t, HasAnyScope(&response, "public_repo", "repo"))

	response.Header.Set(xOAuthScopesHeader, "gist, notifications, admin:org")
	require.False(t, HasAnyScope(&response, "public_repo", "repo"))
}

func TestEnrichErrorResponse(t *testing.T) {
	response := github.Response{
		Response: &http.Response{Header: http.Header{}},
	}

	require.Equal(t, "The error message.: The underlying error.", EnrichResponseError(nil, errors.New("The underlying error."), "The error message.").Error())

	require.Equal(t, "The error message.: The underlying error.", EnrichResponseError(&response, errors.New("The underlying error."), "The error message.").Error())

	response.Header.Set(xGitHubRequestIDHeader, "AAAA:BBBB:CCCCCCC:DDDDDDD:EEEEEEEE")
	require.Equal(t, "The error message. (AAAA:BBBB:CCCCCCC:DDDDDDD:EEEEEEEE): The underlying error.", EnrichResponseError(&response, errors.New("The underlying error."), "The error message.").Error())
}
