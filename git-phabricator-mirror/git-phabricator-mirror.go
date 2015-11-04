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

package main

import (
	"flag"
	"github.com/google/git-appraise/repository"
	"github.com/google/git-phabricator-mirror/mirror"
	"log"
	"os"
	"path/filepath"
	"time"
)

var searchDir = flag.String("search_dir", "/var/repo", "Directory under which to search for git repos")
var syncToRemote = flag.Bool("sync_to_remote", false, "Sync the local repos (including git notes) to their remotes")
var syncPeriod = flag.Int("sync_period", 30, "Expected number of seconds between subsequent syncs of a repo.")

func findRepos(searchDir string) ([]repository.Repo, error) {
	// This method finds repos by recursively traversing the given directory,
	// and looking for any git repos.
	var repos []repository.Repo
	filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			gitRepo, err := repository.NewGitRepo(path)
			if err == nil {
				repos = append(repos, gitRepo)
				// Since we have found a git repo, we don't need to
				// traverse any of its child directories.
				return filepath.SkipDir
			}
		}
		return nil
	})
	return repos, nil
}

func main() {
	flag.Parse()
	// We want to always start processing new repos that are added after the binary has started,
	// so we need to run the findRepos method in an infinite loop.

	ticker := time.Tick(time.Duration(*syncPeriod) * time.Second)
	for {
		repos, err := findRepos(*searchDir)
		if err != nil {
			log.Fatal(err.Error())
		}
		for _, repo := range repos {
			mirror.Repo(repo, *syncToRemote)
		}
		if *syncToRemote {
			<-ticker
		}
	}
}
