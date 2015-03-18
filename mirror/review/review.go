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
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/repository"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/comment"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/request"
)

// Review represents a review tool's concept of a code review.
type Review interface {
	// LoadComments returns the comments for a review
	LoadComments() []comment.Comment

	// GetFirstCommit returns the first commit that is included in the review
	GetFirstCommit(repo repository.Repo) *repository.Revision
}

// Tool represents a code review tool.
//
// For example, this can be used to wrap Phabricator's "arcanist" command-line tool.
type Tool interface {
	// EnsureRequestExists creates a code review for the given request, if one does not already exist.
	EnsureRequestExists(repo repository.Repo, revision repository.Revision, req request.Request, comments map[string]comment.Comment)

	// ListOpenReviews returns the list of reviews that the tool knows about that have not yet been closed.
	ListOpenReviews(repo repository.Repo) []Review

	// Refresh advises the review tool that the code being reviewed has changed, and to reload it.
	Refresh(repo repository.Repo)
}
