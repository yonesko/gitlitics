package main

import (
	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/rodaine/table"
	"github.com/samber/lo"
	"sort"
)

var fileStatFilter = func(fs object.FileStat) bool {
	return true
}

var authorFunc = func(fs object.Signature) string {
	return fs.Email
}

func main() {
	directory := "/Users/gdanichev/GolandProjects/simona/delta/price-reconciliation-service.git"
	//directory := os.Args[1]

	statByAuthor := analyzeRepo(directory)

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
}

func (s stat) total() int {
	return s.additions + s.deletions
}

func analyzeRepo(directory string) map[string]stat {
	repository := lo.Must(git.PlainOpen(directory))
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
