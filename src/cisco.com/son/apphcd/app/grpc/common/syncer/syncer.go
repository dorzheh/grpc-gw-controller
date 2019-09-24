// Author  <dorzheho@cisco.com>

package syncer

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

const (
	errEmptyRepo         = "remote repository is empty"
	errReferenceNotFound = "reference not found"
	errAlreadyUpToDate   = "already up-to-date"
	defaultRemoteName    = "origin"
)

// Clone creates local charts cache
func Clone(protocol, username, password, ip, repo, targetDir, branchName string) error {

	url := fmt.Sprintf("%s://%s:%s@%s/%s/%s", protocol, username, password, ip,username, repo)

	logrus.WithFields(logrus.Fields{"url": url, "target": targetDir}).Debug("Cloning repository")
	r, err := git.PlainClone(targetDir, false,
		&git.CloneOptions{
			URL:               url,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		})

	if err != nil {
		if err.Error() == errEmptyRepo {
			return nil
		}
		return err
	}

	logrus.WithFields(logrus.Fields{"name": branchName}).Debug("Checking out branch")
	if _, err := checkout(r, branchName, true); err != nil {
		return err
	}

	return nil
}

// Pull updates local cache
func Pull(repoPath, branchName string) error {

	logrus.WithFields(logrus.Fields{"path": repoPath}).Debug("Finding repository path")

	// Open local repo
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		logrus.Error(repoPath)
		return err
	}

	// Checkout branch
	logrus.WithFields(logrus.Fields{"name": branchName}).Debug("Checking out branch")

	w, err := checkout(r, branchName, true)
	if err != nil {
		switch err.Error() {
		case errReferenceNotFound:
			return nil
		case errEmptyRepo:
			err = nil
		}
	}

	logrus.WithFields(logrus.Fields{"remote": defaultRemoteName}).Debug("Pulling changes")

	// Pull the branch delta
	if err := w.Pull(&git.PullOptions{RemoteName: defaultRemoteName, Force: true}); err != nil {
		if err.Error() == errAlreadyUpToDate {
			return nil
		}
	}

	return err
}

// Check out appropriate branch
func checkout(wt *git.Repository, branchName string, force bool) (*git.Worktree, error) {
	w, err := wt.Worktree()
	if err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{"name": branchName}).Debug("Checking out branch")

	return w, w.Checkout(&git.CheckoutOptions{
		Hash:  plumbing.NewHash(branchName),
		Force: force,
	})
}

// Push pushes the changes
func Push(repoPath, description string) error {

	logrus.WithFields(logrus.Fields{"repoPath": repoPath}).Debug("Opening repository")

	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"repoPath": repoPath}).Debug("Adding changes")

	if err := w.AddGlob("."); err != nil {
		return err
	}

	logrus.Debug("Committing changes")

	// Commits the current staging are to the repository, with the new file
	// just created. We should provide the object.Signature of Author of the commit.
	h, err := w.Commit(description, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  "catalog",
			Email: "catalog@cisco.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"repository": h.String()}).Debug("Pushing changes")

	return r.Push(&git.PushOptions{
		RemoteName: defaultRemoteName,
	})
}
