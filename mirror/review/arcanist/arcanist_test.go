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

package arcanist

import (
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/comment"
	"strings"
	"testing"
)

func TestGenerateCommentRequests(t *testing.T) {
	revisionID := "testReview"
	review := differentialReview{ID: revisionID}

	commitToDiffMap := map[string]string{
		"ABCD": "1",
		"EFGH": "2",
	}
	comments := []comment.Comment{
		comment.Comment{
			Timestamp:   "01234",
			Author:      "example@example.com",
			Location:    nil,
			Description: "A review comment",
			Resolved:    nil,
		},
		comment.Comment{
			Timestamp: "01234",
			Author:    "example@example.com",
			Location: &comment.CommentLocation{
				Commit: "ABCD",
				Path:   "hello.txt",
			},
			Description: "A file comment",
			Resolved:    nil,
		},
		comment.Comment{
			Timestamp: "01234",
			Author:    "example@example.com",
			Location: &comment.CommentLocation{
				Commit: "EFGH",
				Path:   "hello.txt",
				Range: &comment.CommentRange{
					StartLine: 42,
				},
			},
			Description: "A line comment",
			Resolved:    nil,
		},
	}
	inlineRequests, commentRequests := review.buildCommentRequests(comments, commitToDiffMap)
	if inlineRequests == nil || commentRequests == nil {
		t.Errorf("Failed to build the comment requests: %v, %v", inlineRequests, commentRequests)
	}
	if len(commentRequests) != 1 || commentRequests[0].RevisionID != revisionID || commentRequests[0].Action != "comment" {
		t.Errorf("Bad comment requests: %v", commentRequests)
	}
	if len(inlineRequests) != 2 {
		t.Errorf("Unexpected number of inline requests: %v", inlineRequests)
	}
	for _, r := range inlineRequests {
		if r.RevisionID != revisionID {
			t.Errorf("Unexpected revisionID: %v", r)
		}
		if r.IsNewFile != 1 {
			t.Errorf("Unexpected IsNewFile field: %v", r)
		}
		if !strings.HasPrefix(r.Content, "example@example.com:\n\n") {
			t.Errorf("Inline comment not quoted as expected: %v", r)
		}
		if r.FilePath != "hello.txt" {
			t.Errorf("Unexpected file path: %v", r)
		}
	}
	firstInline := inlineRequests[0]
	secondInline := inlineRequests[1]
	if firstInline.DiffID != "1" || !strings.HasSuffix(firstInline.Content, "A file comment") || firstInline.LineNumber != 1 {
		t.Errorf("Unexpected first inline request: %v", firstInline)
	}
	if secondInline.DiffID != "2" || !strings.HasSuffix(secondInline.Content, "A line comment") || secondInline.LineNumber != 42 {
		t.Errorf("Unexpected second inline request: %v", secondInline)
	}
}
