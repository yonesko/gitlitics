package main

import (
	"flag"
	"fmt"
	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/rodaine/table"
	"github.com/samber/lo"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

var fileStatFilter = func(fs object.FileStat) bool {
	return strings.HasSuffix(fs.Name, ".go")
}

// TODO to config file
var authorFunc = func(fs object.Signature) string {
	switch fs.Name {
	case "gdanichev", "Глеб", "Глеб Данчев":
		return "Глеб Данчев"
	case "Evgeny Nazarov", "Евгений Назаров":
		return "Евгений Назаров"
	case "sagadzhanov", "Сергей Агаджанов":
		return "Сергей Агаджанов"
	case "Edgar Sipki", "Эдгар Сипки":
		return "Эдгар Сипки"
	case "sbaronenkov", "Сергей Бароненков":
		return "Сергей Бароненков"
	case "Daniil Guzanov", "Даниил Гузанов":
		return "Даниил Гузанов"
	}
	return fs.Name
}

var urls = flag.String("urls", "", "example: https://gitlab.int.tsum.com/preowned/simona/delta/customer-service.git, comma-separated")

func main() {
	flag.Usage = func() {
		fmt.Println("Use GITLAB_USER and GITLAB_PASSWORD for auth")
		flag.PrintDefaults()
	}
	flag.Parse()
	if strings.TrimSpace(lo.FromPtr(urls)) == "" {
		flag.Usage()
		os.Exit(1)
	}

	for _, url := range strings.Split(*urls, ",") {
		repository := lo.Must(git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL:  url,
			Auth: &http.BasicAuth{Username: os.Getenv("GITLAB_USER"), Password: os.Getenv("GITLAB_PASSWORD")},
		}))
		statByAuthor := analyzeRepo(repository)

		tabl := table.New(path.Base(url)+":", " author", "commits", "total", "additions", "deletions", "days", "additions/day")
		tabl.WithHeaderFormatter(color.New(color.FgGreen, color.Underline).SprintfFunc()).
			WithFirstColumnFormatter(color.New(color.FgYellow).SprintfFunc())

		authors := lo.Keys(statByAuthor)
		sort.Slice(authors, func(i, j int) bool {
			return statByAuthor[authors[i]].total() > statByAuthor[authors[j]].total()
		})

		for _, author := range authors {
			st := statByAuthor[author]
			tabl.AddRow("", author, len(st.commits), st.total(), st.additions, st.deletions, len(st.days), st.additions/len(st.days))
		}
		tabl.Print()
	}
}

type stat struct {
	additions int
	deletions int
	commits   []*object.Commit
	days      map[time.Time]bool
}

func newStat(commits []*object.Commit) stat {
	var additions, deletions int
	days := map[time.Time]bool{}
	for _, c := range commits {
		a, d := statSum(c)
		additions += a
		deletions += d
		days[c.Author.When.Truncate(24*time.Hour)] = true
	}
	return stat{additions: additions, deletions: deletions, commits: commits, days: days}
}

func (s stat) total() int {
	return s.additions + s.deletions
}

func analyzeRepo(repository *git.Repository) map[string]stat {
	commitIter := lo.Must(repository.CommitObjects())
	defer commitIter.Close()

	commitsByAuthor := map[string][]*object.Commit{}
	lo.Must0(commitIter.ForEach(func(commit *object.Commit) error {
		key := authorFunc(commit.Author)
		commitsByAuthor[key] = append(commitsByAuthor[key], commit)
		return nil
	}))

	return lo.MapValues(commitsByAuthor, func(commits []*object.Commit, _ string) stat {
		return newStat(commits)
	})
}

func statSum(commit *object.Commit) (int, int) {
	fileStats := lo.Must(commit.Stats())

	var additions, deletions int
	for _, fs := range fileStats {
		if !fileStatFilter(fs) {
			continue
		}
		additions += fs.Addition
		deletions += fs.Deletion
	}
	return additions, deletions
}
