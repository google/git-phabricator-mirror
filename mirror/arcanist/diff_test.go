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
	"testing"
)

func verifyMalformedDiff(t *testing.T, malformedDiff *queryDiffItem) {
	malformedCommit := malformedDiff.findLastCommit()
	if malformedCommit != "" {
		t.Errorf("Wrong result returned from findLastCommit: %v, %s", malformedDiff, malformedCommit)
	}
}

func TestFindLastCommit(t *testing.T) {
	diff := &queryDiffItem{
		Properties: map[string]interface{}{
			"local:commits": map[string]interface{}{
				"ABCD": map[string]interface{}{
					"time": "012345",
				},
				"EFGHI": map[string]interface{}{
					"time": "456789",
				},
			},
		},
	}
	lastCommit := diff.findLastCommit()
	if lastCommit != "EFGHI" {
		t.Errorf("Wrong result returned from findLastCommit: %v, %s", diff, lastCommit)
	}

	verifyMalformedDiff(t, &queryDiffItem{
		Properties: map[string]interface{}{
			"local:commits": map[string]interface{}{
				"ABCD": map[string]interface{}{
					"time": 12345,
				},
			},
		},
	})
	verifyMalformedDiff(t, &queryDiffItem{
		Properties: map[string]interface{}{
			"local:commits": map[string]interface{}{
				"ABCD": "ABCD",
			},
		},
	})
	verifyMalformedDiff(t, &queryDiffItem{
		Properties: map[string]interface{}{
			"local:commits": "A bunch of stuff",
		},
	})
	verifyMalformedDiff(t, &queryDiffItem{
		Properties: "props",
	})
}
