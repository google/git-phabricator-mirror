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
	"testing"
)

func TestOverlaps(t *testing.T) {
	description := `Some comment description

With some text in it.`

	location := CommentLocation{
		Commit: "ABCDEFG",
		Path:   "hello.txt",
		Range: &CommentRange{
			StartLine: 42,
		},
	}
	comment := Comment{
		Timestamp:   "012345",
		Author:      "foo@bar.com",
		Location:    &location,
		Description: description,
	}
	quotedComment := Comment{
		Timestamp:   "456789",
		Author:      "bot@robots-r-us.com",
		Location:    &location,
		Description: comment.QuoteDescription(),
	}
	if !comment.Overlaps(quotedComment) {
		t.Errorf("%v and %v do not overlap", comment, quotedComment)
	}
	if !quotedComment.Overlaps(comment) {
		t.Errorf("%v and %v do not overlap", quotedComment, comment)
	}

}

func TestResolvedOverlaps(t *testing.T) {
  reject := false
	accept := true

	blankComment := Comment{
		Timestamp: "012345",
		Author:    "bar@foo.com",
		Resolved:  &reject,
	}

	blankComment2 := Comment{
		Timestamp: "012345",
		Author:    "bar@foo.com",
		Resolved:  &accept,
	}

  // should not overlap because resolved bits are set for both 
  // and different even though with same timestamp
	if blankComment.Overlaps(blankComment2) {
		t.Errorf("%v and %v  overlap", blankComment, blankComment2)
	}

  blankComment2.Resolved = &reject
  // should overlap because resolved bits are set for both and the same with the same timestamp
  if !blankComment.Overlaps(blankComment2) {
		t.Errorf("%v and %v  do not overlap", blankComment, blankComment2)
	}

  blankComment2.Timestamp = "56789"
  // should not overlap because resolved bits are set for both and the same but timestamps are different
  if blankComment.Overlaps(blankComment2) {
		t.Errorf("%v and %v  overlap", blankComment, blankComment2)
	}

  blankComment2.Resolved = &accept
  // should not overlap because resolved bits are set for both and the timestamps are different
  if blankComment.Overlaps(blankComment2) {
		t.Errorf("%v and %v  overlap", blankComment, blankComment2)
	}

  blankComment2.Timestamp = "012345"
  blankComment2.Resolved = nil
   // should not overlap because resolved bit is nil for one
  if blankComment.Overlaps(blankComment2) {
		t.Errorf("%v and %v  overlap", blankComment, blankComment2)
	}

  blankComment.Resolved = nil
  // should overlap because resolved bit is nil for both and there is no other descriptor
  // seperating them apart
  if !blankComment.Overlaps(blankComment2) {
		t.Errorf("%v and %v do not overlap", blankComment, blankComment2)
	}
}


func TestFilterOverlapping(t *testing.T) {
	description := `Some comment description

With some text in it.`
	location := CommentLocation{
		Commit: "ABCDEFG",
		Path:   "hello.txt",
		Range: &CommentRange{
			StartLine: 42,
		},
	}
	comment := Comment{
		Timestamp:   "012345",
		Author:      "foo@bar.com",
		Location:    &location,
		Description: description,
	}
	quotedComment := Comment{
		Timestamp:   "456789",
		Author:      "bot@robots-r-us.com",
		Location:    &location,
		Description: comment.QuoteDescription(),
	}
	replyComment := Comment{
		Timestamp:   "456789",
		Author:      "bot@robots-r-us.com",
		Location:    &location,
		Description: fmt.Sprintf("'%s': Actually, I disagree", description),
	}

	commentMap := make(CommentMap)
	addComment := func(c Comment) {
		hash, err := c.Hash()
		if err != nil {
			t.Errorf("Failure while hashing a comment: %v", err)
		}
		commentMap[hash] = c
	}
	addComment(comment)
	addComment(quotedComment)
	addComment(replyComment)
	existingComments := []Comment{comment}

	filteredComments := commentMap.FilterOverlapping(existingComments)
	if len(filteredComments) != 1 {
		t.Errorf("Unexpected number of filtered results: %v", filteredComments)
	}
	if filteredComments[0] != replyComment {
		t.Errorf("Unexpected filtered comment result: %v", filteredComments[0])
	}
}
