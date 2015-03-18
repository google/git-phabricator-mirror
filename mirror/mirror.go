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
	"log"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/repository"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/arcanist"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/comment"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/request"
)

var arc = arcanist.Arcanist{}

// processedStates is used to keep track of the state of each repository at the last time we processed it.
// That, in turn, is used to avoid re-processing a repo if its state has not changed.
var processedStates = make(map[string]string)
var existingComments = make(map[repository.Revision]map[string]comment.Comment)
var openReviews = make(map[string][]review.Review)

func getExistingComments(revision repository.Revision) map[string]comment.Comment {
	revisionComments, ok := existingComments[revision]
	if !ok {
		revisionComments = make(map[string]comment.Comment)
		existingComments[revision] = revisionComments
	}
	return revisionComments
}

func hasOverlap(newComment comment.Comment, existingComments map[string]comment.Comment) bool {
	for _, existing := range existingComments {
		if newComment.Overlaps(existing) {
			return true
		}
	}
	return false
}

func mirrorRepoToReview(repo repository.Repo, tool review.Tool, syncToRemote bool) {
	if syncToRemote {
		if err := repo.PullUpdates(); err != nil {
			log.Printf("Failed to pull updates for the repo %v: %v\n", repo, err)
			return
		}
	}

	stateHash := repo.GetRepoStateHash()
	if processedStates[repo.GetPath()] != stateHash {
		log.Print("Mirroring repo: ", repo)
		for _, revision := range repo.ListNotedRevisions(request.Ref) {
			existingComments[revision] = comment.ParseAllValid(repo.GetNotes(comment.Ref, revision))
			for _, req := range request.ParseAllValid(repo.GetNotes(request.Ref, revision)) {
				tool.EnsureRequestExists(repo, revision, req, existingComments[revision])
			}
		}
		openReviews[repo.GetPath()] = tool.ListOpenReviews(repo)
		processedStates[repo.GetPath()] = stateHash
		tool.Refresh(repo)
	}
	for _, review := range openReviews[repo.GetPath()] {
		if reviewCommit := review.GetFirstCommit(repo); reviewCommit != nil {
			log.Println("Processing review: ", *reviewCommit)
			revisionComments := getExistingComments(*reviewCommit)
			log.Printf("Loaded %d comments for %v\n", len(revisionComments), *reviewCommit)
			for _, c := range review.LoadComments() {
				if !hasOverlap(c, revisionComments) {
					// The comment is new.
					note, err := c.Write()
					if err != nil {
						log.Fatal(err)
					}
					log.Printf("Appending a comment: %s", string(note))
					repo.AppendNote(comment.Ref, *reviewCommit, note, c.Author)
				} else {
					log.Printf("Skipping '%v', as it has already been written\n", c)
				}
			}
		}
	}
	if syncToRemote {
		if err := repo.PushUpdates(); err != nil {
			log.Printf("Failed to push updates to the repo %v: %v\n", repo, err)
		}
	}
}

// Repo mirrors the given repository using the system-wide installation of
// the "arcanist" command line tool.
func Repo(repo repository.Repo, syncToRemote bool) {
	mirrorRepoToReview(repo, arc, syncToRemote)
	// TODO: Mirror robot comments.
}
