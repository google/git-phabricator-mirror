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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/git-phabricator-mirror/mirror/repository"
	"github.com/google/git-phabricator-mirror/mirror/review/request"
	"log"
	"sort"
	"strconv"
)

type differentialCreateRawDiffRequest struct {
	Diff string `json:"diff"`
}

type rawDiff struct {
	ID int `json:"id"`
}

type differentialCreateRawDiffResponse struct {
	Error        string  `json:"error,omitempty"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
	Response     rawDiff `json:"response,omitempty"`
}

type differentialQueryDiffsRequest struct {
	IDs []int `json:"ids"`
}

type queryDiffItem struct {
	ID         string        `json:"id"`
	Changes    []interface{} `json:"changes"`
	Properties interface{}   `json:"properties"`
}

type differentialQueryDiffsResponse struct {
	Error        string                   `json:"error,omitempty"`
	ErrorMessage string                   `json:"errorMessage,omitempty"`
	Response     map[string]queryDiffItem `json:"response"`
}

func readDiff(diffID int) (*queryDiffItem, error) {
	queryRequest := differentialQueryDiffsRequest{IDs: []int{diffID}}
	var queryResponse differentialQueryDiffsResponse
	runArcCommandOrDie("differential.querydiffs", queryRequest, &queryResponse)
	if queryResponse.Error != "" {
		return nil, fmt.Errorf(queryResponse.ErrorMessage)
	}
	if diff, ok := queryResponse.Response[strconv.Itoa(diffID)]; ok {
		return &diff, nil
	}
	return nil, nil
}

// Differential does not actually store the commit hash for the right hand side of a diff.
// As such, if we have to do some deep inspection to find it. What Differential *does*
// store is a map of "local commits". The last such local commit (by timestamp) is the
// one that was actually used to generate the right hand side of the diff.
func findLastCommit(commitsMap map[string]interface{}) string {
	var timestamps []int
	timestampCommitMap := make(map[int]string)
	for commit, commitData := range commitsMap {
		commitProperties, ok := commitData.(map[string]interface{})
		if ok {
			timestampString, ok := commitProperties["time"].(string)
			if ok {
				timestamp, err := strconv.Atoi(timestampString)
				if err != nil {
					log.Fatal(err)
				}
				timestamps = append(timestamps, timestamp)
				timestampCommitMap[timestamp] = commit
			}
		}
	}
	if len(timestamps) == 0 {
		return ""
	}
	sort.Sort(sort.Reverse(sort.IntSlice(timestamps)))
	return timestampCommitMap[timestamps[0]]
}

// findLastCommit returns the last commit included in a Differential diff.
func (diff *queryDiffItem) findLastCommit() string {
	propertiesMap, ok := diff.Properties.(map[string]interface{})
	if ok {
		commitsMap, ok := propertiesMap["local:commits"].(map[string]interface{})
		if ok {
			return findLastCommit(commitsMap)
		}
	}
	return ""
}

// getDiffChanges takes two revisions from which to generate a "git diff", and returns a
// slice of "changes" objects that represent that diff as parsed by Phabricator.
func (arc Arcanist) getDiffChanges(repo repository.Repo, from, to repository.Revision) ([]interface{}, error) {
	// TODO(ojarjur): This is a big hack, but so far there does not seem to be a better solution:
	// We need to pass a list of "changes" JSON objects that contain the parsed diff contents.
	// The simplest way to do that parsing seems to be to create a rawDiff and have Phabricator
	// parse it on the server side. We then read back that diff, and return the changes from it.
	rawDiff, err := repo.GetRawDiff(from, to)
	if err != nil {
		return nil, err
	}
	createRequest := differentialCreateRawDiffRequest{Diff: rawDiff}
	var createResponse differentialCreateRawDiffResponse
	runArcCommandOrDie("differential.createrawdiff", createRequest, &createResponse)
	if createResponse.Error != "" {
		return nil, fmt.Errorf(createResponse.ErrorMessage)
	}
	diffID := createResponse.Response.ID

	diff, err := readDiff(diffID)
	if err != nil {
		return nil, err
	}
	if diff != nil {
		return diff.Changes, nil
	}
	return nil, fmt.Errorf("Failed to retrieve the raw diff for %s..%s", from, to)
}

type differentialCreateDiffRequest struct {
	Branch                    string        `json:"branch,omitempty"`
	SourceControlBaseRevision string        `json:"sourceControlBaseRevision,omitempty"`
	SourceControlPath         string        `json:"sourceControlPath,omitempty"`
	SourceControlSystem       string        `json:"sourceControlSystem,omitempty"`
	SourceMachine             string        `json:"sourceMachine,omitempty"`
	SourcePath                string        `json:"sourcePath,omitempty"`
	LintStatus                string        `json:"lintStatus,omitempty"`
	UnitStatus                string        `json:"unitStatus,omitempty"`
	Changes                   []interface{} `json:"changes,omitempty"`
}

type differentialDiff struct {
	ID  int    `json:"diffid,omitempty"`
	URI string `json:"uri,omitempty"`
}

type differentialCreateDiffResponse struct {
	Error        string           `json:"error,omitempty"`
	ErrorMessage string           `json:"errorMessage,omitempty"`
	Response     differentialDiff `json:"response,omitempty"`
}

type differentialSetDiffPropertyRequest struct {
	ID   int    `json:"diff_id"`
	Name string `json:"name"`
	Data string `json:"data"`
}

type differentialSetDiffPropertyResponse struct {
	Error        string `json:"error,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func (arc Arcanist) setDiffProperty(diffID int, name, value string) error {
	setPropertyRequest := differentialSetDiffPropertyRequest{
		ID:   diffID,
		Name: name,
		Data: value,
	}
	var setPropertyResponse differentialSetDiffPropertyResponse
	runArcCommandOrDie("differential.setdiffproperty", setPropertyRequest, &setPropertyResponse)
	if setPropertyResponse.Error != "" {
		return errors.New(setPropertyResponse.ErrorMessage)
	}
	return nil
}

// createDifferentialDiff generates a Phabricator resource that represents a diff between two revisions.
//
// The generated resource includes metadata about how the diff was generated, and a JSON representation
// of the changes from the diff, as parsed by Phabricator.
func (arc Arcanist) createDifferentialDiff(repo repository.Repo, mergeBase, revision repository.Revision, req request.Request, priorDiffs []string) (*differentialDiff, error) {
	revisionDetails, err := repo.GetDetails(revision)
	if err != nil {
		return nil, err
	}
	changes, err := arc.getDiffChanges(repo, mergeBase, revision)
	if err != nil {
		return nil, err
	}
	createRequest := differentialCreateDiffRequest{
		Branch:                    abbreviateRefName(req.ReviewRef),
		SourceControlSystem:       "git",
		SourceControlBaseRevision: string(mergeBase),
		SourcePath:                repo.GetPath(),
		LintStatus:                "5", // Status code 5 means "linter postponed"
		UnitStatus:                "5", // Status code 5 means "unit tests have been postponed"
		Changes:                   changes,
	}
	var createResponse differentialCreateDiffResponse
	runArcCommandOrDie("differential.creatediff", createRequest, &createResponse)
	if createResponse.Error != "" {
		return nil, fmt.Errorf(createResponse.ErrorMessage)
	}

	localCommits := make(map[string]interface{})
	for _, priorDiff := range priorDiffs {
		diffID, err := strconv.Atoi(priorDiff)
		if err != nil {
			return nil, err
		}
		queryRequest := differentialQueryDiffsRequest{[]int{diffID}}
		var queryResponse differentialQueryDiffsResponse
		runArcCommandOrDie("differential.querydiffs", queryRequest, &queryResponse)
		if queryResponse.Error != "" {
			return nil, fmt.Errorf(queryResponse.ErrorMessage)
		}
		priorProperty := queryResponse.Response[priorDiff].Properties
		if priorPropertyMap, ok := priorProperty.(map[string]interface{}); ok {
			if localCommitsProperty, ok := priorPropertyMap["local:commits"]; ok {
				if priorLocalCommits, ok := localCommitsProperty.(map[string]interface{}); ok {
					for id, val := range priorLocalCommits {
						localCommits[id] = val
					}
				}
			}
		}
	}
	localCommits[string(revision)] = *revisionDetails
	localCommitsProperty, err := json.Marshal(localCommits)
	if err != nil {
		return nil, err
	}
	if err := arc.setDiffProperty(createResponse.Response.ID, "local:commits", string(localCommitsProperty)); err != nil {
		return nil, err
	}
	if err := arc.setDiffProperty(createResponse.Response.ID, "arc:unit", "{}"); err != nil {
		return nil, err
	}

	return &createResponse.Response, nil
}
