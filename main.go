package main

import (
	"context"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/google/go-github/v58/github"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

type Commit struct {
	SHA    string
	Title  string
	Commit *github.HeadCommit
	runs   []*github.WorkflowRun
}

type RepoView struct {
	Owner           string
	Repo            string
	Commits         []Commit
	CommitToRuns    map[string][]*github.WorkflowRun
	FailedRunToJobs map[int64][]*github.WorkflowJob
}

func collectCommits(owner, repo string) (*RepoView, error) {
	token := os.Getenv("GITHUB_TOKEN")

	ctx := context.Background()
	client := github.NewClient(nil).WithAuthToken(token)

	r, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	branch := *r.DefaultBranch

	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(
		ctx,
		owner, repo,
		&github.ListWorkflowRunsOptions{
			Branch:      branch,
			ListOptions: github.ListOptions{PerPage: 20},
		})
	if err != nil {
		return nil, err
	}

	var commits []Commit
	commitToRuns := make(map[string][]*github.WorkflowRun)
	failedRunToJobs := make(map[int64][]*github.WorkflowJob)
	for _, run := range runs.WorkflowRuns {
		headSHA := *run.HeadSHA

		if run.GetConclusion() == "failure" {
			runID := run.GetID()
			jobs, _, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, &github.ListWorkflowJobsOptions{})
			if err != nil {
				return nil, err
			}
			failedRunToJobs[runID] = append(failedRunToJobs[runID], jobs.Jobs...)
		}

		if commitToRuns[headSHA] == nil {
			lines := strings.Split(run.GetHeadCommit().GetMessage(), "\n")
			commits = append(commits, Commit{
				SHA:    headSHA,
				Title:  lines[0],
				Commit: run.HeadCommit,
			})
		}
		commitToRuns[headSHA] = append(commitToRuns[headSHA], run)
	}

	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Commit.Timestamp.After(commits[j].Commit.Timestamp.Time)
	})

	return &RepoView{
		Owner:           owner,
		Repo:            repo,
		Commits:         commits,
		CommitToRuns:    commitToRuns,
		FailedRunToJobs: failedRunToJobs,
	}, nil
}

func main() {
	ec := echo.New()

	t := &Template{
		templates: template.Must(template.ParseGlob("html/*.html")),
	}
	ec.Renderer = t

	ec.Use(middleware.Logger())
	ec.Use(middleware.Recover())
	ec.GET("/:owner/:repo", func(c echo.Context) error {
		owner := c.Param("owner")
		repo := c.Param("repo")
		view, err := collectCommits(owner, repo)
		if err != nil {
			return err
		}
		return c.Render(http.StatusOK, "repo.html", view)
	})
	ec.Start(":8080")
}
