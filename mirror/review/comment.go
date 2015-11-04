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

// Package comment defines the internal representation of a review comment.
package review

import (
	"github.com/google/git-appraise/review"
	"github.com/google/git-appraise/review/comment"
	"strings"
)

// CommentMap is a map of comments indexed by their hashes.
type CommentMap map[string]comment.Comment

// QuoteDescription generates the description that quotes the given comment.
//
// This is for when one user (such as our mirroring bot) needs to post a comment
// on behalf of another user.
func QuoteDescription(comment comment.Comment) string {
	return comment.Author + ":\n\n" + comment.Description
}

// isQuote determines if the given comment is a quote of the other comment.
//
// For these purposes, a quote is a sequence of:
// 1. The comment's author
// 2. A separator composed of a ':' and two newlines
// 3. The quoted comment's description.
func isQuote(comment, other comment.Comment) bool {
	if comment.Description == QuoteDescription(other) {
		return true
	}
	if comment.Description == strings.Replace(QuoteDescription(other), "\n", "\\n", -1) {
		return true
	}
	if strings.Replace(comment.Description, "\n", "\\n", -1) == QuoteDescription(other) {
		return true
	}
	return false
}

// descriptionOverlaps determines if two comment descriptions are roughly the same.
//
// Here, rough equivalence means that the two descriptions are the same, or that one
// is a quote of the other posted on behalf of another user.
func descriptionOverlaps(comment, other comment.Comment) bool {
	if comment.Description == other.Description {
		return true
	}
	if isQuote(comment, other) {
		return true
	}
	return isQuote(other, comment)
}

// Overlaps compares two comment locations to see if they are the same.
func LocationOverlaps(location, other comment.Location) bool {
	if location.Commit != other.Commit {
		return false
	}
	if location.Path != other.Path {
		return false
	}

	if location.Range == nil && other.Range == nil {
		return true
	}
	if location.Range == nil || other.Range == nil {
		return false
	}
	return location.Range.StartLine == other.Range.StartLine
}

// Overlaps compares two comments to see if they are roughly the same.
//
// This is necessary because the internal data models used by Phabricator and
// git-notes do not have an exact match, so we have to introduce a bit of a fudge-factor.
//
// We define overlap to mean that two comments are anchored at the same location,
// and that the two descriptions are either identical, or one is a quote of the other
// and if their resolved bits are unset or set but with the same timestamp and have the same value
func Overlaps(comment, other comment.Comment) bool {
	if !descriptionOverlaps(comment, other) {
		return false
	}
	if (comment.Resolved != nil && other.Resolved == nil) ||
		(comment.Resolved == nil && other.Resolved != nil) {
		return false
	}

	if comment.Resolved != nil && other.Resolved != nil {
		if (*comment.Resolved != *other.Resolved) ||
			(comment.Timestamp != other.Timestamp) {
			return false
		}
	}

	if comment.Location == nil && other.Location == nil {
		return true
	}
	if comment.Location == nil || other.Location == nil {
		return false
	}
	if LocationOverlaps(*comment.Location, *other.Location) {
		return true
	}
	return false
}

func (comments CommentMap) AddThreads(threads []review.CommentThread) {
	for _, thread := range threads {
		comments[thread.Hash] = thread.Comment
		comments.AddThreads(thread.Children)
	}
}

// FilterOverlapping takes a slice of comments to exclude, and then returns
// a slice of comments, from the comment map, that do not overlap with the
// comments to exclude.
func (comments CommentMap) FilterOverlapping(exclude []comment.Comment) []comment.Comment {
	var filtered []comment.Comment
	for _, c := range comments {
		passed := true
		for _, e := range exclude {
			if Overlaps(e, c) {
				passed = false
			}
		}
		if passed {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func FilterOverlapping(threads []review.CommentThread, exclude []comment.Comment) []comment.Comment {
	comments := CommentMap(make(map[string]comment.Comment))
	comments.AddThreads(threads)
	return comments.FilterOverlapping(exclude)
}
