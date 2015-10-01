/*
Copyright 2015 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package repository

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Timeout used for all network operations involving a remote repo
const gitRemoteTimeout = 1 * time.Minute

// GitRepo represents an instance of a (local) git repository.
type GitRepo struct {
	Path string
}

func (repo GitRepo) runGitCommandWithEnvAndTimeout(timeout *time.Duration, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo.Path
	cmd.Env = env
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Start()
	if timeout != nil {
		go func() {
			time.Sleep(*timeout)
			cmd.Process.Kill()
		}()
	}
	if err := cmd.Wait(); err != nil {
		log.Printf("A git command failed: %q, %q, %q, %q\n", args, err, stdout.String(), stderr.String())
		return "", err
	}
	return strings.Trim(stdout.String(), "\n"), nil
}

func (repo GitRepo) runGitCommandWithEnv(env []string, args ...string) (string, error) {
	return repo.runGitCommandWithEnvAndTimeout(nil, env, args...)
}

func (repo GitRepo) runGitCommand(args ...string) (string, error) {
	return repo.runGitCommandWithEnv(nil, args...)
}

func (repo GitRepo) runGitCommandWithTimeout(timeout time.Duration, args ...string) (string, error) {
	return repo.runGitCommandWithEnvAndTimeout(&timeout, nil, args...)
}

func (repo GitRepo) runGitCommandWithEnvOrDie(env []string, args ...string) string {
	out, err := repo.runGitCommandWithEnv(env, args...)
	if err != nil {
		log.Print("git", args, out)
		log.Fatal(err)
	}
	return out
}

func (repo GitRepo) runGitCommandOrDie(args ...string) string {
	return repo.runGitCommandWithEnvOrDie(nil, args...)
}

func (repo GitRepo) runGitCommandAsUserOrDie(userEmail string, args ...string) string {
	env := append(
		os.Environ(),
		fmt.Sprintf("GIT_AUTHOR_NAME=%s", userEmail),
		fmt.Sprintf("GIT_AUTHOR_EMAIL=%s", userEmail),
		fmt.Sprintf("GIT_COMMITTER_NAME=%s", userEmail),
		fmt.Sprintf("GIT_COMMITTER_EMAIL=%s", userEmail))
	return repo.runGitCommandWithEnvOrDie(env, args...)
}

// IsGitRepo takes the given path and determins if it is inside of a git repository.
func IsGitRepo(path string) (bool, error) {
	// Note: we cannot reuse the runGitCommand method for this because this is not a
	// method on the GitRepo type, and we need to treat exec.ExitErrors specially.
	cmd := exec.Command("git", "rev-parse")
	cmd.Dir = path
	_, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, err
}

// GetPath returns the path to the repo.
func (repo GitRepo) GetPath() string {
	return repo.Path
}

// GetRepoStateHash returns a hash which embodies the entire current state of a repository.
func (repo GitRepo) GetRepoStateHash() string {
	// We assume that any errors in the following command simply mean that the repo is empty.
	stateSummary, _ := repo.runGitCommand("show-ref")
	return fmt.Sprintf("%x", sha1.Sum([]byte(stateSummary)))
}

// GetNotes uses the "git" command-line tool to read the notes from the given ref for a given revision.
func (repo GitRepo) GetNotes(ref NotesRef, revision Revision) []Note {
	var notes []Note
	rawNotes, err := repo.runGitCommand("notes", "--ref", string(ref), "show", string(revision))
	if err != nil {
		// This is expected when there are no notes for a given revision
		return notes
	}
	for _, line := range strings.Split(rawNotes, "\n") {
		notes = append(notes, Note([]byte(line)))
	}
	return notes
}

// AppendNote appends a note to a revision under the given ref.
func (repo GitRepo) AppendNote(ref NotesRef, revision Revision, note Note, authorEmail string) {
	repo.runGitCommandAsUserOrDie(authorEmail, "notes", "--ref", string(ref), "append", "-m", string(note), string(revision))
}

// ListNotedRevisions returns the collection of revisions that are annotated by notes in the given ref.
func (repo GitRepo) ListNotedRevisions(ref NotesRef) []Revision {
	var revisions []Revision
	notesList := strings.Split(repo.runGitCommandOrDie("notes", "--ref", string(ref), "list"), "\n")
	for _, notePair := range notesList {
		noteParts := strings.SplitN(notePair, " ", 2)
		if len(noteParts) == 2 {
			objHash := noteParts[1]
			objType, err := repo.runGitCommand("cat-file", "-t", objHash)
			// If a note points to an object that we do not know about (yet), then err will not
			// be nil. We can safely just ignore those notes.
			if err == nil && objType == "commit" {
				revisions = append(revisions, Revision(objHash))
			}
		}
	}
	return revisions
}

// GetMergeBase returns the latest revision that is a common ancestor of the two given revision.
//
// This is the revision that is used as the left-hand side for review diffs.
func (repo GitRepo) GetMergeBase(from, to Revision) (Revision, error) {
	mergeBaseCommit, err := repo.runGitCommand("merge-base", string(from), string(to))
	return Revision(mergeBaseCommit), err
}

// GetRawDiff returns the raw diff between two revisions.
func (repo GitRepo) GetRawDiff(from, to Revision) (string, error) {
	// Differential does not know how to display the context of changes (i.e. the
	// all of the surrounding lines) unless the raw diff includes the entire file.
	// To accommodate this, we pass the -U flag to make git put all of the changes
	// from the entire file into a single diff hunk.
	return repo.runGitCommand("diff", "-M", "--no-ext-diff", "--no-textconv",
		"--src-prefix=a/", "--dst-prefix=b/",
		fmt.Sprintf("-U%d", 0x7fff), "--no-color",
		fmt.Sprintf("%s..%s", string(from), string(to)))
}

// GetDetails returns the CommitDetails for the given revision.
func (repo GitRepo) GetDetails(revision Revision) (*CommitDetails, error) {
	var err error
	show := func(formatString string) (result string) {
		if err != nil {
			return ""
		}
		result, err = repo.runGitCommand("show", "-s", string(revision), fmt.Sprintf("--format=tformat:%s", formatString))
		return result
	}

	jsonFormatString := "{\"commit\": \"%H\", \"tree\":\"%T\", \"time\": \"%at\"}"
	detailsJSON := show(jsonFormatString)
	if err != nil {
		return nil, err
	}
	var details CommitDetails
	err = json.Unmarshal([]byte(detailsJSON), &details)
	if err != nil {
		log.Fatal(err)
	}
	details.Author = show("%an")
	details.AuthorEmail = show("%ae")
	details.Summary = show("%s")
	parentsString := show("%P")
	details.Parents = strings.Split(parentsString, " ")
	if err != nil {
		return nil, err
	}
	return &details, nil
}

func (repo GitRepo) PullUpdates() error {
	_, err := repo.runGitCommandWithTimeout(gitRemoteTimeout, "fetch", "origin", "+refs/*:refs/*")
	return err
}

func (repo GitRepo) PushUpdates() error {
	_, err := repo.runGitCommandWithTimeout(gitRemoteTimeout, "fetch", "origin", "+refs/notes/devtools/*:refs/notes/origin/devtools/*")
	if err != nil {
		return err
	}

	remoteNotes, err := repo.runGitCommandWithTimeout(gitRemoteTimeout, "ls-remote", "origin", "refs/notes/devtools/*")
	if err != nil {
		return err
	}

	// The resulting output from the ls-remote command is line separated, with each
	// line containing two components (a commit and the ref name) separated by tabs.
	for _, notesLine := range strings.Split(remoteNotes, "\n") {
		lineParts := strings.Split(notesLine, "\t")
		if len(lineParts) == 2 {
			notesRef := lineParts[1]
			remoteRef := strings.Replace(notesRef, "refs/notes/devtools", "refs/notes/origin/devtools", 1)
			_, err := repo.runGitCommand("notes", "--ref", notesRef, "merge", remoteRef, "-s", "cat_sort_uniq")
			if err != nil {
				return err
			}
		}
	}

	_, err = repo.runGitCommandWithTimeout(gitRemoteTimeout, "push", "origin", "refs/notes/devtools/*:refs/notes/devtools/*")
	return err
}
