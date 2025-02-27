// +build e2e

package e2e_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	resource "github.com/telia-oss/github-pr-resource"
)

var (
	targetCommitID       = "a5114f6ab89f4b736655642a11e8d15ce363d882"
	targetPullRequestID  = "4"
	targetDateTime       = time.Date(2018, time.May, 11, 8, 43, 48, 0, time.UTC)
	latestCommitID       = "890a7e4f0d5b05bda8ea21b91f4604e3e0313581"
	latestPullRequestID  = "5"
	latestDateTime       = time.Date(2018, time.May, 14, 10, 51, 58, 0, time.UTC)
	developCommitID      = "ac771f3b69cbd63b22bbda553f827ab36150c640"
	developPullRequestID = "6"
	developDateTime      = time.Date(2018, time.September, 25, 21, 00, 16, 0, time.UTC)
)

func TestCheckE2E(t *testing.T) {

	tests := []struct {
		description string
		source      resource.Source
		version     resource.Version
		expected    resource.CheckResponse
	}{
		{
			description: "check returns the latest version if there is no previous",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
			},
			version: resource.Version{},
			expected: resource.CheckResponse{
				resource.Version{PR: latestPullRequestID, Commit: latestCommitID, CommittedDate: latestDateTime},
			},
		},

		{
			description: "check returns the previous version when its still latest",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
			},
			version: resource.Version{PR: latestPullRequestID, Commit: latestCommitID, CommittedDate: latestDateTime},
			expected: resource.CheckResponse{
				resource.Version{PR: latestPullRequestID, Commit: latestCommitID, CommittedDate: latestDateTime},
			},
		},

		{
			description: "check returns all new versions since the last",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
			},
			version: resource.Version{PR: targetPullRequestID, Commit: targetCommitID, CommittedDate: targetDateTime},
			expected: resource.CheckResponse{
				resource.Version{PR: latestPullRequestID, Commit: latestCommitID, CommittedDate: latestDateTime},
			},
		},

		{
			description: "check will only return versions that match the specified paths",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
				Paths:       []string{"*.md"},
			},
			version: resource.Version{},
			expected: resource.CheckResponse{
				resource.Version{PR: targetPullRequestID, Commit: targetCommitID, CommittedDate: targetDateTime},
			},
		},

		{
			description: "check will skip versions which only match the ignore paths",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
				IgnorePaths: []string{"*.txt"},
			},
			version: resource.Version{},
			expected: resource.CheckResponse{
				resource.Version{PR: targetPullRequestID, Commit: targetCommitID, CommittedDate: targetDateTime},
			},
		},

		{
			description: "check works with custom endpoints",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
				V3Endpoint:  "https://api.github.com/",
				V4Endpoint:  "https://api.github.com/graphql",
			},
			version: resource.Version{},
			expected: resource.CheckResponse{
				resource.Version{PR: latestPullRequestID, Commit: latestCommitID, CommittedDate: latestDateTime},
			},
		},

		{
			description: "check works with custom base branch",
			source: resource.Source{
				Repository:    "itsdalmo/test-repository",
				AccessToken:   os.Getenv("GITHUB_ACCESS_TOKEN"),
				V3Endpoint:    "https://api.github.com/",
				V4Endpoint:    "https://api.github.com/graphql",
				BaseBranch:    "develop",
				DisableCISkip: true,
			},
			version: resource.Version{},
			expected: resource.CheckResponse{
				resource.Version{PR: developPullRequestID, Commit: developCommitID, CommittedDate: developDateTime},
			},
		},

		{
			description: "check works with required review approvals",
			source: resource.Source{
				Repository:              "itsdalmo/test-repository",
				AccessToken:             os.Getenv("GITHUB_ACCESS_TOKEN"),
				V3Endpoint:              "https://api.github.com/",
				V4Endpoint:              "https://api.github.com/graphql",
				RequiredReviewApprovals: 1,
			},
			version: resource.Version{},
			expected: resource.CheckResponse{
				resource.Version{PR: targetPullRequestID, Commit: targetCommitID, CommittedDate: targetDateTime},
			},
		},

		{
			description: "check works when we require multiple review approvals",
			source: resource.Source{
				Repository:              "itsdalmo/test-repository",
				AccessToken:             os.Getenv("GITHUB_ACCESS_TOKEN"),
				V3Endpoint:              "https://api.github.com/",
				V4Endpoint:              "https://api.github.com/graphql",
				RequiredReviewApprovals: 2,
			},
			version:  resource.Version{},
			expected: resource.CheckResponse(nil),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			github, err := resource.NewGithubClient(&tc.source)
			require.NoError(t, err)

			input := resource.CheckRequest{Source: tc.source, Version: tc.version}
			output, err := resource.Check(input, github)

			if assert.NoError(t, err) {
				assert.Equal(t, tc.expected, output)
			}
		})
	}
}

func TestGetAndPutE2E(t *testing.T) {
	tests := []struct {
		description         string
		source              resource.Source
		version             resource.Version
		getParameters       resource.GetParameters
		putParameters       resource.PutParameters
		versionString       string
		metadataString      string
		filesString         string
		metadataFiles       map[string]string
		expectedCommitCount int
		expectedCommits     []string
	}{
		{
			description: "get and put works",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				V3Endpoint:  "https://api.github.com/",
				V4Endpoint:  "https://api.github.com/graphql",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
			},
			version: resource.Version{
				PR:            targetPullRequestID,
				Commit:        targetCommitID,
				CommittedDate: time.Time{},
			},
			getParameters:  resource.GetParameters{},
			putParameters:  resource.PutParameters{},
			versionString:  `{"pr":"4","commit":"a5114f6ab89f4b736655642a11e8d15ce363d882","committed":"0001-01-01T00:00:00Z"}`,
			metadataString: `[{"name":"pr","value":"4"},{"name":"url","value":"https://github.com/itsdalmo/test-repository/pull/4"},{"name":"head_name","value":"my_second_pull"},{"name":"head_sha","value":"a5114f6ab89f4b736655642a11e8d15ce363d882"},{"name":"base_name","value":"master"},{"name":"base_sha","value":"93eeeedb8a16e6662062d1eca5655108977cc59a"},{"name":"message","value":"Push 2."},{"name":"author","value":"itsdalmo"}]`,
			metadataFiles: map[string]string{
				"pr":        "4",
				"url":       "https://github.com/itsdalmo/test-repository/pull/4",
				"head_name": "my_second_pull",
				"head_sha":  "a5114f6ab89f4b736655642a11e8d15ce363d882",
				"base_name": "master",
				"base_sha":  "93eeeedb8a16e6662062d1eca5655108977cc59a",
				"message":   "Push 2.",
				"author":    "itsdalmo",
			},
			expectedCommitCount: 10,
			expectedCommits:     []string{"Merge commit 'a5114f6ab89f4b736655642a11e8d15ce363d882'"},
		},
		{
			description: "get works when rebasing",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				V3Endpoint:  "https://api.github.com/",
				V4Endpoint:  "https://api.github.com/graphql",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
			},
			version: resource.Version{
				PR:            targetPullRequestID,
				Commit:        targetCommitID,
				CommittedDate: time.Time{},
			},
			getParameters: resource.GetParameters{
				IntegrationTool: "rebase",
			},
			putParameters:       resource.PutParameters{},
			versionString:       `{"pr":"4","commit":"a5114f6ab89f4b736655642a11e8d15ce363d882","committed":"0001-01-01T00:00:00Z"}`,
			metadataString:      `[{"name":"pr","value":"4"},{"name":"url","value":"https://github.com/itsdalmo/test-repository/pull/4"},{"name":"head_name","value":"my_second_pull"},{"name":"head_sha","value":"a5114f6ab89f4b736655642a11e8d15ce363d882"},{"name":"base_name","value":"master"},{"name":"base_sha","value":"93eeeedb8a16e6662062d1eca5655108977cc59a"},{"name":"message","value":"Push 2."},{"name":"author","value":"itsdalmo"}]`,
			expectedCommitCount: 9,
			expectedCommits:     []string{"Push 2."},
		},
		{
			description: "get works when checkout",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				V3Endpoint:  "https://api.github.com/",
				V4Endpoint:  "https://api.github.com/graphql",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
			},
			version: resource.Version{
				PR:            targetPullRequestID,
				Commit:        targetCommitID,
				CommittedDate: time.Time{},
			},
			getParameters: resource.GetParameters{
				IntegrationTool: "checkout",
			},
			putParameters:       resource.PutParameters{},
			versionString:       `{"pr":"4","commit":"a5114f6ab89f4b736655642a11e8d15ce363d882","committed":"0001-01-01T00:00:00Z"}`,
			metadataString:      `[{"name":"pr","value":"4"},{"name":"url","value":"https://github.com/itsdalmo/test-repository/pull/4"},{"name":"head_name","value":"my_second_pull"},{"name":"head_sha","value":"a5114f6ab89f4b736655642a11e8d15ce363d882"},{"name":"base_name","value":"master"},{"name":"base_sha","value":"93eeeedb8a16e6662062d1eca5655108977cc59a"},{"name":"message","value":"Push 2."},{"name":"author","value":"itsdalmo"}]`,
			expectedCommitCount: 7,
			expectedCommits: []string{
				"Push 2.",
				"Push 1.",
				"Add another commit to the 2nd PR to verify concourse behaviour.",
				"Add another comment to 2nd pull request.",
				"Add comment from 2nd pull request.",
				"Add comment after creating first pull request.",
				"Initial commit",
			},
		},
		{
			description: "get works with non-master bases",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				V3Endpoint:  "https://api.github.com/",
				V4Endpoint:  "https://api.github.com/graphql",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
			},
			version: resource.Version{
				PR:            developPullRequestID,
				Commit:        developCommitID,
				CommittedDate: time.Time{},
			},
			getParameters:       resource.GetParameters{},
			putParameters:       resource.PutParameters{},
			versionString:       `{"pr":"6","commit":"ac771f3b69cbd63b22bbda553f827ab36150c640","committed":"0001-01-01T00:00:00Z"}`,
			metadataString:      `[{"name":"pr","value":"6"},{"name":"url","value":"https://github.com/itsdalmo/test-repository/pull/6"},{"name":"head_name","value":"test-develop-pr"},{"name":"head_sha","value":"ac771f3b69cbd63b22bbda553f827ab36150c640"},{"name":"base_name","value":"develop"},{"name":"base_sha","value":"93eeeedb8a16e6662062d1eca5655108977cc59a"},{"name":"message","value":"[skip ci] Add a PR with a non-master base"},{"name":"author","value":"itsdalmo"}]`,
			expectedCommitCount: 5,
			expectedCommits:     []string{"[skip ci] Add a PR with a non-master base"}, // This merge ends up being fast-forwarded
		},
		{
			description: "get works when ssl verification is disabled",
			source: resource.Source{
				Repository:          "itsdalmo/test-repository",
				V3Endpoint:          "https://api.github.com/",
				V4Endpoint:          "https://api.github.com/graphql",
				AccessToken:         os.Getenv("GITHUB_ACCESS_TOKEN"),
				SkipSSLVerification: true,
			},
			version: resource.Version{
				PR:            targetPullRequestID,
				Commit:        targetCommitID,
				CommittedDate: time.Time{},
			},
			getParameters:       resource.GetParameters{},
			putParameters:       resource.PutParameters{},
			versionString:       `{"pr":"4","commit":"a5114f6ab89f4b736655642a11e8d15ce363d882","committed":"0001-01-01T00:00:00Z"}`,
			metadataString:      `[{"name":"pr","value":"4"},{"name":"url","value":"https://github.com/itsdalmo/test-repository/pull/4"},{"name":"head_name","value":"my_second_pull"},{"name":"head_sha","value":"a5114f6ab89f4b736655642a11e8d15ce363d882"},{"name":"base_name","value":"master"},{"name":"base_sha","value":"93eeeedb8a16e6662062d1eca5655108977cc59a"},{"name":"message","value":"Push 2."},{"name":"author","value":"itsdalmo"}]`,
			expectedCommitCount: 10,
			expectedCommits:     []string{"Merge commit 'a5114f6ab89f4b736655642a11e8d15ce363d882'"},
		},
		{
			description: "get works with git_depth",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
			},
			version: resource.Version{
				PR:            targetPullRequestID,
				Commit:        targetCommitID,
				CommittedDate: time.Time{},
			},
			getParameters:       resource.GetParameters{GitDepth: 6},
			putParameters:       resource.PutParameters{},
			versionString:       `{"pr":"4","commit":"a5114f6ab89f4b736655642a11e8d15ce363d882","committed":"0001-01-01T00:00:00Z"}`,
			metadataString:      `[{"name":"pr","value":"4"},{"name":"url","value":"https://github.com/itsdalmo/test-repository/pull/4"},{"name":"head_name","value":"my_second_pull"},{"name":"head_sha","value":"a5114f6ab89f4b736655642a11e8d15ce363d882"},{"name":"base_name","value":"master"},{"name":"base_sha","value":"93eeeedb8a16e6662062d1eca5655108977cc59a"},{"name":"message","value":"Push 2."},{"name":"author","value":"itsdalmo"}]`,
			expectedCommitCount: 9,
			expectedCommits: []string{
				"Merge commit 'a5114f6ab89f4b736655642a11e8d15ce363d882'",
				"Push 2.",
				"Push 1.",
				"Add another commit to the 2nd PR to verify concourse behaviour.",
				"Add another file to test merge commit SHA.",
				"Add new file after creating 2nd PR.",
				"Add another comment to 2nd pull request.",
				"Add comment from 2nd pull request.",
				"Add comment after creating first pull request.",
			},
		},
		{
			description: "get works with list_changed_files",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
			},
			version: resource.Version{
				PR:            targetPullRequestID,
				Commit:        targetCommitID,
				CommittedDate: time.Time{},
			},
			getParameters: resource.GetParameters{
				ListChangedFiles: true,
			},
			putParameters:       resource.PutParameters{},
			versionString:       `{"pr":"4","commit":"a5114f6ab89f4b736655642a11e8d15ce363d882","committed":"0001-01-01T00:00:00Z"}`,
			metadataString:      `[{"name":"pr","value":"4"},{"name":"url","value":"https://github.com/itsdalmo/test-repository/pull/4"},{"name":"head_name","value":"my_second_pull"},{"name":"head_sha","value":"a5114f6ab89f4b736655642a11e8d15ce363d882"},{"name":"base_name","value":"master"},{"name":"base_sha","value":"93eeeedb8a16e6662062d1eca5655108977cc59a"},{"name":"message","value":"Push 2."},{"name":"author","value":"itsdalmo"}]`,
			filesString:         "README.md\ntest.txt\n",
			expectedCommitCount: 10,
			expectedCommits:     []string{"Merge commit 'a5114f6ab89f4b736655642a11e8d15ce363d882'"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			// Create temporary directory
			dir, err := ioutil.TempDir("", "github-pr-resource")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			github, err := resource.NewGithubClient(&tc.source)
			require.NoError(t, err)

			git, err := resource.NewGitClient(&tc.source, dir, ioutil.Discard)
			require.NoError(t, err)

			// Get (output and files)
			getRequest := resource.GetRequest{Source: tc.source, Version: tc.version, Params: tc.getParameters}
			getOutput, err := resource.Get(getRequest, github, git, dir)

			require.NoError(t, err)
			assert.Equal(t, tc.version, getOutput.Version)

			version := readTestFile(t, filepath.Join(dir, ".git", "resource", "version.json"))
			assert.Equal(t, tc.versionString, version)

			metadata := readTestFile(t, filepath.Join(dir, ".git", "resource", "metadata.json"))
			assert.Equal(t, tc.metadataString, metadata)

			if tc.getParameters.ListChangedFiles {
				changedFiles := readTestFile(t, filepath.Join(dir, ".git", "resource", "changed_files"))
				assert.Equal(t, tc.filesString, changedFiles)
			}

			for filename, expected := range tc.metadataFiles {
				actual := readTestFile(t, filepath.Join(dir, ".git", "resource", filename))
				assert.Equal(t, expected, actual)
			}

			// Check commit history
			history := gitHistory(t, dir)
			assert.Equal(t, tc.expectedCommitCount, len(history))

			// Loop over the expected commits - allows us to only care about the final commit.
			for i, expected := range tc.expectedCommits {
				actual, ok := history[i]
				if assert.True(t, ok) {
					assert.Equal(t, expected, actual)
				}
			}

			// Put
			putRequest := resource.PutRequest{Source: tc.source, Params: tc.putParameters}
			putOutput, err := resource.Put(putRequest, github, dir)

			require.NoError(t, err)
			assert.Equal(t, tc.version, putOutput.Version)
		})
	}
}

func gitHistory(t *testing.T, directory string) map[int]string {
	cmd := exec.Command("git", "log", "--oneline", "--pretty=format:%s")
	cmd.Dir = directory

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get git historys: %s", err)
	}

	m := strings.Split(string(output), "\n")
	h := make(map[int]string, len(m))
	for i, s := range m {
		h[i] = s
	}

	return h
}

func readTestFile(t *testing.T, path string) string {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read: %s: %s", path, err)
	}
	return string(b)
}
