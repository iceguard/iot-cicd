package server

import (
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	gitplumbing "gopkg.in/src-d/go-git.v4/plumbing"
	"k8s.io/klog"
)

const (
	refRemoteBranchPrefix = "refs/remotes/origin/"
)

type gitRepo struct {
	// remote url for the repository to clone
	url string
	// the path where the repository is located on disk
	path string
	// commitID to check out
	commitID string
	// the branchname that will be detected from the commit Id
	branchName string
	// function that removes the repository from the disk
	cleanup func() error
	// the actual repository object
	r *git.Repository
}

// extractCommitID takes the URL and extracts the commit id
func extractCommitID(url *url.URL) string {
	var commitID string
	if url.Path[1:] != "build" {
		commitID = strings.Replace(url.Path[1:], "build/", "", 1)
	}

	return commitID
}

// setupRepo creates a temp dir and populates an instance of gitRepo type with
// all fields
func setupRepo(url, commitID string, w io.Writer) (repo *gitRepo, err error) {
	repo = &gitRepo{
		url:      url,
		commitID: commitID,
	}
	repo.path, err = ioutil.TempDir("", "iot-cicd")
	repo.cleanup = func() error {
		return os.RemoveAll(repo.path)
	}

	if err != nil {
		return repo, errors.Wrapf(err, "error creating subdirectory")
	}

	err = repo.clone(w)
	if err != nil {
		return repo, errors.Wrapf(err, "error cloning repository")
	}

	err = repo.checkout()
	if err != nil {
		return repo, errors.Wrapf(err, "error checking out commit %v"+repo.commitID)
	}

	repo.branchName, err = repo.branch()
	return repo, errors.Wrapf(err, "error determining branch name")
}

// branch returns the branch belonging to the currently
// checked out git commit
func (repo *gitRepo) branch() (string, error) {
	var branchName string
	head, err := repo.r.Head()
	if err != nil {
		return "", err
	}

	// Iterate over all references in the repository to see which point to the same commit as HEAD
	refs, err := repo.r.Storer.IterReferences()
	if err != nil {
		return "", err
	}
	err = refs.ForEach(func(ref *gitplumbing.Reference) error {
		if ref.Hash() == head.Hash() {
			name := ref.Strings()
			if strings.Contains(name[0], refRemoteBranchPrefix) {
				branchName = strings.ReplaceAll(name[0], refRemoteBranchPrefix, "")
			}
		}
		return nil
	})
	return branchName, err
}

// clone a repository to the path set in the struct.
func (repo *gitRepo) clone(w io.Writer) error {
	var err error
	klog.Infof("Cloning repo %v to %v", repo.url, repo.path)
	repo.r, err = git.PlainClone(repo.path, false, &git.CloneOptions{
		URL:      repo.url,
		Progress: w,
	})
	return errors.Wrapf(err, "Error cloning repository %v", repo)
}

// checkout the given git repository on a specific commit.
// if the commitID is an empty string, leave the git repository
// at the default branch
func (repo *gitRepo) checkout() error {
	if repo.commitID == "" {
		return nil
	}

	klog.V(1).Infof("checking out commit id %v", repo.commitID)
	wt, err := repo.r.Worktree()
	if err != nil {
		return errors.Wrapf(err, "Error extracting worktree")
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Hash: gitplumbing.NewHash(repo.commitID),
	})
	if err != nil {
		return errors.Wrapf(err, "Error checking out commid id %v", repo.commitID)
	}
	return nil
}
