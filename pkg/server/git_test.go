package server

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitBranch(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		commitID   string
		branchName string
	}{
		{
			"testing-branch",
			"https://github.com/iceguard/iot-cicd.git",
			"111ba9ac487bb5696975fc45c8618277b8acdf13",
			"testing-branch",
		},
		{
			"master-with-id",
			"https://github.com/iceguard/iot-cicd.git",
			"4c5916825d3e8d63d6cc866e74ce22a5b3ee384a",
			"master",
		},
		{
			"master-no-id",
			"https://github.com/iceguard/iot-cicd.git",
			"",
			"master",
		},
	}

	var b bytes.Buffer
	buf := bufio.NewWriter(&b)
	for _, tt := range tests {
		t.Run(tt.branchName, func(t *testing.T) {
			t.Parallel()
			repo, err := setupRepo(tt.url, tt.commitID, buf)
			assert.Nil(t, err)
			branchName, err := repo.branch()
			assert.Equal(t, tt.branchName, branchName)
			assert.Nil(t, err)
		})
	}
}
