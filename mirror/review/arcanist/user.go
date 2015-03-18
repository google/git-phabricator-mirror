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
	"fmt"
	"time"
)

type userQueryRequest struct {
	Emails    []string `json:"emails,omitempty"`
	UserNames []string `json:"usernames,omitempty"`
	IDs       []string `json:"phids,omitempty"`
}

type user struct {
	PHID     string `json:"phid"`
	UserName string `json:"userName,omitempty"`
	RealName string `json:"realName,omitempty"`
	Email    string `json:"primaryEmail,omitempty"`
}

type cachedUser struct {
	User *user
	Time time.Time
}

type userQueryResponse struct {
	Error        string `json:"error,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	Response     []user `json:"response,omitempty"`
}

type whoAmIResponse struct {
	Error        string `json:"error,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	Response     user   `json:"response,omitempty"`
}

var userQueryCache = make(map[string]cachedUser)
var userLookupCache = make(map[string]cachedUser)

// We should have *some* time limit for cache values, as the user might change their
// email address in Phabricator, but we don't have any data to decide what is a
// reasonable limit, so we are just starting with 5 minutes as an initial value.
var userCacheDuration = -(time.Minute * 5)

func userCacheLookup(key string, cache map[string]cachedUser, f func() (*user, error)) (*user, error) {
	if cachedValue, ok := cache[key]; ok {
		if cachedValue.Time.After(time.Now().Add(userCacheDuration)) {
			return cachedValue.User, nil
		}
	}
	result, err := f()
	if err != nil {
		return result, err
	}
	cache[key] = cachedUser{
		User: result,
		Time: time.Now(),
	}
	return result, nil
}

// queryUser returns the Phabricator user with the given name, or nil if there is none.
//
// Since we do not know if the name is an email address or a username, we first try
// to find a user whose email matches the name, and then fall back to a username
// search if that fails.
func queryUser(name string) (*user, error) {
	return userCacheLookup(name, userQueryCache, func() (*user, error) {
		emailQueryRequest := userQueryRequest{Emails: []string{name}}
		var queryResponse userQueryResponse
		runArcCommandOrDie("user.query", emailQueryRequest, &queryResponse)
		if queryResponse.Error != "" {
			return nil, fmt.Errorf("Failed to query the Phabricator users: %s", queryResponse.ErrorMessage)
		}
		if len(queryResponse.Response) == 0 {
			usernameQueryRequest := userQueryRequest{UserNames: []string{name}}
			runArcCommandOrDie("user.query", usernameQueryRequest, &queryResponse)
			if queryResponse.Error != "" {
				return nil, fmt.Errorf("Failed to query the Phabricator users: %s", queryResponse.ErrorMessage)
			}
		}
		if len(queryResponse.Response) != 1 {
			return nil, nil
		}
		return &queryResponse.Response[0], nil
	})
}

// lookupUser reads the Phabricator user given the corresponding unique ID.
func lookupUser(userPHID string) (*user, error) {
	return userCacheLookup(userPHID, userLookupCache, func() (*user, error) {
		queryRequest := userQueryRequest{IDs: []string{userPHID}}
		var queryResponse userQueryResponse
		runArcCommandOrDie("user.query", queryRequest, &queryResponse)
		if queryResponse.Error != "" {
			return nil, fmt.Errorf("Failed to query the Phabricator users: %s", queryResponse.ErrorMessage)
		}
		if len(queryResponse.Response) != 1 {
			return nil, nil
		}
		return &queryResponse.Response[0], nil
	})
}

var mirrorUser *user = nil

// whoAmI returns the Phabricator user for the mirroring tool.
func whoAmI() (user, error) {
	if mirrorUser != nil {
		return *mirrorUser, nil
	}
	var response whoAmIResponse
	runArcCommandOrDie("user.whoami", struct{}{}, &response)
	if response.Error != "" {
		return user{}, fmt.Errorf("Failed to lookup the current user: %s", response.ErrorMessage)
	}
	mirrorUser = &response.Response
	return *mirrorUser, nil
}
