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
	"sort"
	"strings"
)

var fileStatFilter = func(fs object.FileStat) bool {
	return true
}

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

var url = flag.String("url", "", "example: https://gitlab.int.tsum.com/preowned/simona/delta/customer-service.git")

func main() {
	flag.Usage = func() {
		fmt.Println("Use GITLAB_USER and GITLAB_PASSWORD for auth")
		flag.PrintDefaults()
	}
	flag.Parse()
	if strings.TrimSpace(lo.FromPtr(url)) == "" {
		flag.Usage()
		os.Exit(1)
	}

	repository := lo.Must(git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:  *url,
		Auth: &http.BasicAuth{Username: os.Getenv("GITLAB_USER"), Password: os.Getenv("GITLAB_PASSWORD")},
	}))
	statByAuthor := analyzeRepo(repository)

	tabl := table.New("author", "commits", "total", "additions", "deletions")
	tabl.WithHeaderFormatter(color.New(color.FgGreen, color.Underline).SprintfFunc()).
		WithFirstColumnFormatter(color.New(color.FgYellow).SprintfFunc())

	authors := lo.Keys(statByAuthor)
	sort.Slice(authors, func(i, j int) bool {
		return statByAuthor[authors[i]].total() > statByAuthor[authors[j]].total()
	})

	for _, author := range authors {
		st := statByAuthor[author]
		tabl.AddRow(author, len(st.commits), st.total(), st.additions, st.deletions)
	}
	tabl.Print()
}

type stat struct {
	additions int
	deletions int
	commits   []*object.Commit
	//TODO time period
}

func (s stat) total() int {
	return s.additions + s.deletions
}

/*
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
	    URL: "https://github.com/go-git/go-billy",
	})
*/
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
		a, d := statsSum(commits)
		return stat{additions: a, deletions: d, commits: commits}
	})
}

func statsSum(commits []*object.Commit) (int, int) {
	var additions, deletions int
	for _, c := range commits {
		a, d := statSum(c)
		additions += a
		deletions += d
	}
	return additions, deletions
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
