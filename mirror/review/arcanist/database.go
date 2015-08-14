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

// This is far from ideal.
//
// Phabricator does not currently provide any sort of API for quering the code review comments.
// To work around this, we directly query the underlying database tables.
//
// There are three tables from which we need to read, all under the "phabricator_differential" schema:
//  differential_transaction stores the top level code review actions, like commenting.
//  differential_transaction_comment stores the actual contents of comments.
//  differential_changeset stores the diffs against which a comment was made.

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/comment"
	"strconv"
	"strings"
	"time"
)

const (
	// SQL query for differential "transactions". These are atomic operations on a review.
	selectTransactionsQueryTemplate = `
select id, phid, authorPHID, dateCreated, transactionType, newValue, commentPHID
  from phabricator_differential.differential_transaction
	where objectPHID="%s"
		and viewPolicy="public"
		and (transactionType = "differential:action" or
     transactionType = "differential:inline" or
     transactionType = "core:comment")
  order by id;`
	// SQL query for differential "transaction comments". These are always tied
	// to a differential "transaction" and include the body of a review comment.
	selectTransactionCommentsQueryTemplate = `
select phid, changesetID, lineNumber, replyToCommentPHID
	from phabricator_differential.differential_transaction_comment
	where viewPolicy = "public" and transactionPHID = "%s";`
	// SQL query for the contents of a differential "transaction comment". This
	// is separated from the query for the other fields so that we don't have to
	// worry about contents that include tabs, which mysql uses as the separator
	// between multiple column values.
	selectCommentContentsQueryTemplate = `
select content from phabricator_differential.differential_transaction_comment
	where phid = "%s";`
	// SQL query to read the filename for a diff.
	selectChangesetFilenameTemplate = `
select filename from phabricator_differential.differential_changeset
	where id = "%d";`
	// SQL query to read the ID for a diff. We need this in order to be able
	// to read the commit hash for a diff (which we do using the Differential API).
	selectChangesetDiffTemplate = `
select diffID from phabricator_differential.differential_changeset
	where id = "%d";`

	// Timeout used for all SQL queries
	sqlQueryTimeout = 1 * time.Minute
)

// runRawSqlCommandOrDie runs the given SQL command with no additional formatting
// included in the output.
//
// Any errors that could occur here would be a sign of something being seriously
// wrong, so they are treated as fatal. This makes it more evident that something
// has gone wrong when the command is manually run by a user, and gives further
// operations a clean-slate when this is run by supervisord with automatic restarts.
func runRawSqlCommandOrDie(command string) string {
	cmd := exec.Command("mysql", "-Ns", "-r", "-e", command)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Start()
	go func() {
		time.Sleep(sqlQueryTimeout)
		cmd.Process.Kill()
	}()
	if err := cmd.Wait(); err != nil {
		log.Println("Ran SQL command: ", command)
		log.Fatal(err)
	}
	result := strings.TrimSuffix(stdout.String(), "\n")
	return result
}

// runRawSqlCommandOrDie runs the given SQL command.
//
// Any errors that could occur here would be a sign of something being seriously
// wrong, so they are treated as fatal. This makes it more evident that something
// has gone wrong when the command is manually run by a user, and gives further
// operations a clean-slate when this is run by supervisord with automatic restarts.
func runSqlCommandOrDie(command string) string {
	cmd := exec.Command("mysql", "-Ns", "-e", command)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Start()
	go func() {
		time.Sleep(sqlQueryTimeout)
		cmd.Process.Kill()
	}()
	if err := cmd.Wait(); err != nil {
		log.Println("Ran SQL command: ", command)
		log.Fatal(err)
	}
	result := strings.Trim(stdout.String(), "\n")
	return result
}

// differentialDatabaseTransaction represents a user action on a code review.
//
// This includes things like approving or rejecting the change and commenting.
// However, when a transaction represents a comment, it does not contain the actual
// contents of the comment; those are stored in a diffferentialDatabaseTransactionComment.
type differentialDatabaseTransaction struct {
	PHID        string
	AuthorPHID  string
	DateCreated uint32
	Type        string
	NewValue    *string
	CommentPHID *string
}

type ReadTransactions func(reviewID string) ([]differentialDatabaseTransaction, error)

func readDatabaseTransactions(reviewID string) ([]differentialDatabaseTransaction, error) {
	var transactions []differentialDatabaseTransaction
	result := runSqlCommandOrDie(fmt.Sprintf(selectTransactionsQueryTemplate, reviewID))
	if strings.Trim(result, " ") == "" {
		// There were no matching transactions
		return nil, nil
	}
	// result will be a line-separated list of query results, each of which has 7 columns.
	for _, line := range strings.Split(result, "\n") {
		lineParts := strings.Split(line, "\t")
		if len(lineParts) != 7 {
			return nil, fmt.Errorf("Unexpected number of transaction parts: %v", lineParts)
		}
		var transaction differentialDatabaseTransaction
		transaction.PHID = lineParts[1]
		transaction.AuthorPHID = lineParts[2]
		timestamp, err := strconv.ParseUint(lineParts[3], 10, 32)
		if err != nil {
			return nil, err
		}
		transaction.DateCreated = uint32(timestamp)
		transaction.Type = lineParts[4]
		if lineParts[5] != "NULL" {
			transaction.NewValue = &lineParts[5]
		}
		if lineParts[6] != "NULL" {
			transaction.CommentPHID = &lineParts[6]
		}
		transactions = append(transactions, transaction)
	}
	return transactions, nil
}

// differentialDatabaseTransactionComment stores the actual contents of a code review comment.
type differentialDatabaseTransactionComment struct {
	PHID               string
	Commit             string
	FileName           string
	LineNumber         uint32
	ReplyToCommentPHID *string
	Content            string
}

type ReadTransactionComment func(transactionID string) (*differentialDatabaseTransactionComment, error)

func readDatabaseTransactionComment(transactionID string) (*differentialDatabaseTransactionComment, error) {
	result := runSqlCommandOrDie(fmt.Sprintf(selectTransactionCommentsQueryTemplate, transactionID))
	// result will be a line separated list of query results, each of which includes 4 columns.
	lines := strings.Split(result, "\n")
	if len(lines) != 1 {
		return nil, fmt.Errorf("Unexpected number of query results: %v", lines)
	}
	lineParts := strings.Split(lines[0], "\t")
	if len(lineParts) != 4 {
		return nil, fmt.Errorf("Unexpected size of query results: %v", lineParts)
	}
	var comment differentialDatabaseTransactionComment
	comment.PHID = lineParts[0]
	if lineParts[1] != "NULL" {
		changesetID, err := strconv.ParseUint(lineParts[1], 10, 32)
		if err != nil {
			return nil, err
		}
		changesetResult := runSqlCommandOrDie(fmt.Sprintf(selectChangesetFilenameTemplate, changesetID))
		// changesetResult should have a single query result, which only includes the filename.
		comment.FileName = changesetResult
		diffIDResult := runSqlCommandOrDie(fmt.Sprintf(selectChangesetDiffTemplate, changesetID))
		diffID, err := strconv.Atoi(diffIDResult)
		if err != nil {
			log.Println(diffIDResult)
			log.Fatal(err)
		}
		diff, err := readDiff(diffID)
		if err != nil {
			log.Fatal(err)
		}
		comment.Commit = diff.findLastCommit()
	}
	lineNumber, err := strconv.ParseUint(lineParts[2], 10, 32)
	if err != nil {
		return nil, err
	}
	comment.LineNumber = uint32(lineNumber)
	if lineParts[3] != "NULL" {
		comment.ReplyToCommentPHID = &lineParts[3]
	}
	// The next SQL command is structured to return a single result with a single column, so we
	// don't need to parse it in any way.
	comment.Content = runRawSqlCommandOrDie(fmt.Sprintf(selectCommentContentsQueryTemplate, comment.PHID))
	return &comment, nil
}

// LoadComments takes in a differentialReview and returns the associated comments.
func (review differentialReview) LoadComments() []comment.Comment {
	return LoadComments(review, readDatabaseTransactions, readDatabaseTransactionComment, lookupUser)
}

func LoadComments(review differentialReview, readTransactions ReadTransactions, readTransactionComment ReadTransactionComment, lookupUser UserLookup) []comment.Comment {

	allTransactions, err := readTransactions(review.PHID)
	if err != nil {
		log.Fatal(err)
	}
	var comments []comment.Comment
	commentsByPHID := make(map[string]comment.Comment)
	rejectionCommentsByUser := make(map[string][]string)

	log.Printf("LOADCOMMENTS: Returning %d transactions", len(allTransactions))
	for _, transaction := range allTransactions {
		author, err := lookupUser(transaction.AuthorPHID)
		if err != nil {
			log.Fatal(err)
		}
		c := comment.Comment{
			Author:    author.Email,
			Timestamp: fmt.Sprintf("%d", transaction.DateCreated),
		}
		if author.Email != "" {
			c.Author = author.Email
		} else {
			c.Author = author.UserName
		}

		if transaction.CommentPHID != nil {
			transactionComment, err := readTransactionComment(transaction.PHID)
			if err != nil {
				log.Fatal(err)
			}
			if transactionComment.FileName != "" {
				c.Location = &comment.CommentLocation{
					Commit: transactionComment.Commit,
					Path:   transactionComment.FileName,
				}
				if transactionComment.LineNumber != 0 {
					c.Location.Range = &comment.CommentRange{
						StartLine: transactionComment.LineNumber,
					}
				}
			}
			c.Description = transactionComment.Content
			if transactionComment.ReplyToCommentPHID != nil {
				// We assume that the parent has to have been processed before the child,
				// and enforce that by ordering the transactions in our queries.
				if replyTo, ok := commentsByPHID[*transactionComment.ReplyToCommentPHID]; ok {
					parentHash, err := replyTo.Hash()
					if err != nil {
						log.Fatal(err)
					}
					c.Parent = parentHash
				}
			}
		}

		// Set the resolved bit based on whether the change was approved or not.
		if transaction.Type == "differential:action" && transaction.NewValue != nil {
			action := *transaction.NewValue
			var resolved bool
			if action == "\"accept\"" {
				resolved = true
				c.Resolved = &resolved

				// Add child comments to all previous rejects by this user and make them accepts
				for _, rejectionCommentHash := range rejectionCommentsByUser[author.UserName] {
					approveComment := comment.Comment{
						Author:    c.Author,
						Timestamp: c.Timestamp,
						Resolved:  &resolved,
						Parent:    rejectionCommentHash,
					}
					comments = append(comments, approveComment)
					log.Printf("LOADCOMMENTS: Received approval. Adding child comment %v with parent hash %x", approveComment, rejectionCommentHash)
				}
			} else if action == "\"reject\"" {
				resolved = false
				c.Resolved = &resolved
			}

		}

		// Phabricator only publishes inline comments when you publish a top-level comment.
		// This results in a lot of empty top-level comments, which we do not want to mirror.
		// To work around this, we only return comments that are non-empty.
		if c.Parent != "" || c.Location != nil || c.Description != "" || c.Resolved != nil {
			comments = append(comments, c)
			commentsByPHID[transaction.PHID] = c

			//If this was a rejection comment, add it to ordered comment hash
			if c.Resolved != nil && *c.Resolved == false {
				commentHash, err := c.Hash()
				if err != nil {
					log.Fatal(err)
				}
				log.Printf("LOADCOMMENTS: Received rejection. Adding comment %v with hash %x", c, commentHash)
				rejectionCommentsByUser[author.UserName] = append(rejectionCommentsByUser[author.UserName], commentHash)
			}

		}
	}

	log.Printf("LOADCOMMENTS: Returning %d comments", len(comments))
	return comments
}
