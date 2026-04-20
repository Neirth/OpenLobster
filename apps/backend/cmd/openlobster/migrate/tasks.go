// # License
// See LICENSE in the root of the repository.
package migrate

import (
	"fmt"
	"os"
)

// migrateTasks reads cron job definitions from the OpenClaw config and creates
// the equivalent tasks in OpenLobster via addTask mutations.
func migrateTasks(cfg viperReader, c *gqlClient) error {
	jobs := readCronJobs(cfg)
	if len(jobs) == 0 {
		fmt.Println("tasks: no cron jobs found — skipping")
		return nil
	}

	fmt.Printf("tasks: %d job(s) to migrate\n", len(jobs))

	const mutation = `mutation AddTask($prompt: String!, $schedule: String) {
		addTask(prompt: $prompt, schedule: $schedule) { id prompt schedule }
	}`

	migrated := 0
	for _, job := range jobs {
		if job.Prompt == "" {
			continue
		}
		label := job.Name
		if label == "" {
			label = job.ID
		}
		fmt.Printf("  %-30s schedule=%q\n", label, job.Schedule)

		if c.dryRun {
			migrated++
			continue
		}

		vars := map[string]any{"prompt": job.Prompt}
		if job.Schedule != "" {
			vars["schedule"] = job.Schedule
		}

		var result struct {
			AddTask struct {
				ID string `json:"id"`
			} `json:"addTask"`
		}
		if err := c.do(mutation, vars, &result); err != nil {
			fmt.Fprintf(os.Stderr, "  task %q: %v\n", label, err)
			continue
		}
		migrated++
	}

	fmt.Printf("  migrated %d/%d task(s)\n", migrated, len(jobs))
	return nil
}
