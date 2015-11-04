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

// Package review provides tools for manipulating code reviews.
package review

import (
	"github.com/google/git-appraise/repository"
	"github.com/google/git-appraise/review"
	"github.com/google/git-appraise/review/comment"
)

// PhabricatorReview represents a code review stored in Phabricator.
type PhabricatorReview interface {
	// LoadComments returns the comments for a review
	LoadComments() []comment.Comment

	// GetFirstCommit returns the first commit that is included in the review
	GetFirstCommit(repo repository.Repo) string
}

// Tool represents our interface to the code review portion of Phabricator.
//
// The default implementation wraps calls to Phabricator's "arcanist" command-line tool.
type Tool interface {
	// EnsureRequestExists mirrors a review from git-notes into Phabricator.
	EnsureRequestExists(repo repository.Repo, review review.Review)

	// ListOpenReviews returns the list of reviews that the tool knows about that have not yet been closed.
	ListOpenReviews(repo repository.Repo) []PhabricatorReview

	// Refresh advises the review tool that the code being reviewed has changed, and to reload it.
	Refresh(repo repository.Repo)
}
