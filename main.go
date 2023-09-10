package main

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/rodaine/table"
	"github.com/samber/lo"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"time"
	"unicode/utf8"
)

var fileStatRe *regexp.Regexp
var fileStatFilter = func(fs object.FileStat) bool {
	return true
}

var authorFunc = func(fs object.Signature) string {
	return fs.Name
}

func main() {
	conf, err := parseConfig()
	if err != nil {
		log.Fatal("parseConfig:", err)
	}
	if conf.Files.IncludeRe != "" {
		fileStatRe, err = regexp.Compile(conf.Files.IncludeRe)
		if err != nil {
			log.Fatalf("can't parse Files.IncludeRe %q: %s", conf.Files.IncludeRe, err)
		}
	}

	fileStatFilter = func(fs object.FileStat) bool {
		if fileStatRe == nil {
			return true
		}
		return fileStatRe.MatchString(fs.Name)
	}
	authorFunc = func(fs object.Signature) string {
		key := fs.Name
		if conf.Author.Key == "mail" {
			key = fs.Email
		}

		for mainName, duplicates := range conf.Author.Duplicates {
			if key == mainName || lo.Contains(duplicates, key) {
				return mainName
			}
		}
		return key
	}

	maxPathLen := lo.Max(lo.Map(conf.Paths, func(item string, _ int) int {
		return utf8.RuneCountInString(path.Base(item))
	}))
	totalStatsByAuthor := map[string]stat{}
	for _, url := range conf.Paths {
		repository := lo.Must(git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			RecurseSubmodules: git.NoRecurseSubmodules,
			URL:               url,
			Auth:              &http.BasicAuth{Username: os.Getenv("GITLAB_USER"), Password: os.Getenv("GITLAB_PASSWORD")},
		}))
		statByAuthor := analyzeRepoByAuthor(repository)

		tabl := table.New(fmt.Sprintf("%*s:", maxPathLen, path.Base(url)), " author", "commits", "total", "additions", "deletions", "days", "additions/day")
		tabl.WithHeaderFormatter(color.New(color.FgGreen, color.Underline).SprintfFunc()).
			WithFirstColumnFormatter(color.New(color.FgYellow).SprintfFunc())

		authors := lo.Keys(statByAuthor)
		sort.Slice(authors, func(i, j int) bool {
			return statByAuthor[authors[i]].additionsPerDay() > statByAuthor[authors[j]].additionsPerDay()
		})

		for _, author := range authors {
			st := statByAuthor[author]
			totalStatsByAuthor[author] = st.aggregate(totalStatsByAuthor[author])
			tabl.AddRow("", author, len(st.commits), st.total(), st.additions, st.deletions, len(st.days), st.additionsPerDay())
		}
		tabl.Print()
	}

	//totals results
	if len(totalStatsByAuthor) != 0 {
		tabl := table.New(fmt.Sprintf("%*s:", maxPathLen, "TOTAL"), " author", "commits", "total", "additions", "deletions", "days", "additions/day")
		tabl.WithHeaderFormatter(color.New(color.FgGreen, color.Underline).SprintfFunc()).
			WithFirstColumnFormatter(color.New(color.FgYellow).SprintfFunc())
		authors := lo.Keys(totalStatsByAuthor)
		sort.Slice(authors, func(i, j int) bool {
			return totalStatsByAuthor[authors[i]].additionsPerDay() > totalStatsByAuthor[authors[j]].additionsPerDay()
		})
		for _, author := range authors {
			st := totalStatsByAuthor[author]
			tabl.AddRow("", st.author, len(st.commits), st.total(), st.additions, st.deletions, len(st.days), st.additionsPerDay())
		}
		tabl.Print()
	}
	//largest commits

}

type stat struct {
	additions int
	deletions int
	commits   []*object.Commit
	days      map[time.Time]bool
	author    string
}

func newStat(commits []*object.Commit, author string) stat {
	var additions, deletions int
	days := map[time.Time]bool{}
	for _, c := range commits {
		a, d := statSum(c)
		additions += a
		deletions += d
		days[c.Author.When.Truncate(24*time.Hour)] = true
	}
	return stat{additions: additions, deletions: deletions, commits: commits, days: days, author: author}
}

func (s stat) additionsPerDay() int {
	return s.additions / len(s.days)
}

func (s stat) total() int {
	return s.additions + s.deletions
}

func (s stat) aggregate(other stat) stat {
	return stat{
		additions: s.additions + other.additions,
		deletions: s.deletions + other.deletions,
		commits:   append(s.commits, other.commits...),
		days: lo.SliceToMap(append(lo.Keys(s.days), lo.Keys(other.days)...), func(day time.Time) (time.Time, bool) {
			return day, true
		}),
		author: s.author,
	}
}

func analyzeRepoByAuthor(repository *git.Repository) map[string]stat {
	commitIter := lo.Must(repository.CommitObjects())
	defer commitIter.Close()

	commitsByAuthor := map[string][]*object.Commit{}
	lo.Must0(commitIter.ForEach(func(commit *object.Commit) error {
		key := authorFunc(commit.Author)
		commitsByAuthor[key] = append(commitsByAuthor[key], commit)
		return nil
	}))

	return lo.MapValues(commitsByAuthor, func(commits []*object.Commit, author string) stat {
		return newStat(commits, author)
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
