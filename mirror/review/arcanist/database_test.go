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
	"testing"
)

func MockReadTransactions(reviewID string) ([]differentialDatabaseTransaction, error) {
	acceptAction := "\"accept\""
	rejectAction := "\"reject\""

	var transactions []differentialDatabaseTransaction

	t1 := BuildTransactionForUser("u1", rejectAction, 1)
	t2 := BuildTransactionForUser("u1", acceptAction, 2)
	t3 := BuildTransactionForUser("u1", rejectAction, 3)
	t4 := BuildTransactionForUser("u2", rejectAction, 4)
	t5 := BuildTransactionForUser("u1", acceptAction, 5)

	transactions = append(transactions, t1, t2, t3, t4, t5)
	return transactions, nil
}

func BuildTransactionForUser(userId string, action string, order int) differentialDatabaseTransaction {
	var transaction differentialDatabaseTransaction
	transaction.AuthorPHID = userId
	transaction.PHID = "123"
	transaction.DateCreated = uint32(order)
	transaction.Type = "differential:action"
	transaction.NewValue = &action
	return transaction
}

func MockReadTransactionComment(transactionID string) (*differentialDatabaseTransactionComment, error) {
	var comment differentialDatabaseTransactionComment
	comment.PHID = "2"
	return &comment, nil
}

func MockLookupUser(userPHID string) (*user, error) {
	var commentUser user
	commentUser.UserName = userPHID
	commentUser.Email = userPHID + "@gmail.com"
	commentUser.PHID = "123"
	return &commentUser, nil
}

/*
Verify the correct tree for the revision has been generated. For the above setup the following tree should be generated.
Hash is made equal to id for understanding purposes

  testReview
    |
 	  |-->[id:1 user:u1 resolved:false parent: hash:1] children: [id:2 user:u1, resolved:true parent:1 hash:2], [id:6 user:u1, resolved:true parent:1 hash: 6]
    |
    |-->[id:3 user:u1 resolved:true parent: hash:3]
    |
    |-->[id:4 user:u1 resolved:false parent: hash:4]  children: [id:7 user:u1, resolved:true parent:4 hash:7]
  	|
  	|-->[id:5 user:u2 resolved:false parent: hash:5]
    |
    |-->[id:8 user:u1 resolved:true parent: hash:8]

*/
func TestLoadComments(t *testing.T) {
	revisionID := "testReview"
	review := differentialReview{ID: revisionID}

	expectedComments := SetupExpectedComments()
	actualComments := LoadComments(review, MockReadTransactions, MockReadTransactionComment, MockLookupUser)

	if len(actualComments) != len(expectedComments) {
		t.Errorf("Unexpected number of comments: %v", actualComments)
	}

	if !validateExpectedComments(expectedComments, actualComments) {
		t.Errorf("Unexpected content expectedComments: %v and actual Comments: %v", expectedComments, actualComments)
	}

}

func validateExpectedComments(existingComments []comment.Comment, expectedComments []comment.Comment) bool {
	for i, actual := range existingComments {
		if actual.Timestamp != expectedComments[i].Timestamp ||
			actual.Author != expectedComments[i].Author ||
			*actual.Resolved != *expectedComments[i].Resolved {
			return false
		}
	}
	return true
}

func SetupExpectedComments() []comment.Comment {

	var expectedComments []comment.Comment
	accept := true
	reject := false
	c1 := comment.Comment{
		Timestamp: "1",
		Author:    "u1@gmail.com",
		Location:  nil,
		Resolved:  &reject,
	}

	c2 := comment.Comment{
		Timestamp: "2",
		Author:    "u1@gmail.com",
		Location:  nil,
		Resolved:  &accept,
		Parent:    getHash(c1),
	}

	c3 := comment.Comment{
		Timestamp: "2",
		Author:    "u1@gmail.com",
		Location:  nil,
		Resolved:  &accept,
	}

	c4 := comment.Comment{
		Timestamp: "3",
		Author:    "u1@gmail.com",
		Location:  nil,
		Resolved:  &reject,
	}

	c5 := comment.Comment{
		Timestamp: "4",
		Author:    "u2@gmail.com",
		Location:  nil,
		Resolved:  &reject,
	}

	c6 := comment.Comment{
		Timestamp: "5",
		Author:    "u1@gmail.com",
		Location:  nil,
		Resolved:  &accept,
		Parent:    getHash(c1),
	}

	c7 := comment.Comment{
		Timestamp: "5",
		Author:    "u1@gmail.com",
		Location:  nil,
		Resolved:  &accept,
		Parent:    getHash(c4),
	}

	c8 := comment.Comment{
		Timestamp: "5",
		Author:    "u1@gmail.com",
		Location:  nil,
		Resolved:  &accept,
	}
	expectedComments = append(expectedComments, c1, c2, c3, c4, c5, c6, c7, c8)
	return expectedComments
}

func getHash(c comment.Comment) string {
	cHash, error := c.Hash()
	if error != nil {
	}
	return cHash
}
