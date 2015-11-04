package ci

import (
	"github.com/google/git-appraise/repository"
	gaCi "github.com/google/git-appraise/review/ci"
	"log"
	"sort"
	"strconv"
)

func GetLatestCIReport(notes []repository.Note) gaCi.Report {
	timestampReportMap := make(map[int]gaCi.Report)
	var timestamps []int

	validCIReports := gaCi.ParseAllValid(notes)
	for _, report := range validCIReports {
		timestamp, err := strconv.Atoi(report.Timestamp)
		if err != nil {
			log.Fatal(err)
		}
		timestamps = append(timestamps, timestamp)
		timestampReportMap[timestamp] = report
	}
	if len(timestamps) == 0 {
		return gaCi.Report{}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(timestamps)))
	return timestampReportMap[timestamps[0]]
}
