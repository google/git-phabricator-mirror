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

// Package arcanist contains methods for issuing API calls to Phabricator via the "arc" command-line tool.
package arcanist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/repository"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/comment"
	"source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/request"
  "source.developers.google.com/id/AOYtBqJZlBK.git/mirror/review/ci"
	"strconv"
	"strings"
	"time"
)

// commitHashType is a special string that Phabricator uses internally to distinguish
// hashes for commit objects from hashes for other types of objects (such as trees or file blobs).
const commitHashType = "gtcm"

// Differential (Phabricator's code review tool) limits review titles to 40 characters.
const differentialTitleLengthLimit = 256

// Differential stores review status as a string, so we have to maintain a mapping of
// the status strings that we care about.
const (
	differentialNeedsReviewStatus = "0"
	differentialClosedStatus      = "3"
	differentialAbandonedStatus   = "4"
)

// defaultRepoDirPrefix is the default parent directory Phabricator uses to store repos.
const defaultRepoDirPrefix = "/var/repo/"

// arcanistRequestTimeout is the amount of time we allow arcanist requests to wait before interrupting them.
const arcanistRequestTimeout = 1 * time.Minute

// Arcanist represents an instance of the "arcanist" command-line tool.
type Arcanist struct {
}

// runArcCommandOrDie runs the given Conduit API call using the "arc" command line tool.
//
// Any errors that could occur here would be a sign of something being seriously
// wrong, so they are treated as fatal. This makes it more evident that something
// has gone wrong when the command is manually run by a user, and gives further
// operations a clean-slate when this is run by supervisord with automatic restarts.
func runArcCommandOrDie(method string, request interface{}, response interface{}) {
	cmd := exec.Command("arc", "call-conduit", method)
	input, err := json.Marshal(request)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Running conduit request: ", method, string(input))
	cmd.Stdin = strings.NewReader(string(input))

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	go func() {
		time.Sleep(arcanistRequestTimeout)
		cmd.Process.Kill()
	}()
	if err := cmd.Wait(); err != nil {
		log.Print("arc", "call-conduit", method, string(input), stdout.String())
		log.Fatal(err)
	}
	log.Print("Received conduit response ", stdout.String())
	if err = json.Unmarshal(stdout.Bytes(), response); err != nil {
		log.Fatal(err)
	}
}

func abbreviateRefName(ref string) string {
	if strings.HasPrefix(ref, "refs/heads/") {
		return ref[len("refs/heads/"):]
	}
	return ref
}

type differentialReview struct {
	ID         string     `json:"id,omitempty"`
	PHID       string     `json:"phid,omitempty"`
	Title      string     `json:"title,omitempty"`
	Branch     string     `json:"branch,omitempty"`
	Status     string     `json:"status,omitempty"`
	StatusName string     `json:"statusName,omitempty"`
	AuthorPHID string     `json:"authorPHID,omitempty"`
	Reviewers  []string   `json:"reviewers,omitempty"`
	Hashes     [][]string `json:"hashes,omitempty"`
	Diffs      []string   `json:"diffs,omitempty"`
}

// GetFirstCommit returns the first commit that is included in the review
func (review differentialReview) GetFirstCommit(repo repository.Repo) *repository.Revision {
	var commits []string
	for _, hashPair := range review.Hashes {
		// We only care about the hashes for commits, which have exactly two
		// elements, the first of which is "gtcm".
		if len(hashPair) == 2 && hashPair[0] == commitHashType {
			commits = append(commits, hashPair[1])
		}
	}
	var commitTimestamps []int
	commitsByTimestamp := make(map[int]string)
	for _, commit := range commits {
		details, err := repo.GetDetails(repository.Revision(commit))
		if err == nil {
			timestamp, err := strconv.Atoi(details.Time)
			if err == nil {
				commitTimestamps = append(commitTimestamps, timestamp)
				// If there are multiple, equally old commits, then the last one wins.
				commitsByTimestamp[timestamp] = commit
			}
		}
	}
	if len(commitTimestamps) == 0 {
		return nil
	}
	sort.Ints(commitTimestamps)
	revision := repository.Revision(commitsByTimestamp[commitTimestamps[0]])
	return &revision
}

// queryRequest specifies filters for review queries. Specifically, CommitHashes filters
// reviews to only those that contain the specified hashes, and Status filters reviews to
// only those that match the given status (e.g. "status-any", "status-open", etc.)
type queryRequest struct {
	CommitHashes [][]string `json:"commitHashes,omitempty"`
	Status       string     `json:"status,omitempty"`
}

type queryResponse struct {
	Error        string               `json:"error,omitempty"`
	ErrorMessage string               `json:"errorMessage,omitempty"`
	Response     []differentialReview `json:"response,omitempty"`
}

func (arc Arcanist) listDifferentialReviewsOrDie(reviewRef string, revision repository.Revision) []differentialReview {
	request := queryRequest{
		CommitHashes: [][]string{[]string{commitHashType, string(revision)}},
	}
	var response queryResponse
	runArcCommandOrDie("differential.query", request, &response)

	var filteredList []differentialReview
	for _, review := range response.Response {
		// Phabricator has a branch field for limiting query results, but it seems to
		// handle that field incorrectly, and returns no results if it is specified.
		// As such, we simple query for all results, and filter them on the client side.
		if review.Branch == reviewRef || review.Branch == abbreviateRefName(reviewRef) {
			filteredList = append(filteredList, review)
		}
	}
	return filteredList
}

func (arc Arcanist) ListOpenReviews(repo repository.Repo) []review.Review {
	// TODO(ojarjur): Filter the query by the repo.
	// As is, we simply return all open reviews for *any* repo, and then filter in
	// the calling level.
	request := queryRequest{
		Status: "status-open",
	}
	var response queryResponse
	runArcCommandOrDie("differential.query", request, &response)
	var reviews []review.Review
	for _, r := range response.Response {
		reviews = append(reviews, r)
	}
	return reviews
}

type revisionFields struct {
	Title     string   `json:"title,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Reviewers []string `json:"reviewerPHIDs,omitempty"`
	CCs       []string `json:"ccPHIDs,omitempty"`
}

type createRevisionRequest struct {
	DiffID int            `json:"diffid"`
	Fields revisionFields `json:"fields,omitempty"`
}

type differentialRevision struct {
	RevisionID int        `json:"revisionid,omitempty"`
	URI        string     `json:"uri,omitempty"`
	Title      string     `json:"title,omitempty"`
	Branch     string     `json:"branch,omitempty"`
	Hashes     [][]string `json:"hashes,omitempty"`
}

type createRevisionResponse struct {
	Error        string               `json:"error,omitempty"`
	ErrorMessage string               `json:"errorMessage,omitempty"`
	Response     differentialRevision `json:"response,omitempty"`
}

func (arc Arcanist) createDifferentialRevision(repo repository.Repo, revision repository.Revision, diffID int, req request.Request) (*differentialRevision, error) {
	// If the description is multiple lines, then treat the first as the title.
	fields := revisionFields{Title: strings.Split(req.Description, "\n")[0]}
	// Truncate the title if it is too long.
	if len(fields.Title) > differentialTitleLengthLimit {
		truncatedLimit := differentialTitleLengthLimit - 4
		fields.Title = fields.Title[0:truncatedLimit] + "..."
	}
	// If we modified the title from the description, then put the full description in the summary.
	if fields.Title != req.Description {
		fields.Summary = req.Description
	}
	for _, reviewer := range req.Reviewers {
		user, err := queryUser(reviewer)
		if err != nil {
			log.Print(err)
		} else if user != nil {
			fields.Reviewers = append(fields.Reviewers, user.PHID)
		}
	}
	if req.Requester != "" {
		user, err := queryUser(req.Requester)
		if err != nil {
			log.Print(err)
		} else if user != nil {
			fields.CCs = append(fields.CCs, user.PHID)
		}
	}
	createRequest := createRevisionRequest{diffID, fields}
	var createResponse createRevisionResponse
	runArcCommandOrDie("differential.createrevision", createRequest, &createResponse)
	if createResponse.Error != "" {
		return nil, fmt.Errorf("Failed to create the differential revision: %s", createResponse.ErrorMessage)
	}
	return &createResponse.Response, nil
}

type differentialUpdateRevisionRequest struct {
	ID     string `json:"id"`
	DiffID string `json:"diffid"`
	// Fields is a map of new values for the fields of the revision object. Any fields
	// that do not have corresponding entries are left unchanged.
	Fields map[string]interface{} `json:"fields,omitempty"`
}

type differentialUpdateRevisionResponse struct {
	Error        string `json:"error,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func (review differentialReview) isClosed() bool {
	return review.Status == differentialClosedStatus || review.Status == differentialAbandonedStatus
}

type differentialCloseRequest struct {
	ID int `json:"revisionID"`
}

type differentialCloseResponse struct {
	Error        string `json:"error,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func (review differentialReview) close() {
	reviewID, err := strconv.Atoi(review.ID)
	if err != nil {
		log.Fatal(err)
	}
	closeRequest := differentialCloseRequest{reviewID}
	var closeResponse differentialCloseResponse
	runArcCommandOrDie("differential.close", closeRequest, &closeResponse)
	if closeResponse.Error != "" {
		// This might happen if someone merged in a review that wasn't accepted yet, or if the review is not owned by the robot account.
		log.Println(closeResponse.ErrorMessage)
	}
}

func findCommitForDiff(diffIDString string) string {
	diffID, err := strconv.Atoi(diffIDString)
	if err != nil {
		return ""
	}
	diff, err := readDiff(diffID)
	if err != nil {
		return ""
	}

	return diff.findLastCommit()
}

// createCommentRequest models the request format for
// Phabricator's differential.createcomment API method.
type createCommentRequest struct {
	RevisionID    string `json:"revision_id,omitempty"`
	Message       string `json:"content,omitempty"`
	Action        string `json:"action,omitempty"`
	AttachInlines bool   `json:"attach_inlines,omitempty"`
}

// createInlineRequest models the request format for
// Phabricator's differential.createinline API method.
type createInlineRequest struct {
	RevisionID string `json:"revisionID,omitempty"`
	DiffID     string `json:"diffID,omitempty"`
	FilePath   string `json:"filePath,omitempty"`
	LineNumber uint32 `json:"lineNumber,omitempty"`
	Content    string `json:"content,omitempty"`
	IsNewFile  uint32 `json:"isNewFile"`
}

// createInlineResponse models the response format for
// Phabricator's differential.createinline API method.
type createInlineResponse struct {
	Error        string `json:"error,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// createCommentResponse models the response format for
// Phabricator's differential.createcomment API method.
type createCommentResponse struct {
	Error        string `json:"error,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func (review differentialReview) buildCommentRequests(newComments []comment.Comment, commitToDiffMap map[string]string) ([]createInlineRequest, []createCommentRequest) {
	var inlineRequests []createInlineRequest
	var commentRequests []createCommentRequest

	for _, c := range newComments {
		if c.Location != nil && c.Location.Path != "" {
			// TODO(ojarjur): Also mirror whole-review comments.
			var lineNumber uint32 = 1
			if c.Location.Range != nil {
				lineNumber = c.Location.Range.StartLine
			}
			diffID := commitToDiffMap[c.Location.Commit]
			if diffID != "" {
				content := c.QuoteDescription()
				request := createInlineRequest{
					RevisionID: review.ID,
					DiffID:     diffID,
					FilePath:   c.Location.Path,
					LineNumber: lineNumber,
					// IsNewFile indicates if the comment is on the left-hand side (0) or the right-hand side (1).
					// We always post comments to the right-hand side.
					IsNewFile: 1,
					Content:   content,
				}
				inlineRequests = append(inlineRequests, request)
			}
		}
	}
	if len(inlineRequests) > 0 {
		request := createCommentRequest{
			RevisionID:    review.ID,
			Action:        "comment",
			AttachInlines: true,
		}
		commentRequests = append(commentRequests, request)
	}
	return inlineRequests, commentRequests
}

type differentialUpdateUnitResultsRequest struct {
  DiffID string `json:"diffid"`
  Result string `json:"result"`
  Link string `json:"link"`
}

type differentialUpdateUnitResultsResponse struct {
  Error        string `json:"error,omitempty"`
  ErrorMessage string `json:"errorMessage,omitempty"`
}

func (arc Arcanist) mirrorCommentsIntoReview(repo repository.Repo, review differentialReview, comments comment.CommentMap) {
	existingComments := review.LoadComments()
	newComments := comments.FilterOverlapping(existingComments)

  var lastCommitForLastDiff string
	commitToDiffMap := make(map[string]string)
	for _, diffIDString := range review.Diffs {
		lastCommit := findCommitForDiff(diffIDString)
		commitToDiffMap[lastCommit] = diffIDString
    lastCommitForLastDiff = lastCommit
	}

  report := ci.GetLatestCIReport(repo.GetNotes(ci.Ref, repository.Revision(lastCommitForLastDiff)))

  log.Println("The latest CI report for diff %s is %+v ",commitToDiffMap[lastCommitForLastDiff], report)
  if report.URL != "" {
  	updateUnitResultsRequest := differentialUpdateUnitResultsRequest{
																	DiffID: commitToDiffMap[lastCommitForLastDiff],
      														Result: report.Status,
      														Link: report.URL,
		       										}
  	var unitResultsResponse differentialUpdateUnitResultsResponse
  	runArcCommandOrDie("differential.updateunitresults", updateUnitResultsRequest, &unitResultsResponse)
  	if unitResultsResponse.Error != "" {
    	log.Fatal(unitResultsResponse.ErrorMessage)
  	}
  }

	inlineRequests, commentRequests := review.buildCommentRequests(newComments, commitToDiffMap)
	for _, request := range inlineRequests {
		var response createInlineResponse
		runArcCommandOrDie("differential.createinline", request, &response)
		if response.Error != "" {
			log.Println(response.ErrorMessage)
		}
	}
	for _, request := range commentRequests {
		var response createCommentResponse
		runArcCommandOrDie("differential.createcomment", request, &response)
		if response.Error != "" {
			log.Println(response.ErrorMessage)
		}
	}
}

// updateReviewDiffs updates the status of a differential review so that it matches the state of the repo.
//
// This consists of making sure the latest commit pushed to the review ref has a corresponding
// diff in the differential review.
func (arc Arcanist) updateReviewDiffs(repo repository.Repo, review differentialReview, headCommit string, req request.Request, comments map[string]comment.Comment) {
	if review.isClosed() {
		return
	}

	headRevision := repository.Revision(headCommit)
	mergeBase, err := repo.GetMergeBase(repository.Revision(req.TargetRef), headRevision)
	if err != nil {
		// This can happen if the target ref has been deleted while we were performing the updates.
		return
	}

	for _, hashPair := range review.Hashes {
		if len(hashPair) == 2 && hashPair[0] == commitHashType && hashPair[1] == headCommit {
			// The review already has the hash of the HEAD commit, so we have nothing to do beyond mirroring comments
      // and build status if applicable
			arc.mirrorCommentsIntoReview(repo, review, comments)
			return
		}
	}

	diff, err := arc.createDifferentialDiff(repo, mergeBase, headRevision, req, review.Diffs)
	if err != nil {
		log.Fatal(err)
	}
	if diff == nil {
		// This means that phabricator silently refused to create the diff. Just move on.
		return
	}

	updateRequest := differentialUpdateRevisionRequest{ID: review.ID, DiffID: strconv.Itoa(diff.ID)}
	var updateResponse differentialUpdateRevisionResponse
	runArcCommandOrDie("differential.updaterevision", updateRequest, &updateResponse)
	if updateResponse.Error != "" {
		log.Fatal(updateResponse.ErrorMessage)
	}
}

// EnsureRequestExists runs the "arcanist" command-line tool to create a Differential diff for the given request, if one does not already exist.
func (arc Arcanist) EnsureRequestExists(repo repository.Repo, revision repository.Revision, req request.Request, comments map[string]comment.Comment) {
	mergeBase, err := repo.GetMergeBase(repository.Revision(req.TargetRef), revision)
	if err != nil {
		// There are lots of reasons that we might not be able to compute a merge base,
		// (e.g. the revision already being merged in, or being dropped and garbage collected),
		// but they all indicate that the review request is no longer valid.
		log.Printf("Ignoring review request '%v', because we could not compute a merge base", req)
		return
	}

	existingReviews := arc.listDifferentialReviewsOrDie(req.ReviewRef, revision)
	if mergeBase == revision {
		// The change has already been merged in, so we should simply close any open reviews.
		for _, review := range existingReviews {
			if !review.isClosed() {
				review.close()
			}
		}
		return
	}

	headDetails, err := repo.GetDetails(repository.Revision(req.ReviewRef))
	if err != nil {
		// The given review ref has been deleted (or never existed), but the change wasn't merged.
		// TODO(ojarjur): We should mark the existing reviews as abandoned.
		log.Printf("Ignoring review because the review ref '%s' does not exist", req.ReviewRef)
		return
	}

	if len(existingReviews) > 0 {
		// The change is still pending, but we already have existing reviews, so we should just update those.
		for _, review := range existingReviews {
			arc.updateReviewDiffs(repo, review, headDetails.Commit, req, comments)
		}
		return
	}

	diff, err := arc.createDifferentialDiff(repo, mergeBase, revision, req, []string{})
	if err != nil {
		log.Fatal(err)
	}
	if diff == nil {
		// The revision is already merged in, ignore it.
		return
	}
	rev, err := arc.createDifferentialRevision(repo, revision, diff.ID, req)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Created diff %v and revision %v for the review of %s", diff, rev, revision)

	// If the review already contains multiple commits by the time we mirror it, then
	// we need to ensure that at least the first and last ones are added.
	existingReviews = arc.listDifferentialReviewsOrDie(req.ReviewRef, revision)
	for _, review := range existingReviews {
		arc.updateReviewDiffs(repo, review, headDetails.Commit, req, comments)
	}
}

// lookSoonRequest specifies a list of callsigns (repo identifier) for repos that have recently changed.
type lookSoonRequest struct {
	Callsigns []string `json:"callsigns,omitempty"`
}

// Refresh advises the review tool that the code being reviewed has changed, and to reload it.
//
// This corresponds to calling the diffusion.looksoon API.
func (arc Arcanist) Refresh(repo repository.Repo) {
	// We cannot determine the repo's callsign (the identifier Phabricator uses for the repo)
	// in all cases, but we can figure it out in the case that the mirror runs on the same
	// directories that Phabricator is using. In that scenario, the repo directories default
	// to being named "/var/repo/<CALLSIGN>", so if the repo path starts with that prefix then
	// we can try to strip out that prefix and use the rest as a callsign.
	if strings.HasPrefix(repo.GetPath(), defaultRepoDirPrefix) {
		possibleCallsign := strings.TrimPrefix(repo.GetPath(), defaultRepoDirPrefix)
		request := lookSoonRequest{Callsigns: []string{possibleCallsign}}
		response := make(map[string]interface{})
		runArcCommandOrDie("diffusion.looksoon", request, &response)
	}
}
