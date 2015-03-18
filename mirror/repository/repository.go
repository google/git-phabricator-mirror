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

// Package repository provides types for representing source code repositories.
package repository

import (
	"crypto/sha1"
	"fmt"
)

// NotesRef represents a git ref that contains git notes.
type NotesRef string

// Revision represents git revisions.
type Revision string

// Note represents a series of bytes stored in git and used to annotate a git object.
type Note []byte

// CommitDetails represents the metadata for a specific commit.
type CommitDetails struct {
	Commit      string   `json:"commit,omitempty"`
	Author      string   `json:"author,omitempty"`
	AuthorEmail string   `json:"authorEmail,omitempty"`
	Tree        string   `json:"tree,omitempty"`
	Time        string   `json:"time,omitempty"`
	Parents     []string `json:"parents,omitempty"`
	Summary     string   `json:"summary,omitempty"`
}

// Repo represents a source code repository.
type Repo interface {
	// GetPath returns the path to the repo.
	GetPath() string

	// GetRepoStateHash returns a hash which embodies the entire current state of a repository.
	GetRepoStateHash() string

	// GetNotes reads the notes from the given ref that annotate the given revision.
	GetNotes(ref NotesRef, revision Revision) []Note

	// AppendNote appends a note to a revision under the given ref.
	AppendNote(ref NotesRef, revision Revision, note Note, authorEmail string)

	// ListNotedRevisions returns the collection of revisions that are annotated by notes in the given ref.
	ListNotedRevisions(ref NotesRef) []Revision

	// GetMergeBase returns the latest revision that is a common ancestor of the two given revision.
	//
	// This is the revision that is used as the left-hand side for review diffs.
	GetMergeBase(from, to Revision) (Revision, error)

	// GetRawDiff returns the raw diff between two revisions.
	GetRawDiff(from, to Revision) (string, error)

	// GetDetails returns the CommitDetails for the given revision.
	GetDetails(revision Revision) (*CommitDetails, error)

	// PullUpdates updates the status of the local repo based on the remote.
	PullUpdates() error

	// PushUpdates updates the remote repo based on the local one.
	PushUpdates() error
}

func (note Note) ID() string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(note)))
}
