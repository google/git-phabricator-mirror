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
	"github.com/google/git-appraise/repository"
	"github.com/google/git-appraise/review"
	"github.com/google/git-appraise/review/request"
	phabricatorReview "github.com/google/git-phabricator-mirror/mirror/review"
	"testing"
)

type mockReviewTool struct {
	Requests map[string]request.Request
}

func (tool *mockReviewTool) EnsureRequestExists(repo repository.Repo, r review.Review) {
	tool.Requests[r.Revision] = r.Request
}

func (tool *mockReviewTool) ListOpenReviews(repo repository.Repo) []phabricatorReview.PhabricatorReview {
	return nil
}

func (tool *mockReviewTool) Refresh(repo repository.Repo) {}

func TestMirrorRepo(t *testing.T) {
	repo := repository.NewMockRepoForTest()
	tool := mockReviewTool{make(map[string]request.Request)}
	syncToRemote := true
	mirrorRepoToReview(repo, &tool, syncToRemote)
	if len(tool.Requests) != len(review.ListAll(repo)) {
		t.Errorf("Review requests are not what we expected: %v", tool.Requests)
	}
}
