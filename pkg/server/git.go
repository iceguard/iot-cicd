package server

import (
	"io"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	gitplumbing "gopkg.in/src-d/go-git.v4/plumbing"
	"k8s.io/klog"
)

// extractCommitID takes the URL and extracts the commit id
func extractCommitID(url *url.URL) string {
	var commitID string
	if url.Path[1:] != "build" {
		commitID = strings.Replace(url.Path[1:], "build/", "", 1)
	}

	return commitID
}

// cloneRepository fetches the given git repository and checks out the specified commitID.
// The clone output will be written to the given writer
// If no commitID is given, the default branch is checked out
func cloneRepository(repoURL, commitID string, w io.Writer) (repoPath string, err error) {
	repoPath, err = ioutil.TempDir("", "iot-cicd")
	if err != nil {
		return "", err
	}

	klog.Infof("Cloning repo %v to %v", repoURL, repoPath)
	g, err := git.PlainClone(repoPath, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: w,
	})
	if err != nil {
		return "", errors.Wrapf(err, "Error cloning module %v", repoURL)
	}

	err = checkout(commitID, g)
	return repoPath, errors.Wrapf(err, "Error checking out commit %v", commitID)
}

// checkout the given git repository on a specific commit.
// if the commitID is an empty string, leave the git repository
// at the default branch
func checkout(commitID string, g *git.Repository) error {
	if commitID == "" {
		return nil
	}

	klog.V(1).Infof("checking out commit id %v", commitID)
	wt, err := g.Worktree()
	if err != nil {
		return errors.Wrapf(err, "Error extracting worktree")
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Hash: gitplumbing.NewHash(commitID),
	})
	if err != nil {
		return errors.Wrapf(err, "Error checking out commid id %v", commitID)
	}
	return nil
}
