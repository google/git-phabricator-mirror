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

package mirror

import (
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/repository"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/comment"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/request"
	"testing"
)

const testRevision = "ABCDEFG"
const testReviewNote = `{
	"reviewRef": "refs/heads/ojarjur/my-feature-branch",
	"targetRef": "refs/heads/master",
	"description": "Test Code Review"
}`

var expectedRequest = request.Request{
	ReviewRef:   "refs/heads/ojarjur/my-feature-branch",
	TargetRef:   "refs/heads/master",
	Description: "Test Code Review",
}

type mockRepo struct {
	Notes map[repository.Revision][]repository.Note
}

func (repo mockRepo) GetPath() string {
	return ""
}

func (repo mockRepo) GetRepoStateHash() string {
	return "abcde"
}

func (repo mockRepo) GetNotes(ref repository.NotesRef, revision repository.Revision) []repository.Note {
	return repo.Notes[revision]
}

func (repo mockRepo) AppendNote(ref repository.NotesRef, revision repository.Revision, note repository.Note, authorEmail string) {
	repo.Notes[revision] = append(repo.Notes[revision], note)
}

func (repo mockRepo) ListNotedRevisions(ref repository.NotesRef) []repository.Revision {
	return []repository.Revision{
		repository.Revision(testRevision),
	}
}

func (repo mockRepo) GetMergeBase(from, to repository.Revision) (repository.Revision, error) {
	return repository.Revision(""), nil
}

func (repo mockRepo) GetRawDiff(from, to repository.Revision) (string, error) {
	return "", nil
}

func (repo mockRepo) GetDetails(revision repository.Revision) (*repository.CommitDetails, error) {
	return &repository.CommitDetails{}, nil
}

func (repo mockRepo) PullUpdates() error {
	return nil
}

func (repo mockRepo) PushUpdates() error {
	return nil
}

type mockReviewTool struct {
	Requests map[repository.Revision]request.Request
}

func (tool *mockReviewTool) EnsureRequestExists(repo repository.Repo, revision repository.Revision, req request.Request, comments map[string]comment.Comment) {
	tool.Requests[revision] = req
}

func (tool *mockReviewTool) ListOpenReviews(repo repository.Repo) []review.Review {
	return nil
}

func (tool *mockReviewTool) Refresh(repo repository.Repo) {}

func validateExpectedRequest(expected request.Request, actual request.Request) bool {
	return (expected.ReviewRef == actual.ReviewRef &&
		expected.TargetRef == actual.TargetRef &&
		expected.Description == actual.Description)
}

func TestMirrorRepo(t *testing.T) {
	repo := mockRepo{
		Notes: map[repository.Revision][]repository.Note{
			repository.Revision(testRevision): []repository.Note{
				repository.Note(testReviewNote),
			},
		},
	}
	tool := mockReviewTool{make(map[repository.Revision]request.Request)}
	syncToRemote := true
	mirrorRepoToReview(repo, &tool, syncToRemote)
	if len(tool.Requests) != 1 || !validateExpectedRequest(expectedRequest, tool.Requests[testRevision]) {
		t.Errorf("Review requests are not what we expected: %v", tool.Requests)
	}
}
