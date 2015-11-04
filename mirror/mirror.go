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

// Package mirror provides two-way synchronization of metadata stored in git-notes and Phabricator.
package mirror

import (
	"github.com/google/git-appraise/repository"
	"github.com/google/git-appraise/review"
	"github.com/google/git-appraise/review/comment"
	"github.com/google/git-phabricator-mirror/mirror/arcanist"
	review_utils "github.com/google/git-phabricator-mirror/mirror/review"
	"log"
)

var arc = arcanist.Arcanist{}

// processedStates is used to keep track of the state of each repository at the last time we processed it.
// That, in turn, is used to avoid re-processing a repo if its state has not changed.
var processedStates = make(map[string]string)
var existingComments = make(map[string][]review.CommentThread)
var openReviews = make(map[string][]review_utils.PhabricatorReview)

func hasOverlap(newComment comment.Comment, existingComments []review.CommentThread) bool {
	for _, existing := range existingComments {
		if review_utils.Overlaps(newComment, existing.Comment) {
			return true
		} else if hasOverlap(newComment, existing.Children) {
			return true
		}
	}
	return false
}

func mirrorRepoToReview(repo repository.Repo, tool review_utils.Tool, syncToRemote bool) {
	if syncToRemote {
		repo.PullNotes("origin", "refs/notes/devtools/*")
	}

	stateHash := repo.GetRepoStateHash()
	if processedStates[repo.GetPath()] != stateHash {
		log.Print("Mirroring repo: ", repo)
		for _, r := range review.ListAll(repo) {
			existingComments[r.Revision] = r.Comments
			tool.EnsureRequestExists(repo, r)
		}
		openReviews[repo.GetPath()] = tool.ListOpenReviews(repo)
		processedStates[repo.GetPath()] = stateHash
		tool.Refresh(repo)
	}
	for _, phabricatorReview := range openReviews[repo.GetPath()] {
		if reviewCommit := phabricatorReview.GetFirstCommit(repo); reviewCommit != "" {
			log.Println("Processing review: ", reviewCommit)
			revisionComments := existingComments[reviewCommit]
			log.Printf("Loaded %d comments for %v\n", len(revisionComments), reviewCommit)
			for _, c := range phabricatorReview.LoadComments() {
				if !hasOverlap(c, revisionComments) {
					// The comment is new.
					note, err := c.Write()
					if err != nil {
						log.Fatal(err)
					}
					log.Printf("Appending a comment: %s", string(note))
					repo.AppendNote(comment.Ref, reviewCommit, note)
				} else {
					log.Printf("Skipping '%v', as it has already been written\n", c)
				}
			}
		}
	}
	if syncToRemote {
		if err := repo.PushNotes("origin", "refs/notes/devtools/*"); err != nil {
			log.Printf("Failed to push updates to the repo %v: %v\n", repo, err)
		}
	}
}

// Repo mirrors the given repository using the system-wide installation of
// the "arcanist" command line tool.
func Repo(repo repository.Repo, syncToRemote bool) {
	mirrorRepoToReview(repo, arc, syncToRemote)
}
