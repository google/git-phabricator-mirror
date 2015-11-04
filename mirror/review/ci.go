package review

import (
	"github.com/google/git-appraise/repository"
	"github.com/google/git-appraise/review/ci"
	"log"
	"sort"
	"strconv"
)

func GetLatestCIReport(notes []repository.Note) ci.Report {
	timestampReportMap := make(map[int]ci.Report)
	var timestamps []int

	validCIReports := ci.ParseAllValid(notes)
	for _, report := range validCIReports {
		timestamp, err := strconv.Atoi(report.Timestamp)
		if err != nil {
			log.Fatal(err)
		}
		timestamps = append(timestamps, timestamp)
		timestampReportMap[timestamp] = report
	}
	if len(timestamps) == 0 {
		return ci.Report{}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(timestamps)))
	return timestampReportMap[timestamps[0]]
}
