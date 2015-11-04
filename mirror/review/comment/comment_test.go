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

package comment

import (
	"fmt"
	gaComment "github.com/google/git-appraise/review/comment"
	"testing"
)

func TestOverlaps(t *testing.T) {
	description := `Some comment description

With some text in it.`

	location := gaComment.Location{
		Commit: "ABCDEFG",
		Path:   "hello.txt",
		Range: &gaComment.Range{
			StartLine: 42,
		},
	}
	comment := gaComment.Comment{
		Timestamp:   "012345",
		Author:      "foo@bar.com",
		Location:    &location,
		Description: description,
	}
	quotedComment := gaComment.Comment{
		Timestamp:   "456789",
		Author:      "bot@robots-r-us.com",
		Location:    &location,
		Description: QuoteDescription(comment),
	}
	if !Overlaps(comment, quotedComment) {
		t.Errorf("%v and %v do not overlap", comment, quotedComment)
	}
	if !Overlaps(quotedComment, comment) {
		t.Errorf("%v and %v do not overlap", quotedComment, comment)
	}

}

func TestResolvedOverlaps(t *testing.T) {
	reject := false
	accept := true

	blankComment := gaComment.Comment{
		Timestamp: "012345",
		Author:    "bar@foo.com",
		Resolved:  &reject,
	}

	blankComment2 := gaComment.Comment{
		Timestamp: "012345",
		Author:    "bar@foo.com",
		Resolved:  &accept,
	}

	// should not overlap because resolved bits are set for both
	// and different even though with same timestamp
	if Overlaps(blankComment, blankComment2) {
		t.Errorf("%v and %v  overlap", blankComment, blankComment2)
	}

	blankComment2.Resolved = &reject
	// should overlap because resolved bits are set for both and the same with the same timestamp
	if !Overlaps(blankComment, blankComment2) {
		t.Errorf("%v and %v  do not overlap", blankComment, blankComment2)
	}

	blankComment2.Timestamp = "56789"
	// should not overlap because resolved bits are set for both and the same but timestamps are different
	if Overlaps(blankComment, blankComment2) {
		t.Errorf("%v and %v  overlap", blankComment, blankComment2)
	}

	blankComment2.Resolved = &accept
	// should not overlap because resolved bits are set for both and the timestamps are different
	if Overlaps(blankComment, blankComment2) {
		t.Errorf("%v and %v  overlap", blankComment, blankComment2)
	}

	blankComment2.Timestamp = "012345"
	blankComment2.Resolved = nil
	// should not overlap because resolved bit is nil for one
	if Overlaps(blankComment, blankComment2) {
		t.Errorf("%v and %v  overlap", blankComment, blankComment2)
	}

	blankComment.Resolved = nil
	// should overlap because resolved bit is nil for both and there is no other descriptor
	// seperating them apart
	if !Overlaps(blankComment, blankComment2) {
		t.Errorf("%v and %v do not overlap", blankComment, blankComment2)
	}
}

func TestFilterOverlapping(t *testing.T) {
	description := `Some comment description

With some text in it.`
	location := gaComment.Location{
		Commit: "ABCDEFG",
		Path:   "hello.txt",
		Range: &gaComment.Range{
			StartLine: 42,
		},
	}
	comment := gaComment.Comment{
		Timestamp:   "012345",
		Author:      "foo@bar.com",
		Location:    &location,
		Description: description,
	}
	quotedComment := gaComment.Comment{
		Timestamp:   "456789",
		Author:      "bot@robots-r-us.com",
		Location:    &location,
		Description: QuoteDescription(comment),
	}
	replyComment := gaComment.Comment{
		Timestamp:   "456789",
		Author:      "bot@robots-r-us.com",
		Location:    &location,
		Description: fmt.Sprintf("'%s': Actually, I disagree", description),
	}

	commentMap := make(CommentMap)
	addComment := func(c gaComment.Comment) {
		hash, err := c.Hash()
		if err != nil {
			t.Errorf("Failure while hashing a comment: %v", err)
		}
		commentMap[hash] = c
	}
	addComment(comment)
	addComment(quotedComment)
	addComment(replyComment)
	existingComments := []gaComment.Comment{comment}

	filteredComments := commentMap.FilterOverlapping(existingComments)
	if len(filteredComments) != 1 {
		t.Errorf("Unexpected number of filtered results: %v", filteredComments)
	}
	if filteredComments[0] != replyComment {
		t.Errorf("Unexpected filtered comment result: %v", filteredComments[0])
	}
}
