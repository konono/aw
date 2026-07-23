package reaper

import (
	"encoding/json"
	"os"
)

const (
	DefaultReaperTimeout     = 60
	DefaultReportRetention   = 10
	DoctorReportLookback     = 10
	DefaultCollectLogs       = "on_failure"
)

func effectiveReportRetention(n int) int {
	if n > 0 {
		return n
	}
	return DefaultReportRetention
}

func taskKey(task ReaperTask) string {
	return task.Label + "|" + task.Type
}

// succeededTaskKeys returns labels of tasks that succeeded in the latest report
// for the given container, keyed by "label|type".
func succeededTaskKeys(containerName string) map[string]bool {
	reports := ListReports()
	for i := len(reports) - 1; i >= 0; i-- {
		report, err := ReadReport(reports[i])
		if err != nil || report.ContainerName != containerName {
			continue
		}
		out := make(map[string]bool)
		for _, t := range report.Tasks {
			if t.Status == "ok" {
				out[t.Label+"|"+t.Type] = true
			}
		}
		return out
	}
	return nil
}

func ReadReport(path string) (*RunReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}
