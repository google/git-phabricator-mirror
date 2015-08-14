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
package comment

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/repository"
	"strconv"
	"strings"
)

const notesRef = "refs/notes/devtools/discuss"

// Ref defines the git-notes ref that we expect to contain review requests.
var Ref = repository.NotesRef(notesRef)

type CommentRange struct {
	StartLine uint32 `json:"startLine"`
}

type CommentLocation struct {
	Commit string `json:"commit,omitempty"`
	Path   string `json:"path,omitempty"`
	// If the range is omitted, then the location represents an entire file.
	Range *CommentRange `json:"range,omitempty"`
}

// Comment represents a review comment, and can occur in any of the following contexts:
// 1. As a comment on an entire commit.
// 2. As a comment about a specific file in a commit.
// 3. As a comment about a specific line in a commit.
// 4. As a response to another comment.
type Comment struct {
	// Timestamp and Author are optimizations that allows us to display comment threads
	// without having to run git-blame over the notes object. This is done because
	// git-blame will become more and more expensive as the number of code reviews grows.
	Timestamp string `json:"timestamp,omitempty"`
	Author    string `json:"author,omitempty"`
	// If parent is provided, then the comment is a response to another comment.
	Parent string `json:"parent,omitempty"`
	// If location is provided, then the comment is specific to that given location.
	Location    *CommentLocation `json:"location,omitempty"`
	Description string           `json:"description,omitempty"`
	// The resolved bit indicates whether to accept or reject the review. If unset,
	// it indicates that the comment is only an FYI.
	Resolved *bool `json:"resolved,omitempty"`
}

// CommentMap is a map of comments indexed by their hashes.
type CommentMap map[string]Comment

// Parse parses a review comment from a git note.
func Parse(note repository.Note) (Comment, error) {
	bytes := []byte(note)
	var comment Comment
	err := json.Unmarshal(bytes, &comment)
	return comment, err
}

// ParseAllValid takes collection of git notes and tries to parse a review
// comment from each one. Any notes that are not valid review comments get
// ignored, as we expect the git notes to be a heterogenous list, with only
// some of them being review comments.
func ParseAllValid(notes []repository.Note) CommentMap {
	comments := make(map[string]Comment)
	for _, note := range notes {
		comment, err := Parse(note)
		if err == nil {
			hash, err := comment.Hash()
			if err == nil {
				comments[hash] = comment
			}
		}
	}
	return comments
}

func (comment Comment) serialize() ([]byte, error) {
	if len(comment.Timestamp) < 10 {
		// To make sure that timestamps from before 2001 appear in the correct
		// alphabetical order, we reformat the timestamp to be at least 10 characters
		// and zero-padded.
		time, err := strconv.ParseInt(comment.Timestamp, 10, 64)
		if err == nil {
			comment.Timestamp = fmt.Sprintf("%010d", time)
		}
		// We ignore the other case, as the comment timestamp is not in a format
		// we expected, so we should just leave it alone.
	}
	return json.Marshal(comment)
}

// Write writes a review comment as a JSON-formatted git note.
func (comment Comment) Write() (repository.Note, error) {
	bytes, err := comment.serialize()
	return repository.Note(bytes), err
}

// Hash returns the SHA1 hash of a review comment.
func (comment Comment) Hash() (string, error) {
	bytes, err := comment.serialize()
	return fmt.Sprintf("%x", sha1.Sum(bytes)), err
}

// QuoteDescription generates the description that quotes the given comment.
//
// This is for when one user (such as our mirroring bot) needs to post a comment
// on behalf of another user.
func (comment Comment) QuoteDescription() string {
	return comment.Author + ":\n\n" + comment.Description
}

// isQuote determines if the given comment is a quote of the other comment.
//
// For these purposes, a quote is a sequence of:
// 1. The comment's author
// 2. A separator composed of a ':' and two newlines
// 3. The quoted comment's description.
func (comment Comment) isQuote(other Comment) bool {
	if comment.Description == other.QuoteDescription() {
		return true
	}
	if comment.Description == strings.Replace(other.QuoteDescription(), "\n", "\\n", -1) {
		return true
	}
	if strings.Replace(comment.Description, "\n", "\\n", -1) == other.QuoteDescription() {
		return true
	}
	return false
}

// descriptionOverlaps determines if two comment descriptions are roughly the same.
//
// Here, rough equivalence means that the two descriptions are the same, or that one
// is a quote of the other posted on behalf of another user.
func (comment Comment) descriptionOverlaps(other Comment) bool {
	if comment.Description == other.Description {
		return true
	}
	if comment.isQuote(other) {
		return true
	}
	return other.isQuote(comment)
}

// Overlaps compares two comment locations to see if they are the same.
func (location CommentLocation) Overlaps(other CommentLocation) bool {
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
func (comment Comment) Overlaps(other Comment) bool {
	if !comment.descriptionOverlaps(other) {
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
	if comment.Location.Overlaps(*other.Location) {
		return true
	}
	return false
}

// FilterOverlapping takes a slice of comments to exclude, and then returns
// a slice of comments, from the comment map, that do not overlap with the
// comments to exclude.
func (comments CommentMap) FilterOverlapping(exclude []Comment) []Comment {
	var filtered []Comment
	for _, c := range comments {
		passed := true
		for _, e := range exclude {
			if e.Overlaps(c) {
				passed = false
			}
		}
		if passed {
			filtered = append(filtered, c)
		}
	}
	return filtered
}
